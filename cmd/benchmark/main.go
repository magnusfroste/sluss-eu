// Command benchmark measures how well Sluss finds sensitive data, on a
// labelled set of synthetic questions (benchmark/corpus.jsonl), so the
// numbers can be checked, repeated and compared with other tools.
//
//	benchmark run    [-corpus f] [-url u -key k] [-repeat n] [-out predictions.jsonl] [-name label]
//	benchmark score  [-corpus f] [-markdown] predictions.jsonl [more.jsonl ...]
//	benchmark export [-corpus f] -dir out/
//
// run classifies every question. Without -url it uses the rules in this
// binary (rules only, no network, the same code the router runs). With -url
// it asks a running Sluss through the dry-run (POST /router/decision), which
// calls no model for the answer but does ask a configured second opinion.
//
// score compares predictions with the labels: per class recall and
// precision, the share of harmless questions flagged, recall per kind of
// case (identifier, term, meaning) and latency. Predictions from another
// tool in the same format ({"id","flagged","classes"}) are scored the same
// way; that is how a gateway such as Zscaler or Netskope, or Presidio, is
// compared without anyone's numbers being copied from a brochure.
//
// export writes each question as a text file and a CSV, for tools that are
// tested by uploading files or pasting prompts.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
	"github.com/magnusfroste/sluss/internal/secondopinion"
)

type benchCase struct {
	ID     string   `json:"id"`
	Text   string   `json:"text"`
	Labels []string `json:"labels"`
	Kind   string   `json:"kind"`
	Lang   string   `json:"lang,omitempty"`
}

type prediction struct {
	ID      string   `json:"id"`
	Flagged bool     `json:"flagged"`
	Classes []string `json:"classes"`
	Ms      float64  `json:"ms,omitempty"`
	// Varied is set when repeated runs of the same question disagreed.
	Varied bool `json:"varied,omitempty"`
	// Runs is how many times the question was classified (0 or 1: once).
	Runs int    `json:"runs,omitempty"`
	Tool string `json:"tool,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "score":
		scoreCmd(os.Args[2:])
	case "export":
		exportCmd(os.Args[2:])
	case "record":
		recordCmd(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: benchmark run|score|export|record [flags]")
	os.Exit(2)
}

func loadCorpus(path string) []benchCase {
	f, err := os.Open(path)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	var out []benchCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var c benchCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			fail(fmt.Errorf("%s: %w", path, err))
		}
		out = append(out, c)
	}
	if err := sc.Err(); err != nil {
		fail(err)
	}
	return out
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "benchmark:", err)
	os.Exit(1)
}

// ---- run ----

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	corpus := fs.String("corpus", "benchmark/corpus.jsonl", "labelled questions")
	url := fs.String("url", "", "a running Sluss; empty classifies in-process with the rules only")
	key := fs.String("key", os.Getenv("SLUSS_KEY"), "API key for -url (or SLUSS_KEY)")
	repeat := fs.Int("repeat", 1, "classify each question this many times; differences are marked")
	out := fs.String("out", "", "predictions file (default stdout)")
	name := fs.String("name", "", "tool label written into each prediction")
	recorded := fs.String("second-opinion", "", "a recording of the local model's answers (benchmark record): the rules in this binary plus those answers, with no model and no network")
	_ = fs.Parse(args)

	cases := loadCorpus(*corpus)
	classify := classifyLocal
	tool := "sluss rules only"
	if *recorded != "" {
		classify = replayClassifier(cases, *recorded)
		tool = "sluss + second opinion (recorded)"
	}
	if *url != "" {
		classify = remoteClassifier(strings.TrimRight(*url, "/"), *key)
		tool = "sluss " + *url
	}
	if *name != "" {
		tool = *name
	}

	w := io.Writer(os.Stdout)
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fail(err)
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, c := range cases {
		var first []string
		var varied bool
		var total time.Duration
		for i := 0; i < max(1, *repeat); i++ {
			start := time.Now()
			classes, err := classify(c.Text)
			total += time.Since(start)
			if err != nil {
				fail(fmt.Errorf("case %s: %w", c.ID, err))
			}
			sort.Strings(classes)
			if i == 0 {
				first = classes
			} else if strings.Join(classes, ",") != strings.Join(first, ",") {
				varied = true
			}
		}
		if first == nil {
			first = []string{}
		}
		ms := float64(total.Microseconds()) / 1000 / float64(max(1, *repeat))
		_ = enc.Encode(prediction{ID: c.ID, Flagged: len(first) > 0, Classes: first, Ms: ms, Varied: varied, Runs: max(1, *repeat), Tool: tool})
	}
}

func classifyLocal(text string) ([]string, error) {
	job := router.NewJobDescriptor(router.JobDescriptorInput{Request: &openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: text}},
	}})
	return job.AllSensitivities(), nil
}

// postDecision asks the dry-run about one question. A dropped connection or
// a 5xx is retried, so one network hiccup does not end a run of several
// hundred questions. A block (403) is an answer; any other status is an error.
func postDecision(client *http.Client, base, key, text string) ([]byte, error) {
	body, _ := json.Marshal(map[string]any{
		"model":    "auto",
		"messages": []map[string]string{{"role": "user", "content": text}},
	})
	var data []byte
	var status int
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		req, _ := http.NewRequest(http.MethodPost, base+"/router/decision", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Router-Explain", "true")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		status = resp.StatusCode
		if status >= 500 {
			lastErr = fmt.Errorf("HTTP %d: %s", status, strings.TrimSpace(string(data)))
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return nil, lastErr
	}
	if status != http.StatusOK && status != http.StatusForbidden {
		return nil, fmt.Errorf("HTTP %d: %s", status, strings.TrimSpace(string(data)))
	}
	return data, nil
}

// remoteReasons returns the decision's reasons for one question.
func remoteReasons(base, key string) func(string) ([]string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	return func(text string) ([]string, error) {
		data, err := postDecision(client, base, key, text)
		if err != nil {
			return nil, err
		}
		var dec struct {
			DecisionReasons []string `json:"decision_reasons"`
		}
		if err := json.Unmarshal(data, &dec); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		return dec.DecisionReasons, nil
	}
}

func remoteClassifier(base, key string) func(string) ([]string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	return func(text string) ([]string, error) {
		data, err := postDecision(client, base, key, text)
		if err != nil {
			return nil, err
		}
		var dec struct {
			Job *struct {
				Sensitivity   string   `json:"Sensitivity"`
				Sensitivities []string `json:"Sensitivities"`
			} `json:"job"`
			DecisionReasons []string `json:"decision_reasons"`
		}
		if err := json.Unmarshal(data, &dec); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		var found []string
		if dec.Job != nil {
			found = append([]string{dec.Job.Sensitivity}, dec.Job.Sensitivities...)
		} else {
			// A block carries no job. The rules' classes are the same code as
			// this binary's (run it from the commit the instance runs, see
			// GET /version); the second opinion's are in decision_reasons.
			found, _ = classifyLocal(text)
			found = append(found, secondOpinionClasses(dec.DecisionReasons)...)
		}
		seen := map[string]bool{}
		var out []string
		for _, s := range found {
			if s != "" && s != "none" && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
		return out, nil
	}
}

// secondOpinionClasses reads the classes from the reason line
// "second opinion (local model m): pii, health — may only make ...".
func secondOpinionClasses(reasons []string) []string {
	var out []string
	for _, r := range reasons {
		if !strings.HasPrefix(r, "second opinion (local model ") {
			continue
		}
		i := strings.Index(r, "): ")
		if i < 0 {
			continue
		}
		list := r[i+3:]
		if j := strings.Index(list, " — "); j >= 0 {
			list = list[:j]
		}
		for _, c := range strings.Split(list, ",") {
			if c = strings.TrimSpace(c); c != "" {
				out = append(out, c)
			}
		}
	}
	return out
}

// ---- record and replay (ISSUE-170) ----
//
// The second opinion is a model, so CI cannot ask it. Instead its answers are
// recorded once against a running instance and replayed: the rules in this
// binary, plus what the model said. That measures today's rules together with
// the model, catches a rule change that makes the combination worse, and
// fails loudly when the recording no longer matches the questions or the
// instruction the model is given.

type recordingHeader struct {
	Recording  string `json:"recording"` // "second-opinion"
	Model      string `json:"model"`
	PromptHash string `json:"prompt_hash"`
	Commit     string `json:"commit,omitempty"`
	Recorded   string `json:"recorded"`
	Questions  int    `json:"questions"`
}

type recordedAnswer struct {
	ID     string   `json:"id"`
	Text   string   `json:"text_hash"`
	Labels []string `json:"labels"`
}

func textHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:6])
}

func recordCmd(args []string) {
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	corpus := fs.String("corpus", "benchmark/corpus.jsonl", "labelled questions")
	url := fs.String("url", "", "a running Sluss with a second opinion configured")
	key := fs.String("key", os.Getenv("SLUSS_KEY"), "API key (or SLUSS_KEY)")
	out := fs.String("out", "benchmark/second-opinion.jsonl", "recording to write")
	_ = fs.Parse(args)
	if *url == "" {
		fail(fmt.Errorf("record: -url is required"))
	}
	base := strings.TrimRight(*url, "/")
	cases := loadCorpus(*corpus)

	var version struct {
		Commit    string `json:"commit"`
		Decisions string `json:"decisions"`
	}
	if resp, err := http.Get(base + "/version"); err == nil {
		_ = json.NewDecoder(resp.Body).Decode(&version)
		resp.Body.Close()
	}
	if version.Decisions != "rules+second-opinion" {
		fail(fmt.Errorf("record: %s decides with %q; a second opinion must be configured", base, version.Decisions))
	}

	ask := remoteReasons(base, *key)
	answers := make([]recordedAnswer, 0, len(cases))
	model := ""
	for _, c := range cases {
		reasons, err := ask(c.Text)
		if err != nil {
			fail(fmt.Errorf("case %s: %w", c.ID, err))
		}
		labels := secondOpinionClasses(reasons)
		if m := secondOpinionModel(reasons); m != "" {
			model = m
		}
		sort.Strings(labels)
		if labels == nil {
			labels = []string{}
		}
		answers = append(answers, recordedAnswer{ID: c.ID, Text: textHash(c.Text), Labels: labels})
	}
	f, err := os.Create(*out)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(recordingHeader{Recording: "second-opinion", Model: model, PromptHash: secondopinion.PromptHash(),
		Commit: version.Commit, Recorded: time.Now().UTC().Format("2006-01-02"), Questions: len(answers)})
	for _, a := range answers {
		_ = enc.Encode(a)
	}
	fmt.Printf("recorded %d answers from %s (model %s) to %s\n", len(answers), base, model, *out)
}

func secondOpinionModel(reasons []string) string {
	const p = "second opinion (local model "
	for _, r := range reasons {
		if strings.HasPrefix(r, p) {
			if i := strings.Index(r, "): "); i > len(p) {
				return r[len(p):i]
			}
		}
	}
	return ""
}

// loadRecording reads a recording and checks it against the questions and
// the instruction in this binary.
func loadRecording(cases []benchCase, path string) (recordingHeader, map[string][]string, error) {
	var h recordingHeader
	f, err := os.Open(path)
	if err != nil {
		return h, nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	byID := map[string]recordedAnswer{}
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			if err := json.Unmarshal([]byte(line), &h); err != nil || h.Recording != "second-opinion" {
				return h, nil, fmt.Errorf("%s: the first line is not a recording header", path)
			}
			continue
		}
		var a recordedAnswer
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			return h, nil, fmt.Errorf("%s: %w", path, err)
		}
		byID[a.ID] = a
	}
	const again = "record it again: go run ./cmd/benchmark record -url <instance> -key <key>"
	if h.PromptHash != secondopinion.PromptHash() {
		return h, nil, fmt.Errorf("%s was recorded with another instruction to the model (%s, now %s); %s", path, h.PromptHash, secondopinion.PromptHash(), again)
	}
	labels := map[string][]string{}
	for _, c := range cases {
		a, ok := byID[c.ID]
		if !ok || a.Text != textHash(c.Text) {
			return h, nil, fmt.Errorf("%s does not match the questions (case %s); %s", path, c.ID, again)
		}
		labels[c.Text] = a.Labels
	}
	return h, labels, nil
}

func replayClassifier(cases []benchCase, path string) func(string) ([]string, error) {
	_, labels, err := loadRecording(cases, path)
	if err != nil {
		fail(err)
	}
	return func(text string) ([]string, error) {
		// Raise only, as in the router: the rules' classes plus the model's.
		out, _ := classifyLocal(text)
		seen := set(out)
		for _, l := range labels[text] {
			if !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
		return out, nil
	}
}

// ---- score ----

type tally struct{ tp, fp, fn int }

func (t tally) recall() float64    { return ratio(t.tp, t.tp+t.fn) }
func (t tally) precision() float64 { return ratio(t.tp, t.tp+t.fp) }

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

type report struct {
	tool                  string
	sensitive, caught     int
	harmless, overflagged int
	exact                 int
	perClass              map[string]*tally
	perKind               map[string][2]int // caught, total
	varied                int
	repeated              bool
	ms                    []float64
	missing               int
	misses, falseAlarms   []benchCase
}

func score(cases []benchCase, preds map[string]prediction) report {
	r := report{perClass: map[string]*tally{}, perKind: map[string][2]int{}}
	for _, c := range cases {
		p, ok := preds[c.ID]
		if !ok {
			r.missing++
			continue
		}
		if r.tool == "" {
			r.tool = p.Tool
		}
		if p.Varied {
			r.varied++
		}
		if p.Runs > 1 {
			r.repeated = true
		}
		if p.Ms > 0 {
			r.ms = append(r.ms, p.Ms)
		}
		harmless := len(c.Labels) == 0 || len(c.Labels) == 1 && c.Labels[0] == "harmless"
		kc := r.perKind[c.Kind]
		kc[1]++
		if harmless {
			r.harmless++
			if p.Flagged {
				r.overflagged++
				r.falseAlarms = append(r.falseAlarms, c)
			} else {
				kc[0]++ // for harmless kinds, "caught" means left alone
			}
		} else {
			r.sensitive++
			if p.Flagged {
				r.caught++
				kc[0]++
			} else {
				r.misses = append(r.misses, c)
			}
		}
		r.perKind[c.Kind] = kc

		want := set(c.Labels)
		got := set(p.Classes)
		delete(want, "harmless")
		if len(want) > 0 && sameSet(want, got) || len(want) == 0 && len(got) == 0 {
			r.exact++
		}
		for cl := range want {
			t := ensure(r.perClass, cl)
			if got[cl] {
				t.tp++
			} else {
				t.fn++
			}
		}
		for cl := range got {
			if !want[cl] {
				ensure(r.perClass, cl).fp++
			}
		}
	}
	return r
}

func ensure(m map[string]*tally, k string) *tally {
	if m[k] == nil {
		m[k] = &tally{}
	}
	return m[k]
}

func set(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	i := int(p*float64(len(s)-1) + 0.5)
	return s[i]
}

var classOrder = []string{"pii", "secrets_possible", "security_classified", "health", "financial", "legal", "source_code"}
var kindOrder = []string{"identifier", "term", "meaning", "lookalike", "plain"}

func scoreCmd(args []string) {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	corpus := fs.String("corpus", "benchmark/corpus.jsonl", "labelled questions")
	markdown := fs.Bool("markdown", false, "print Markdown tables")
	misses := fs.Bool("misses", false, "list the questions missed and the harmless ones flagged")
	minRecall := fs.Float64("min-recall", 0, "exit 1 if overall recall of the first file is below this (0-1)")
	maxFPR := fs.Float64("max-fpr", 1, "exit 1 if the first file flags more than this share of harmless questions (0-1)")
	_ = fs.Parse(args)
	if fs.NArg() == 0 {
		fail(fmt.Errorf("score: give one or more predictions files"))
	}
	cases := loadCorpus(*corpus)
	var reports []report
	for _, path := range fs.Args() {
		preds := loadPredictions(path)
		r := score(cases, preds)
		if r.tool == "" {
			r.tool = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		reports = append(reports, r)
	}
	if *markdown {
		printMarkdown(reports, len(cases))
	} else {
		printText(reports, len(cases))
	}
	if *misses {
		for _, r := range reports {
			fmt.Printf("\n%s: missed (%d)\n", r.tool, len(r.misses))
			for _, c := range r.misses {
				fmt.Printf("  %s %-20s %-10s %s\n", c.ID, strings.Join(c.Labels, ","), c.Kind, c.Text)
			}
			fmt.Printf("%s: harmless but flagged (%d)\n", r.tool, len(r.falseAlarms))
			for _, c := range r.falseAlarms {
				fmt.Printf("  %s %-10s %s\n", c.ID, c.Kind, c.Text)
			}
		}
	}
	first := reports[0]
	if ratio(first.caught, first.sensitive) < *minRecall || ratio(first.overflagged, first.harmless) > *maxFPR || first.missing > 0 {
		fmt.Fprintf(os.Stderr, "benchmark: below threshold (recall %.3f, harmless flagged %.3f, missing %d)\n",
			ratio(first.caught, first.sensitive), ratio(first.overflagged, first.harmless), first.missing)
		os.Exit(1)
	}
}

func loadPredictions(path string) map[string]prediction {
	f, err := os.Open(path)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	out := map[string]prediction{}
	if strings.HasSuffix(path, ".csv") {
		// The filled-in predictions-template.csv from export: id, flagged
		// (yes/no, true/false, 1/0), classes separated by ";".
		rows, err := csv.NewReader(f).ReadAll()
		if err != nil {
			fail(fmt.Errorf("%s: %w", path, err))
		}
		for i, row := range rows {
			if i == 0 || len(row) < 2 {
				continue
			}
			p := prediction{ID: row[0]}
			switch strings.ToLower(strings.TrimSpace(row[1])) {
			case "yes", "true", "1", "y", "ja", "blocked", "flagged":
				p.Flagged = true
			}
			if len(row) > 2 && strings.TrimSpace(row[2]) != "" {
				p.Classes = strings.Split(strings.TrimSpace(row[2]), ";")
				p.Flagged = true
			}
			out[p.ID] = p
		}
		return out
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var p prediction
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			fail(fmt.Errorf("%s: %w", path, err))
		}
		if !p.Flagged && len(p.Classes) > 0 {
			p.Flagged = true
		}
		out[p.ID] = p
	}
	return out
}

func pct(a, b int) string {
	if b == 0 {
		return "—"
	}
	return fmt.Sprintf("%d of %d (%.0f %%)", a, b, 100*ratio(a, b))
}

func printText(reports []report, n int) {
	for _, r := range reports {
		fmt.Printf("%s — %d questions\n", r.tool, n)
		fmt.Printf("  sensitive flagged      %s\n", pct(r.caught, r.sensitive))
		fmt.Printf("  harmless flagged       %s\n", pct(r.overflagged, r.harmless))
		fmt.Printf("  exact classes          %s\n", pct(r.exact, n))
		for _, k := range kindOrder {
			if v, ok := r.perKind[k]; ok {
				label := "flagged"
				if k == "lookalike" || k == "plain" {
					label = "left alone"
				}
				fmt.Printf("  %-12s %-9s %s\n", k, label, pct(v[0], v[1]))
			}
		}
		for _, cl := range classOrder {
			if t := r.perClass[cl]; t != nil {
				fmt.Printf("  %-20s recall %.2f  precision %.2f  (tp %d fn %d fp %d)\n", cl, t.recall(), t.precision(), t.tp, t.fn, t.fp)
			}
		}
		if len(r.ms) > 0 {
			fmt.Printf("  latency                p50 %.2f ms  p95 %.2f ms  max %.2f ms\n", percentile(r.ms, .5), percentile(r.ms, .95), percentile(r.ms, 1))
		}
		if r.repeated {
			fmt.Printf("  varied between runs    %d\n", r.varied)
		}
		if r.missing > 0 {
			fmt.Printf("  NO PREDICTION          %d\n", r.missing)
		}
		fmt.Println()
	}
}

func printMarkdown(reports []report, n int) {
	head := "| | " + join(reports, func(r report) string { return r.tool }) + " |"
	fmt.Println(head)
	fmt.Println("|---|" + strings.Repeat("---|", len(reports)))
	row := func(label string, f func(report) string) {
		fmt.Printf("| %s | %s |\n", label, join(reports, f))
	}
	row("Sensitive questions flagged", func(r report) string { return pct(r.caught, r.sensitive) })
	row("Harmless questions flagged", func(r report) string { return pct(r.overflagged, r.harmless) })
	for _, k := range []string{"identifier", "term", "meaning"} {
		k := k
		row("— with "+kindLabel(k), func(r report) string { v := r.perKind[k]; return pct(v[0], v[1]) })
	}
	for _, k := range []string{"lookalike", "plain"} {
		k := k
		row("— "+kindLabel(k)+" left alone", func(r report) string { v := r.perKind[k]; return pct(v[0], v[1]) })
	}
	row("Right class, exactly", func(r report) string { return pct(r.exact, n) })
	row("Same answer every run", func(r report) string {
		if !r.repeated {
			return "not measured"
		}
		if r.varied == 0 {
			return "yes"
		}
		return fmt.Sprintf("%d varied", r.varied)
	})
	row("Time per question (median / p95)", func(r report) string {
		if len(r.ms) == 0 {
			return "—"
		}
		return fmt.Sprintf("%s / %s", ms(percentile(r.ms, .5)), ms(percentile(r.ms, .95)))
	})
	fmt.Println()
	fmt.Println("| Class | " + join(reports, func(r report) string { return r.tool + " recall / precision" }) + " |")
	fmt.Println("|---|" + strings.Repeat("---|", len(reports)))
	for _, cl := range classOrder {
		cl := cl
		row2 := join(reports, func(r report) string {
			t := r.perClass[cl]
			if t == nil {
				return "—"
			}
			return fmt.Sprintf("%.2f / %.2f", t.recall(), t.precision())
		})
		fmt.Printf("| %s | %s |\n", cl, row2)
	}
}

func ms(v float64) string {
	if v < 1 {
		return fmt.Sprintf("%.2f ms", v)
	}
	if v < 1000 {
		return fmt.Sprintf("%.0f ms", v)
	}
	return fmt.Sprintf("%.1f s", v/1000)
}

func kindLabel(k string) string {
	switch k {
	case "identifier":
		return "an identifier or key"
	case "term":
		return "a term or marking"
	case "meaning":
		return "meaning only"
	case "lookalike":
		return "look-alikes"
	case "plain":
		return "plain questions"
	}
	return k
}

func join(rs []report, f func(report) string) string {
	parts := make([]string, len(rs))
	for i, r := range rs {
		parts[i] = f(r)
	}
	return strings.Join(parts, " | ")
}

// ---- export ----

func exportCmd(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	corpus := fs.String("corpus", "benchmark/corpus.jsonl", "labelled questions")
	dir := fs.String("dir", "benchmark-export", "output directory")
	_ = fs.Parse(args)
	cases := loadCorpus(*corpus)
	if err := os.MkdirAll(filepath.Join(*dir, "questions"), 0o755); err != nil {
		fail(err)
	}
	f, err := os.Create(filepath.Join(*dir, "questions.csv"))
	if err != nil {
		fail(err)
	}
	cw := csv.NewWriter(f)
	_ = cw.Write([]string{"id", "text", "labels", "kind"})
	for _, c := range cases {
		_ = cw.Write([]string{c.ID, c.Text, strings.Join(c.Labels, ";"), c.Kind})
		if err := os.WriteFile(filepath.Join(*dir, "questions", c.ID+".txt"), []byte(c.Text+"\n"), 0o644); err != nil {
			fail(err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		fail(err)
	}
	_ = f.Close()
	tmpl := filepath.Join(*dir, "predictions-template.csv")
	t, err := os.Create(tmpl)
	if err != nil {
		fail(err)
	}
	tw := csv.NewWriter(t)
	_ = tw.Write([]string{"id", "flagged", "classes"})
	for _, c := range cases {
		_ = tw.Write([]string{c.ID, "", ""})
	}
	tw.Flush()
	_ = t.Close()
	fmt.Printf("wrote %d questions to %s (questions.csv, questions/*.txt, predictions-template.csv)\n", len(cases), *dir)
}
