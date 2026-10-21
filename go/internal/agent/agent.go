// Package agent is BB-8's own brain: an OpenRouter chat loop (OpenAI-compatible
// /chat/completions with tools) over the shared tool registry.
package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/people"
	"github.com/erickdsama/bb8/go/internal/tools"
)

func Fields() []config.Field {
	return []config.Field{
		{Key: "agent.model", Label: "Model (OpenRouter id)", Type: config.String, Default: "anthropic/claude-sonnet-5.5",
			Help: "Any OpenRouter model with tool calling, e.g. anthropic/…, openai/…, google/…"},
		{Key: "agent.api_key", Label: "OpenRouter API key", Type: config.Secret},
		{Key: "agent.base_url", Label: "API base URL", Type: config.String, Default: "https://openrouter.ai/api/v1"},
		{Key: "agent.vision", Label: "Send photos to the model", Type: config.Bool, Default: true,
			Help: "Turn off for text-only models."},
		{Key: "agent.max_steps", Label: "Max tool steps per request", Type: config.Number, Default: 8.0, Min: config.F(1), Max: config.F(20)},
		{Key: "agent.max_tokens", Label: "Max tokens per reply", Type: config.Number, Default: 1024.0,
			Min: config.F(64), Max: config.F(8192),
			Help: "Spoken answers are short. Without a cap OpenRouter reserves the model's maximum and a small credit limit rejects the request."},
		{Key: "agent.history_messages", Label: "Messages kept in the conversation", Type: config.Number, Default: 12.0,
			Min: config.F(2), Max: config.F(60), Help: "Older ones are dropped. Fewer = cheaper and less confusion from old turns."},
		{Key: "agent.reset_after_s", Label: "Forget the conversation after (s)", Type: config.Number, Default: 120.0,
			Min: config.F(30), Max: config.F(3600)},
	}
}

type Agent struct {
	Cfg          *config.Store
	Reg          *tools.Registry
	Instructions func() string
	HTTP         *http.Client
	People       *people.Store              // optional: speaker profiles and the "people you know" line
	Publish      func(typ string, data any) // optional: heard, reply and tool events for the pages

	mu      sync.Mutex
	history []msg
	last    time.Time
}

func New(cfg *config.Store, reg *tools.Registry, instructions func() string) *Agent {
	return &Agent{Cfg: cfg, Reg: reg, Instructions: instructions, HTTP: &http.Client{Timeout: 90 * time.Second}}
}

// --- OpenAI-compatible wire types ------------------------------------------

type msg struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"` // string or []part
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type part struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	ImageURL     *imageURL     `json:"image_url,omitempty"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

// cacheControl marks the end of a prompt prefix the provider may cache
// (Anthropic through OpenRouter; others ignore it).
type cacheControl struct {
	Type string `json:"type"`
}

type imageURL struct {
	URL string `json:"url"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type fnSpec struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type chatReq struct {
	Model     string   `json:"model"`
	Messages  []msg    `json:"messages"`
	Tools     []fnSpec `json:"tools,omitempty"`
	MaxTokens int      `json:"max_tokens,omitempty"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Reply is what Ask returns.
type Reply struct {
	Text      string   `json:"reply"`
	ToolsUsed []string `json:"tools_used"`
	Steps     int      `json:"steps"`
}

func (a *Agent) Ready() error {
	if a.Cfg.Secret("agent.api_key") == "" {
		return errors.New("set agent.api_key (OpenRouter) in the config")
	}
	return nil
}

func (a *Agent) specs() []fnSpec {
	var out []fnSpec
	for _, t := range a.Reg.List() {
		var s fnSpec
		s.Type = "function"
		s.Function.Name, s.Function.Description, s.Function.Parameters = t.Name, t.Description, t.Schema
		out = append(out, s)
	}
	return out
}

func (a *Agent) complete(ctx context.Context, msgs []msg) (*chatResp, error) {
	body, _ := json.Marshal(chatReq{Model: a.Cfg.String("agent.model"), Messages: msgs, Tools: a.specs(),
		MaxTokens: int(a.Cfg.Float("agent.max_tokens"))})
	url := strings.TrimRight(a.Cfg.String("agent.base_url"), "/") + "/chat/completions"
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+a.Cfg.Secret("agent.api_key"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "BB-8")
	r, err := a.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	var out chatResp
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("model reply HTTP %d: %.200s", r.StatusCode, b)
	}
	if out.Error != nil {
		return nil, errors.New("model: " + out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("model returned no choices (HTTP %d)", r.StatusCode)
	}
	return &out, nil
}

// Turn is one request: the text, the registered voice that said it (empty for
// the web page, an MCP client or no voice ID) and whether that voice is known.
type Turn struct {
	Text    string
	Speaker string
	Known   bool
}

// Ask runs one user turn: model → tools → model … until it answers in text.
func (a *Agent) Ask(ctx context.Context, text string, speakerKnown bool) (Reply, error) {
	return a.AskAs(ctx, Turn{Text: text, Known: speakerKnown})
}

func (a *Agent) publish(typ string, data any) {
	if a.Publish != nil {
		a.Publish(typ, data)
	}
}

// profile looks the speaker up (adding a new registered voice as family) and returns
// the note for the model and whether this person may act. A guest is treated like
// an unknown voice for Moves and KnownOnly tools.
func (a *Agent) profile(t Turn) (string, bool) {
	if a.People == nil || t.Speaker == "" || !t.Known {
		return "", t.Known
	}
	p, err := a.People.Seen(t.Speaker)
	if err != nil {
		slog.Warn("people", "err", err)
		return "", t.Known
	}
	note := " [Profile: " + p.Name + " is " + string(p.Role)
	if !p.CanAct() {
		note += ", may chat but not move you or use external tools"
	}
	if len(p.Notes) > 0 {
		ns := make([]string, len(p.Notes))
		for i, n := range p.Notes {
			ns[i] = n.Text
		}
		note += ". Notes: " + strings.Join(ns, "; ")
	}
	return note + "]", p.CanAct()
}

// AskAs is Ask with the speaker's identity.
func (a *Agent) AskAs(ctx context.Context, t Turn) (rep Reply, err error) {
	if err := a.Ready(); err != nil {
		return Reply{}, err
	}
	a.publish("heard", map[string]any{"text": t.Text, "speaker": t.Speaker})
	defer func() {
		if err != nil {
			a.publish("reply", map[string]any{"error": err.Error(), "tools": rep.ToolsUsed})
		} else {
			a.publish("reply", map[string]any{"text": rep.Text, "tools": rep.ToolsUsed})
		}
	}()
	text := t.Text
	prof, speakerKnown := a.profile(t)
	a.mu.Lock()
	defer a.mu.Unlock()
	if time.Since(a.last) > time.Duration(a.Cfg.Float("agent.reset_after_s"))*time.Second {
		a.history = nil
	}
	a.last = time.Now()
	who := ""
	if !speakerKnown {
		who = " [unknown voice: you cannot move]"
	}
	a.trim()
	turnStart := len(a.history)
	a.history = append(a.history, msg{Role: "user", Content: text + prof + who})

	maxSteps := int(a.Cfg.Float("agent.max_steps"))
	for rep.Steps = 0; rep.Steps < maxSteps; rep.Steps++ {
		// Tools + instructions are ~4.5k tokens and identical every turn: cache them.
		// The clock goes after the cache mark, or it would change the prefix every minute.
		now := "Now: " + time.Now().Format("Mon 2 Jan 2006 15:04")
		if city := a.Cfg.String("home.city"); city != "" {
			now += ". Home: " + city
		}
		if a.People != nil {
			if s := a.People.Summary(); s != "" {
				now += ". People you know: " + s
			}
		}
		sys := []part{
			{Type: "text", Text: a.Instructions(), CacheControl: &cacheControl{Type: "ephemeral"}},
			{Type: "text", Text: now},
		}
		msgs := append([]msg{{Role: "system", Content: sys}}, a.history...)
		res, err := a.complete(ctx, msgs)
		if err != nil {
			// Drop this turn: a question left without an answer made the next
			// one read two questions in a row.
			a.history = a.history[:turnStart]
			return rep, err
		}
		m := res.Choices[0].Message
		a.history = append(a.history, msg{Role: "assistant", Content: m.Content, ToolCalls: m.ToolCalls})
		if len(m.ToolCalls) == 0 {
			rep.Text = strings.TrimSpace(m.Content)
			return rep, nil
		}
		var images [][]byte
		for _, tc := range m.ToolCalls {
			rep.ToolsUsed = append(rep.ToolsUsed, tc.Function.Name)
			out, err := a.Reg.Call(ctx, tc.Function.Name, json.RawMessage(orEmpty(tc.Function.Arguments)), tools.CallOpts{SpeakerKnown: speakerKnown})
			content := ""
			switch {
			case err != nil:
				content = `{"result":"error","reason":` + quote(err.Error()) + `}`
			case out.Image != nil:
				content = `{"result":"done","photo":"attached in the next message"}`
				images = append(images, out.Image)
			default:
				b, _ := json.Marshal(out.Data)
				content = string(b)
			}
			slog.Info("agent tool", "name", tc.Function.Name, "args", tc.Function.Arguments, "err", err)
			ev := map[string]any{"name": tc.Function.Name}
			if err != nil {
				ev["error"] = err.Error()
			} else if m, ok := out.Data.(map[string]any); ok && m["result"] != nil {
				ev["result"] = m["result"]
			}
			a.publish("tool", ev)
			a.history = append(a.history, msg{Role: "tool", ToolCallID: tc.ID, Content: content})
		}
		if len(images) > 0 && a.Cfg.Bool("agent.vision") {
			parts := []part{{Type: "text", Text: "Photo from your head camera:"}}
			for _, img := range images {
				parts = append(parts, part{Type: "image_url", ImageURL: &imageURL{URL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(img)}})
			}
			a.history = append(a.history, msg{Role: "user", Content: parts})
		}
	}
	rep.Text = "Bip. Me enredé; dime otra vez."
	return rep, nil
}

func orEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}

func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

// trim keeps the history short and drops old photos (they cost many tokens).
func (a *Agent) trim() {
	keep := int(a.Cfg.Float("agent.history_messages"))
	if len(a.history) > keep {
		cut := len(a.history) - keep
		// never start on a tool reply without its assistant call
		for cut < len(a.history) && a.history[cut].Role != "user" {
			cut++
		}
		a.history = a.history[cut:]
	}
	for i := range a.history[:max(0, len(a.history)-1)] {
		if parts, ok := a.history[i].Content.([]part); ok {
			a.history[i].Content = parts[0].Text + " (old photo removed)"
		}
	}
}

// Routes: GET /api/agent (status), POST /api/agent/ask {text, speaker_known}.
func (a *Agent) Routes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix, func(w http.ResponseWriter, r *http.Request) {
		st := map[string]any{"model": a.Cfg.String("agent.model"), "ready": a.Ready() == nil}
		if err := a.Ready(); err != nil {
			st["detail"] = err.Error()
		}
		writeJSON(w, 200, st)
	})
	mux.HandleFunc("POST "+prefix+"/ask", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Text         string `json:"text"`
			Speaker      string `json:"speaker"` // registered voice name, if any
			SpeakerKnown *bool  `json:"speaker_known"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil || strings.TrimSpace(in.Text) == "" {
			writeJSON(w, 400, map[string]any{"error": "send {\"text\": …}"})
			return
		}
		known := in.SpeakerKnown == nil || *in.SpeakerKnown
		rep, err := a.AskAs(r.Context(), Turn{Text: in.Text, Speaker: strings.TrimSpace(in.Speaker), Known: known})
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": err.Error(), "tools_used": rep.ToolsUsed})
			return
		}
		writeJSON(w, 200, rep)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
