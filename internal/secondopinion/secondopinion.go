// Package secondopinion asks a model on the operator's own hardware which
// kinds of data a prompt holds, and lets that answer make the routing
// decision stricter — never looser (ISSUE-163).
//
// The rules stay the floor and the primary reason: they run first, they decide
// alone when this is off or silent, and nothing the model says can remove a
// class the rules found or lower the risk. The model may only add a class the
// rules did not see — a name with context, an illness in plain words, a deal
// described without a financial term. Its verdict is written next to the rule's
// in the decision reasons and the inspector, as "second opinion".
//
// Only a roster model tagged local (or on-prem, air-gapped) is accepted, so
// the prompt never leaves the machine to be classified. A slow or broken
// model is skipped after a short timeout: the rules have already decided.
package secondopinion

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// Config is read from the environment at start.
type Config struct {
	// ModelID is the roster id of the local model to ask (ROUTER_SECOND_OPINION_MODEL).
	// Empty disables the second opinion.
	ModelID string
	// Timeout bounds one call (ROUTER_SECOND_OPINION_TIMEOUT, default 500ms).
	// Past it the rules' decision stands alone.
	Timeout time.Duration
}

// DefaultTimeout keeps a slow model from holding a request: the rules have
// already decided, the model may only add to it.
const DefaultTimeout = 500 * time.Millisecond

// maxPromptChars bounds what is sent to the model, so a long document costs
// a bounded amount of time. The classes a reader would give a text are
// almost always visible in its first pages.
const maxPromptChars = 6000

// localTags are the compliance tags that mean "runs on hardware the
// organisation controls". The model must carry one of them.
var localTags = map[string]bool{"local": true, "on-prem": true, "air-gapped": true}

// labels maps what the model may answer to the router's classes. The model
// is asked for these words and nothing else.
var labels = map[string]router.Sensitivity{
	"pii":                 router.SensitivityPII,
	"health":              router.SensitivityHealth,
	"financial":           router.SensitivityFinancial,
	"legal":               router.SensitivityLegal,
	"secrets":             router.SensitivitySecretsPossible,
	"security_classified": router.SensitivitySecurityClassified,
	"source_code":         router.SensitivitySourceCode,
}

const systemPrompt = `You classify text for a data-protection gateway. Answer with a JSON array containing zero or more of exactly these labels, and nothing else:
"pii" (information about an identifiable person: a name with details about them, personal numbers, contact details, employee or customer records),
"health" (anyone's illness, treatment, sick leave, diagnosis or mental state),
"financial" (a company's non-public financial data: deals, results before publication, payroll, bank details),
"legal" (contracts, disputes, legal advice),
"secrets" (passwords, API keys, tokens, private keys),
"security_classified" (classified or security-protected material),
"source_code" (program code).
If none apply, answer []. Do not explain.`

var outcomes = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "router_second_opinion_total",
	Help: "Second-opinion calls by outcome: raised (a class was added), silent (nothing to add), timeout, error, skipped (model not usable).",
}, []string{"outcome"})

// Classifier asks the configured local model. A nil *Classifier is off and
// safe to call.
type Classifier struct {
	cfg      Config
	registry *registry.Store
	adapters func() map[string]provider.Adapter
	logger   *slog.Logger
	lastWarn atomic.Int64 // unix seconds; one warning a minute, not one a request
}

// New returns nil when no model is configured.
func New(cfg Config, store *registry.Store, adapters func() map[string]provider.Adapter, logger *slog.Logger) *Classifier {
	if strings.TrimSpace(cfg.ModelID) == "" {
		return nil
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Classifier{cfg: cfg, registry: store, adapters: adapters, logger: logger}
}

// Enabled reports whether a second opinion is configured.
func (c *Classifier) Enabled() bool { return c != nil }

// ModelID is the configured roster model, for the boot banner.
func (c *Classifier) ModelID() string {
	if c == nil {
		return ""
	}
	return c.cfg.ModelID
}

// Result says what one call did. Nothing in it is prompt content.
type Result struct {
	Model   string   // roster model id
	Found   []string // classes the model named, in the router's vocabulary
	Added   []string // classes that were new to the job and raised it
	Skipped string   // why no call was made ("" when a call was made)
	Err     error    // a timeout or a provider error; the job is unchanged
}

// Apply asks the model about the conversation and raises the job's classes
// and risk by what it finds. It never lowers anything and never returns an
// error that should stop the request: the rules have decided; this adds.
func (c *Classifier) Apply(ctx context.Context, job *router.JobDescriptor, messages []openai.Message) Result {
	if c == nil || job == nil {
		return Result{Skipped: "off"}
	}
	res := Result{Model: c.cfg.ModelID}
	adapter, providerModelID, skip := c.resolve()
	if skip != "" {
		res.Skipped = skip
		outcomes.WithLabelValues("skipped").Inc()
		c.warn("second opinion skipped", "model", c.cfg.ModelID, "reason", skip)
		return res
	}
	text := promptText(messages)
	if strings.TrimSpace(text) == "" {
		res.Skipped = "empty prompt"
		return res
	}
	cctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	temp, max := 0.0, 40
	resp, err := adapter.Complete(cctx, &provider.NormalizedModelRequest{
		Model: providerModelID,
		Messages: []openai.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
		Temperature: &temp,
		MaxTokens:   &max,
	})
	if err != nil {
		res.Err = err
		if cctx.Err() != nil {
			outcomes.WithLabelValues("timeout").Inc()
			c.warn("second opinion timed out; rules decided alone", "model", c.cfg.ModelID, "timeout", c.cfg.Timeout.String())
		} else {
			outcomes.WithLabelValues("error").Inc()
			c.warn("second opinion failed; rules decided alone", "model", c.cfg.ModelID, "err", err.Error())
		}
		return res
	}
	answer := ""
	if len(resp.Choices) > 0 {
		answer = resp.Choices[0].Message.Content
	}
	res.Found = parseLabels(answer)
	for _, class := range res.Found {
		if job.RaiseSensitivity(router.Sensitivity(class)) {
			res.Added = append(res.Added, class)
		}
	}
	job.SecondOpinionModel = c.cfg.ModelID
	job.SecondOpinion = res.Found
	if len(res.Added) > 0 {
		outcomes.WithLabelValues("raised").Inc()
	} else {
		outcomes.WithLabelValues("silent").Inc()
	}
	return res
}

// resolve finds the model in the roster and refuses anything not local.
func (c *Classifier) resolve() (provider.Adapter, string, string) {
	if c.registry == nil || c.adapters == nil {
		return nil, "", "no roster"
	}
	snap, err := c.registry.Active()
	if err != nil || snap == nil {
		return nil, "", "no roster"
	}
	model, ok := snap.Model(c.cfg.ModelID)
	if !ok {
		return nil, "", "model not in the roster"
	}
	if !model.Enabled {
		return nil, "", "model is disabled or has no key"
	}
	tags := append([]string(nil), model.ComplianceTags...)
	if prov, ok := snap.Provider(model.ProviderID); ok {
		tags = append(tags, prov.ComplianceTags...)
	}
	local := false
	for _, t := range tags {
		if localTags[strings.ToLower(strings.TrimSpace(t))] {
			local = true
			break
		}
	}
	if !local {
		return nil, "", "model is not tagged local: a second opinion may only run on your own hardware"
	}
	adapter, ok := c.adapters()[model.ProviderID]
	if !ok || adapter == nil {
		return nil, "", "no adapter for the model's provider"
	}
	return adapter, model.ProviderModelID, ""
}

func (c *Classifier) warn(msg string, args ...any) {
	now := time.Now().Unix()
	if last := c.lastWarn.Load(); now-last < 60 {
		return
	}
	c.lastWarn.Store(now)
	c.logger.Warn(msg, args...)
}

// promptText joins the user's and system's words, bounded.
func promptText(messages []openai.Message) string {
	var b strings.Builder
	for _, m := range messages {
		if m.Role == "assistant" || m.Content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.Content)
		if b.Len() >= maxPromptChars {
			break
		}
	}
	s := b.String()
	if len(s) > maxPromptChars {
		s = s[:maxPromptChars]
	}
	return s
}

// parseLabels reads the labels out of whatever the model answered. Only words
// from the fixed vocabulary count; order follows the vocabulary's strength.
func parseLabels(answer string) []string {
	low := strings.ToLower(answer)
	var out []string
	for _, label := range []string{"secrets", "pii", "security_classified", "health", "financial", "legal", "source_code"} {
		if containsWord(low, label) {
			out = append(out, string(labels[label]))
		}
	}
	return out
}

// containsWord finds label as a whole token (quoted or bare), so that
// "security_classified" is not also "classified" and "pii" is not "piiza".
func containsWord(s, label string) bool {
	idx := 0
	for {
		i := strings.Index(s[idx:], label)
		if i < 0 {
			return false
		}
		start, end := idx+i, idx+i+len(label)
		before := start == 0 || !isWordByte(s[start-1])
		after := end == len(s) || !isWordByte(s[end])
		if before && after {
			return true
		}
		idx = end
	}
}

func isWordByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}
