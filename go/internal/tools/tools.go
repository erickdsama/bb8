// Package tools is the one tool registry shared by the MCP server and the agent.
package tools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"
)

// Output of a tool: JSON-able data and/or a JPEG image.
type Output struct {
	Data  any
	Image []byte
}

type Handler func(ctx context.Context, args json.RawMessage) (Output, error)

type Tool struct {
	Name        string
	Description string
	Schema      map[string]any // JSON schema, type object
	Moves       bool           // moves the robot: refused for unknown voices
	KnownOnly   bool           // acts outside the robot (a remote MCP tool): also refused for unknown voices
	// Confirm: it sends, creates or deletes something outside (an email, an
	// event). A voice must say yes in a later turn; see Registry.Call.
	Confirm bool
	// For says whether this caller may see and use the tool (an account tool
	// is only for its owner). nil: everyone, subject to the rules above.
	For     func(CallOpts) bool
	Handler Handler
}

// CallOpts carries who is asking.
type CallOpts struct {
	SpeakerKnown bool   // false for an unrecognized voice (or a guest)
	Speaker      string // the person's name; "" for the web page, an MCP client or no voice ID
	Admin        bool   // an MCP client or the local API: trusted like the owner, no confirmations
	Turn         uint64 // the agent's user-turn number; a confirmation must come in a later turn
	Voice        bool   // the turn came from the microphone: the reply is spoken by itself
}

type callerKey struct{}

// CallerFrom returns who called the tool (set by Registry.Call).
func CallerFrom(ctx context.Context) CallOpts {
	o, _ := ctx.Value(callerKey{}).(CallOpts)
	return o
}

// WithCaller sets the caller (tests).
func WithCaller(ctx context.Context, o CallOpts) context.Context {
	return context.WithValue(ctx, callerKey{}, o)
}

var (
	ErrUnknownVoice = errors.New("unknown voice: movement tools are only for known voices")
	ErrKnownOnly    = errors.New("unknown voice: this tool is only for known voices")
	ErrNotYours     = errors.New("not available for this person: they have not connected that service")
	ErrConfirmVoice = errors.New("unknown voice: only known voices can send, create or delete things")
)

// ConfirmTTL is how long a pending confirmation stays valid.
const ConfirmTTL = 5 * time.Minute

type pending struct {
	tool, args, speaker string
	turn                uint64
	at                  time.Time
}

type Registry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	owners   map[string]string // tool → owner (connector id or "robot")
	onChange []func()

	pmu     sync.Mutex
	pending map[string]pending // confirm_id → the call waiting for a yes
	Now     func() time.Time
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}, owners: map[string]string{}, pending: map[string]pending{}, Now: time.Now}
}

// withConfirmArg adds the confirm_id argument to a Confirm tool's schema.
func withConfirmArg(t Tool) Tool {
	if !t.Confirm {
		return t
	}
	s := map[string]any{}
	for k, v := range t.Schema {
		s[k] = v
	}
	props := map[string]any{}
	if p, ok := s["properties"].(map[string]any); ok {
		for k, v := range p {
			props[k] = v
		}
	}
	props["confirm_id"] = Str("Only after the person said yes: the confirm_id from the needs_confirmation answer.")
	s["properties"] = props
	if s["type"] == nil {
		s["type"] = "object"
	}
	t.Schema = s
	return t
}

// Set replaces every tool of an owner (a connector re-registers on config change).
func (r *Registry) Set(owner string, ts []Tool) {
	r.mu.Lock()
	for n, o := range r.owners {
		if o == owner {
			delete(r.tools, n)
			delete(r.owners, n)
		}
	}
	for _, t := range ts {
		r.tools[t.Name] = withConfirmArg(t)
		r.owners[t.Name] = owner
	}
	fns := append([]func(){}, r.onChange...)
	r.mu.Unlock()
	for _, f := range fns {
		f()
	}
}

func (r *Registry) OnChange(f func()) {
	r.mu.Lock()
	r.onChange = append(r.onChange, f)
	r.mu.Unlock()
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ListFor is List without the tools this caller may not use (another person's accounts).
func (r *Registry) ListFor(o CallOpts) []Tool {
	all := r.List()
	out := all[:0]
	for _, t := range all {
		if t.For == nil || t.For(o) {
			out = append(out, t)
		}
	}
	return out
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage, o CallOpts) (Output, error) {
	t, ok := r.Get(name)
	if !ok {
		return Output{}, errors.New("unknown tool " + name)
	}
	if t.Moves && !o.SpeakerKnown {
		return Output{}, ErrUnknownVoice
	}
	if t.KnownOnly && !o.SpeakerKnown {
		return Output{}, ErrKnownOnly
	}
	if t.For != nil && !t.For(o) {
		return Output{}, ErrNotYours
	}
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	if t.Confirm && !o.Admin {
		if !o.SpeakerKnown {
			return Output{}, ErrConfirmVoice
		}
		var ok bool
		var out Output
		if args, out, ok = r.confirm(name, args, o); !ok {
			return out, nil
		}
	}
	return t.Handler(WithCaller(ctx, o), args)
}

// confirm enforces "ask first" in code, not in the prompt: the first call is
// parked and answered with a confirm_id; the same call with that id runs only
// from a later user turn by the same person (so the model cannot say yes for them).
func (r *Registry) confirm(name string, args json.RawMessage, o CallOpts) (json.RawMessage, Output, bool) {
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil || m == nil {
		m = map[string]any{}
	}
	id, _ := m["confirm_id"].(string)
	delete(m, "confirm_id")
	clean, _ := json.Marshal(m) // map keys come out sorted: a stable fingerprint
	now := r.Now()
	r.pmu.Lock()
	defer r.pmu.Unlock()
	for k, p := range r.pending {
		if now.Sub(p.at) > ConfirmTTL {
			delete(r.pending, k)
		}
	}
	if p, ok := r.pending[id]; ok && id != "" {
		if p.tool == name && p.args == string(clean) && p.speaker == o.Speaker && o.Turn > p.turn {
			delete(r.pending, id)
			return clean, Output{}, true
		}
		if o.Turn <= p.turn {
			return nil, Output{Data: map[string]any{"result": "needs_confirmation", "confirm_id": id,
				"say": "Wait for the person's answer: ask them first and call again only after they say yes."}}, false
		}
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	id = "c" + hex.EncodeToString(b)
	r.pending[id] = pending{tool: name, args: string(clean), speaker: o.Speaker, turn: o.Turn, at: now}
	return nil, Output{Data: map[string]any{"result": "needs_confirmation", "confirm_id": id,
		"say": "Tell the person in one short sentence exactly what you are about to do and ask yes or no. " +
			"If they say yes in their next message, call this tool again with the same arguments plus this confirm_id."}}, false
}

// Object builds a JSON schema object. props: name → schema.
func Object(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func Num(desc string) map[string]any { return map[string]any{"type": "number", "description": desc} }
func Str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func Enum(desc string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": vals}
}

// Bind decodes args into a typed struct handler.
func Bind[T any](f func(ctx context.Context, in T) (Output, error)) Handler {
	return func(ctx context.Context, args json.RawMessage) (Output, error) {
		var in T
		if err := json.Unmarshal(args, &in); err != nil {
			return Output{}, errors.New("bad arguments: " + err.Error())
		}
		return f(ctx, in)
	}
}

// Data wraps a plain value.
func Data(v any) (Output, error) { return Output{Data: v}, nil }
