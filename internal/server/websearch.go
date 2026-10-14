package server

// Web search for the company chat (ISSUE-129). People use ChatGPT for "look
// this up"; a sanctioned chat has to do the same or it will not be used. The
// search runs through a search engine the organisation chooses — the
// documented default is a self-hosted SearXNG, so a query never becomes
// someone else's data — and it is gated like everything else here:
//
//   - the user turns it on per message ("search": true in the chat send);
//   - the prompt is classified first, deterministically, before anything
//     leaves: if it carries personal, financial, health, legal, security or
//     secret-looking data the search is skipped and the chat says so;
//   - results are handed to the model as a system message with numbered
//     sources, and the same sources go back to the browser in a header so
//     the answer can show where it came from;
//   - what is audited is actor, outcome and result count — never the query.
//
// The routing decision itself is untouched: the enriched request goes
// through the normal classifier, policy and engine like any other.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// SearchOptions configures the chat's web search. URL is a SearXNG-compatible
// search endpoint (ROUTER_SEARCH_URL, e.g. https://search.internal/search);
// empty disables the feature and the toggle reports "off".
type SearchOptions struct {
	URL     string
	Auditor audit.Sink
	Logger  *slog.Logger
	Client  *http.Client
	// MaxResults caps what is handed to the model and the browser (default 5).
	MaxResults int
}

// ActionChatSearch audits a web search from the chat: who, outcome, how
// many results. The query is content and is never recorded.
const ActionChatSearch = audit.Action("chat.search")

// searchSource is one result as the browser sees it.
type searchSource struct {
	Title string `json:"t"`
	URL   string `json:"u"`
}

const (
	hdrSearch  = "X-Router-Search"  // off | skipped-sensitive | done;N | failed | empty
	hdrSources = "X-Router-Sources" // base64url(JSON [{t,u}])
)

// Enabled reports whether a search endpoint is configured.
func (o SearchOptions) Enabled() bool { return strings.TrimSpace(o.URL) != "" }

// Middleware wraps the chat send endpoint. It reads the body once, runs the
// search when asked and allowed, prepends the results as a system message and
// forwards the (possibly enriched) request to the chat handler.
func (o SearchOptions) Middleware(next http.Handler) http.Handler {
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 8 * time.Second}
	}
	if o.MaxResults <= 0 {
		o.MaxResults = 5
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		var probe struct {
			Search bool `json:"search"`
		}
		_ = json.Unmarshal(body, &probe)
		if !probe.Search {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}
		if !o.Enabled() {
			w.Header().Set(hdrSearch, "off")
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		// Classify the latest user message before anything leaves: the same
		// deterministic extractor the router uses, on the same request shape.
		var req openai.ChatRequest
		if err := json.Unmarshal(body, &req); err != nil || len(req.Messages) == 0 {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}
		query := lastUserText(req.Messages)
		var auth router.AuthTenantContext
		if t, ok := tenant.FromContext(r.Context()); ok {
			auth.TenantID, auth.ProjectID = t.ID, t.Project
		}
		job := router.NewJobDescriptor(router.JobDescriptorInput{
			Auth: auth, Headers: r.Header,
			Request: &openai.ChatRequest{Messages: []openai.Message{{Role: "user", Content: query}}},
		})
		actor := AdminUserFromContext(r.Context())
		if job.Sensitivity != router.SensitivityNone || strings.TrimSpace(query) == "" {
			w.Header().Set(hdrSearch, "skipped-sensitive;"+string(job.Sensitivity))
			audit.Record(r.Context(), o.Auditor, audit.Entry{
				Action: ActionChatSearch, Actor: actor, TenantID: auth.TenantID, Outcome: audit.OutcomeBlocked,
				Reason: "prompt classified as " + string(job.Sensitivity) + "; nothing left the gateway",
			})
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		results, err := o.search(r.Context(), query)
		switch {
		case err != nil:
			if o.Logger != nil {
				o.Logger.Warn("chat web search failed", "err", err)
			}
			w.Header().Set(hdrSearch, "failed")
			audit.Record(r.Context(), o.Auditor, audit.Entry{
				Action: ActionChatSearch, Actor: actor, TenantID: auth.TenantID, Outcome: audit.OutcomeFailure, Reason: err.Error(),
			})
			r.Body = io.NopCloser(bytes.NewReader(body))
		case len(results) == 0:
			w.Header().Set(hdrSearch, "empty")
			audit.Record(r.Context(), o.Auditor, audit.Entry{
				Action: ActionChatSearch, Actor: actor, TenantID: auth.TenantID, Detail: map[string]string{"results": "0"},
			})
			r.Body = io.NopCloser(bytes.NewReader(body))
		default:
			enriched, err := injectSystemMessage(body, resultsPrompt(results))
			if err != nil {
				w.Header().Set(hdrSearch, "failed")
				r.Body = io.NopCloser(bytes.NewReader(body))
				break
			}
			w.Header().Set(hdrSearch, "done;"+itoa(len(results)))
			w.Header().Set(hdrSources, encodeSources(results))
			audit.Record(r.Context(), o.Auditor, audit.Entry{
				Action: ActionChatSearch, Actor: actor, TenantID: auth.TenantID, Detail: map[string]string{"results": itoa(len(results))},
			})
			r.Body = io.NopCloser(bytes.NewReader(enriched))
			r.ContentLength = int64(len(enriched))
		}
		next.ServeHTTP(w, r)
	})
}

// lastUserText is the text of the most recent user message (array content
// already concatenated by the decoder).
func lastUserText(msgs []openai.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return ""
}

type searchResult struct {
	Title, URL, Snippet string
}

// search calls a SearXNG-compatible endpoint (?q=…&format=json) and returns
// up to MaxResults results.
func (o SearchOptions) search(ctx context.Context, q string) ([]searchResult, error) {
	u, err := url.Parse(strings.TrimSpace(o.URL))
	if err != nil {
		return nil, err
	}
	qs := u.Query()
	qs.Set("q", q)
	qs.Set("format", "json")
	u.RawQuery = qs.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &searchError{"search endpoint answered HTTP " + itoa(resp.StatusCode)}
	}
	var out struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out); err != nil {
		return nil, err
	}
	var results []searchResult
	for _, r := range out.Results {
		if strings.TrimSpace(r.URL) == "" {
			continue
		}
		results = append(results, searchResult{
			Title: clip(r.Title, 120), URL: clip(r.URL, 300), Snippet: clip(r.Content, 600),
		})
		if len(results) == o.MaxResults {
			break
		}
	}
	return results, nil
}

type searchError struct{ msg string }

func (e *searchError) Error() string { return e.msg }

// resultsPrompt is the system message the model gets.
func resultsPrompt(results []searchResult) string {
	var b strings.Builder
	b.WriteString("Web search results fetched by the company gateway for the user's latest question. Use them when they are relevant, cite sources by their number like [1], and say so when they do not answer the question.\n")
	for i, r := range results {
		b.WriteString("\n[" + itoa(i+1) + "] " + r.Title + " — " + r.URL + "\n" + r.Snippet + "\n")
	}
	return b.String()
}

// injectSystemMessage prepends a system message at the raw JSON level so the
// rest of the request (array content, tools, model pin) is forwarded exactly
// as the browser sent it.
func injectSystemMessage(body []byte, text string) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	var msgs []json.RawMessage
	if err := json.Unmarshal(raw["messages"], &msgs); err != nil {
		return nil, err
	}
	sys, _ := json.Marshal(map[string]string{"role": "system", "content": text})
	msgs = append([]json.RawMessage{sys}, msgs...)
	joined, _ := json.Marshal(msgs)
	raw["messages"] = joined
	delete(raw, "search")
	return json.Marshal(raw)
}

func encodeSources(results []searchResult) string {
	out := make([]searchSource, 0, len(results))
	for _, r := range results {
		out = append(out, searchSource{Title: clip(r.Title, 80), URL: r.URL})
	}
	j, _ := json.Marshal(out)
	return base64.RawURLEncoding.EncodeToString(j)
}

func itoa(n int) string { return strconv.Itoa(n) }
