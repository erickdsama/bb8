// Package tools is the one tool registry shared by the MCP server and the agent.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
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
	Handler     Handler
}

// CallOpts carries who is asking.
type CallOpts struct {
	SpeakerKnown bool // false for an unrecognized voice
}

var ErrUnknownVoice = errors.New("unknown voice: movement tools are only for known voices")

type Registry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	owners   map[string]string // tool → owner (connector id or "robot")
	onChange []func()
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}, owners: map[string]string{}}
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
		r.tools[t.Name] = t
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
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	return t.Handler(ctx, args)
}

// Object builds a JSON schema object. props: name → schema.
func Object(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func Num(desc string) map[string]any  { return map[string]any{"type": "number", "description": desc} }
func Str(desc string) map[string]any  { return map[string]any{"type": "string", "description": desc} }
func Bool(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
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
