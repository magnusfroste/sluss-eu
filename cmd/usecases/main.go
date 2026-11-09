// Command usecases runs the use cases in examples/use-cases/cases.yaml
// against a running Sluss and checks each outcome: cloud, local, mask or
// block. It asks the dry-run (POST /router/decision), which calls no model,
// and for a "mask" case also sends the question through /v1/chat/completions
// to see the X-Router-Masked header a real request gets.
//
// It is how a person running Sluss, or an agent evaluating it, sees in one
// table what Sluss does with realistic questions. `make use-cases` starts a
// fresh instance with test providers and runs it.
//
// Usage:
//
//	usecases -url http://localhost:8080 -key <api key> [-cases file] [-local anthropic] [-markdown]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type useCase struct {
	ID      string   `yaml:"id"`
	Who     string   `yaml:"who"`
	Story   string   `yaml:"story"`
	Prompt  string   `yaml:"prompt"`
	Tools   []string `yaml:"tools"`
	Outcome string   `yaml:"outcome"`
	Classes []string `yaml:"classes"`
	Masked  []string `yaml:"masked"`
}

type result struct {
	c        useCase
	outcome  string
	classes  []string
	masked   []string
	model    string
	toolRisk string
	ms       int64
	problem  string
}

func main() {
	base := flag.String("url", envOr("ROUTER_URL", "http://localhost:8080"), "Sluss base URL")
	key := flag.String("key", envOr("ROUTER_API_KEY", os.Getenv("LOCAL_API_KEY")), "API key (Bearer)")
	file := flag.String("cases", "examples/use-cases/cases.yaml", "use-case file")
	localProv := flag.String("local", "anthropic", "comma-separated provider IDs that are your own hardware")
	markdown := flag.Bool("markdown", false, "print a Markdown table (for the README)")
	flag.Parse()

	raw, err := os.ReadFile(*file)
	if err != nil {
		fail("read %s: %v", *file, err)
	}
	var doc struct {
		Cases []useCase `yaml:"cases"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		fail("parse %s: %v", *file, err)
	}
	local := map[string]bool{}
	for _, p := range strings.Split(*localProv, ",") {
		local[strings.TrimSpace(p)] = true
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var results []result
	failed := 0
	for _, c := range doc.Cases {
		r := run(client, strings.TrimRight(*base, "/"), *key, c, local)
		if r.problem != "" {
			failed++
		}
		results = append(results, r)
	}
	if *markdown {
		printMarkdown(results)
	} else {
		printText(results)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d of %d use cases did not get the expected outcome\n", failed, len(results))
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "\nUSE CASES PASS — %d of %d as expected\n", len(results), len(results))
}

func run(client *http.Client, base, key string, c useCase, local map[string]bool) result {
	r := result{c: c}
	body := map[string]any{"model": "auto", "messages": []map[string]string{{"role": "user", "content": c.Prompt}}}
	if len(c.Tools) > 0 {
		var tools []map[string]any
		for _, t := range c.Tools {
			tools = append(tools, map[string]any{"type": "function", "function": map[string]any{"name": t, "parameters": map[string]any{"type": "object"}}})
		}
		body["tools"] = tools
	}
	start := time.Now()
	status, hdr, data, err := post(client, base+"/router/decision", key, body, true)
	r.ms = time.Since(start).Milliseconds()
	if err != nil {
		r.problem = err.Error()
		return r
	}
	var dec struct {
		SelectedModel    string   `json:"selected_model"`
		SelectedProvider string   `json:"selected_provider"`
		DecisionReasons  []string `json:"decision_reasons"`
		BlockCode        string   `json:"block_code"`
		Job              *struct {
			Sensitivities []string `json:"Sensitivities"`
			ToolRisk      string   `json:"ToolRisk"`
		} `json:"job"`
	}
	_ = json.Unmarshal(data, &dec)
	_ = hdr
	switch {
	case status == http.StatusForbidden:
		r.outcome = "block"
	case status != http.StatusOK:
		r.problem = fmt.Sprintf("HTTP %d: %s", status, strings.TrimSpace(string(data)))
		return r
	default:
		r.model = dec.SelectedModel
		r.outcome = "cloud"
		if local[dec.SelectedProvider] {
			r.outcome = "local"
		}
		for _, reason := range dec.DecisionReasons {
			if strings.HasPrefix(reason, "mask and send: ") && !strings.Contains(reason, "nothing is masked") {
				r.outcome = "mask"
				plan := strings.TrimPrefix(reason, "mask and send: ")
				plan = strings.SplitN(plan, " ", 2)[0]
				for _, part := range strings.Split(plan, ",") {
					r.masked = append(r.masked, strings.SplitN(part, ":", 2)[0])
				}
			}
		}
	}
	if dec.Job != nil {
		r.classes = dec.Job.Sensitivities
		r.toolRisk = dec.Job.ToolRisk
	}

	// A real request for a mask case: the header must say what was masked.
	if r.outcome == "mask" {
		st, h, d, err := post(client, base+"/v1/chat/completions", key, body, false)
		switch {
		case err != nil:
			r.problem = "chat: " + err.Error()
		case st != http.StatusOK:
			r.problem = fmt.Sprintf("chat HTTP %d: %s", st, strings.TrimSpace(string(d)))
		case h.Get("X-Router-Masked") == "":
			r.problem = "the chat request was not masked (no X-Router-Masked header)"
		}
	}

	if r.problem == "" && r.outcome != c.Outcome {
		r.problem = fmt.Sprintf("expected %s, got %s", c.Outcome, r.outcome)
	}
	if r.problem == "" {
		have := map[string]bool{}
		for _, cl := range r.classes {
			have[cl] = true
		}
		for _, want := range c.Classes {
			if r.outcome != "block" && !have[want] {
				r.problem = fmt.Sprintf("expected data class %s, found %v", want, r.classes)
			}
		}
		maskedHave := map[string]bool{}
		for _, m := range r.masked {
			maskedHave[m] = true
		}
		for _, want := range c.Masked {
			if !maskedHave[want] {
				r.problem = fmt.Sprintf("expected %s to be masked, masked %v", want, r.masked)
			}
		}
	}
	return r
}

func post(client *http.Client, url, key string, body any, explain bool) (int, http.Header, []byte, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	if explain {
		req.Header.Set("X-Router-Explain", "true")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, data, nil
}

func printText(results []result) {
	for _, r := range results {
		mark := "ok  "
		if r.problem != "" {
			mark = "FAIL"
		}
		extra := ""
		if len(r.masked) > 0 {
			extra = " masked=" + strings.Join(r.masked, ",")
		}
		fmt.Printf("%s %-24s %-18s %-6s classes=%-28s%s\n", mark, r.c.ID, r.c.Who, r.outcome, strings.Join(r.classes, ","), extra)
		if r.problem != "" {
			fmt.Printf("     %s\n", r.problem)
		}
	}
}

var outcomeWords = map[string]string{
	"cloud": "cloud model",
	"local": "own hardware",
	"mask":  "cloud, masked",
	"block": "blocked",
}

func printMarkdown(results []result) {
	fmt.Println("| Who | Asks | Sluss finds | Outcome |")
	fmt.Println("|---|---|---|---|")
	for _, r := range results {
		found := strings.Join(r.classes, ", ")
		if found == "" {
			found = "—"
		}
		if r.outcome == "block" {
			found = "security_classified (and more)"
		}
		if r.toolRisk != "" && r.toolRisk != "none" {
			if found == "—" {
				found = "agent tools: " + r.toolRisk
			} else {
				found += "; agent tools: " + r.toolRisk
			}
		}
		out := outcomeWords[r.outcome]
		if len(r.masked) > 0 {
			sort.Strings(r.masked)
			out += " (" + strings.Join(r.masked, ", ") + ")"
		}
		if r.problem != "" {
			out += " ⚠ " + r.problem
		}
		ask := r.c.Prompt
		if i := strings.IndexAny(ask, "\n"); i > 0 {
			ask = ask[:i] + " …"
		}
		if rs := []rune(ask); len(rs) > 70 {
			ask = string(rs[:67]) + "…"
		}
		if len(r.c.Tools) > 0 {
			ask += " *(tools: " + strings.Join(r.c.Tools, ", ") + ")*"
		}
		fmt.Printf("| %s | %s | %s | %s |\n", r.c.Who, strings.ReplaceAll(ask, "|", "\\|"), found, out)
	}
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func fail(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "usecases: "+f+"\n", a...)
	os.Exit(2)
}
