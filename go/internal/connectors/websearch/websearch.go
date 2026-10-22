// Package websearch answers questions about the outside world with OpenRouter's
// web plugin: one sub-request that searches, reads and summarizes with sources.
// It reuses agent.api_key, so it needs no account of its own.
package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/tools"
)

const (
	keyModel   = "connectors.web.model"
	keyResults = "connectors.web.max_results"
)

type Web struct {
	HTTP *http.Client
}

func New() *Web { return &Web{HTTP: &http.Client{Timeout: 45 * time.Second}} }

func (w *Web) ID() string      { return "web" }
func (w *Web) DefaultOn() bool { return true }
func (w *Web) Title() string   { return "Web search" }
func (w *Web) Description() string {
	return "News, prices, schedules and anything current, through OpenRouter's web plugin. Uses the agent's OpenRouter key (~$0.02 per search)."
}

func (w *Web) Fields() []config.Field {
	return []config.Field{
		{Key: keyModel, Label: "Search model (OpenRouter id)", Type: config.String, Default: "",
			Help: "Empty = the agent's model. A cheap fast model is enough: it only reads and summarizes the results.", Advanced: true},
		{Key: keyResults, Label: "Results per search", Type: config.Number, Default: 5.0, Min: config.F(1), Max: config.F(10),
			Help: "Each result costs about $0.004 on OpenRouter.", Advanced: true},
	}
}

func (w *Web) Status(c *config.Store) connectors.Status {
	if c.Secret("agent.api_key") == "" {
		return connectors.Status{ID: w.ID(), State: connectors.NeedsConf, Detail: "Set Agent → OpenRouter API key."}
	}
	return connectors.Status{ID: w.ID(), State: connectors.OK}
}

func (w *Web) Tools(c *config.Store) []tools.Tool {
	return []tools.Tool{{
		Name: "web_search",
		Description: "Search the internet for anything current or outside the house: news, sports, prices, " +
			"opening hours, people, places, facts that may have changed. query: a full question in any language. " +
			"Returns answer (a short summary with dates) and sources (title, url). Takes 3–10 s.",
		Schema: tools.Object(map[string]any{"query": tools.Str("what to look up")}, "query"),
		Handler: tools.Bind(func(ctx context.Context, in struct {
			Query string `json:"query"`
		}) (tools.Output, error) {
			if strings.TrimSpace(in.Query) == "" {
				return tools.Output{}, errors.New("query is empty")
			}
			r, err := w.Search(ctx, c, in.Query)
			if err != nil {
				return tools.Data(map[string]any{"result": "error", "reason": err.Error()})
			}
			return tools.Data(r)
		}),
	}}
}

type citation struct {
	Type        string `json:"type"`
	URLCitation struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	} `json:"url_citation"`
}

type reply struct {
	Choices []struct {
		Message struct {
			Content     string     `json:"content"`
			Annotations []citation `json:"annotations"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Search runs one web-plugin request and returns {answer, sources}.
func (w *Web) Search(ctx context.Context, c *config.Store, query string) (map[string]any, error) {
	key := c.Secret("agent.api_key")
	if key == "" {
		return nil, errors.New("set agent.api_key (OpenRouter)")
	}
	model := c.String(keyModel)
	if model == "" {
		model = c.String("agent.model")
	}
	n := int(c.Float(keyResults))
	if n < 1 {
		n = 5
	}
	now := time.Now()
	if l, err := time.LoadLocation(c.String("home.timezone")); err == nil {
		now = now.In(l)
	}
	sys := "You summarize web results for a home robot that will say the answer aloud. " +
		"Answer in the language of the question, in at most 5 short sentences, with concrete numbers and dates. " +
		"Say so if the results do not answer it. Today is " + now.Format("Mon 2 Jan 2006 15:04") + "."
	if city := c.String("home.city"); city != "" {
		sys += " The user lives in " + city + "."
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": query},
		},
		"plugins":    []map[string]any{{"id": "web", "max_results": n}},
		"max_tokens": 600,
	})
	url := strings.TrimRight(c.String("agent.base_url"), "/") + "/chat/completions"
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "BB-8")
	res, err := w.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	var out reply
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("search HTTP %d: %.200s", res.StatusCode, b)
	}
	if out.Error != nil {
		return nil, errors.New("search: " + out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("search returned nothing (HTTP %d)", res.StatusCode)
	}
	m := out.Choices[0].Message
	sources := []map[string]string{}
	seen := map[string]bool{}
	for _, a := range m.Annotations {
		u := a.URLCitation.URL
		if a.Type != "url_citation" || u == "" || seen[u] {
			continue
		}
		seen[u] = true
		sources = append(sources, map[string]string{"title": a.URLCitation.Title, "url": u})
	}
	return map[string]any{"answer": strings.TrimSpace(m.Content), "sources": sources}, nil
}
