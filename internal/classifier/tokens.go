// Package classifier contains deterministic fast-path feature estimators.
package classifier

import (
	"reflect"
	"unicode/utf8"

	"github.com/magnusfroste/sluss/internal/openai"
)

const (
	// DefaultMaxOutputTokensEstimate is used when the client did not provide an
	// explicit output cap. Keep this conservative so routing does not under-plan.
	DefaultMaxOutputTokensEstimate = 4096

	OutputTokenSourceMaxCompletionTokens OutputTokenSource = "max_completion_tokens"
	OutputTokenSourceMaxTokens           OutputTokenSource = "max_tokens"
	OutputTokenSourceDefault             OutputTokenSource = "default"
)

type OutputTokenSource string

type TokenEstimate struct {
	CharCount               int
	PromptTokensEstimate    int
	MaxOutputTokensEstimate int
	MaxOutputTokensSource   OutputTokenSource
}

// EstimateChatRequestTokens estimates prompt and max output tokens for an
// OpenAI-compatible chat request without DB, network, LLM, or tokenizer calls.
func EstimateChatRequestTokens(req *openai.ChatRequest) TokenEstimate {
	var messages []openai.Message
	if req != nil {
		messages = req.Messages
	}
	return buildTokenEstimate(messages, req)
}

func estimateChatRequestTokens(req any) TokenEstimate {
	return buildTokenEstimate(messagesFromRequest(req), req)
}

func buildTokenEstimate(messages []openai.Message, req any) TokenEstimate {
	charCount := countPromptChars(messages)
	outputEstimate, source := maxOutputEstimateFromRequest(req)

	return TokenEstimate{
		CharCount:               charCount,
		PromptTokensEstimate:    ceilDiv(charCount, 4),
		MaxOutputTokensEstimate: outputEstimate,
		MaxOutputTokensSource:   source,
	}
}

func countPromptChars(messages []openai.Message) int {
	total := 0
	for _, message := range messages {
		if !isRelevantChatRole(message.Role) {
			continue
		}
		total += utf8.RuneCountInString(message.Content)
	}
	return total
}

func isRelevantChatRole(role string) bool {
	switch role {
	case "system", "developer", "user", "assistant", "tool":
		return true
	default:
		return false
	}
}

func maxOutputEstimateFromRequest(req any) (int, OutputTokenSource) {
	if value, ok := intPointerField(req, "MaxCompletionTokens"); ok {
		return value, OutputTokenSourceMaxCompletionTokens
	}
	if value, ok := intPointerField(req, "MaxTokens"); ok {
		return value, OutputTokenSourceMaxTokens
	}
	return DefaultMaxOutputTokensEstimate, OutputTokenSourceDefault
}

func messagesFromRequest(req any) []openai.Message {
	field := structField(req, "Messages")
	if !field.IsValid() || field.Kind() != reflect.Slice || field.IsNil() {
		return nil
	}

	messages, ok := field.Interface().([]openai.Message)
	if !ok {
		return nil
	}
	return messages
}

func intPointerField(req any, name string) (int, bool) {
	field := structField(req, name)
	if !field.IsValid() || field.Kind() != reflect.Pointer || field.IsNil() {
		return 0, false
	}
	if field.Type() != reflect.TypeOf((*int)(nil)) {
		return 0, false
	}
	return int(field.Elem().Int()), true
}

func structField(req any, name string) reflect.Value {
	if req == nil {
		return reflect.Value{}
	}

	value := reflect.ValueOf(req)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	field := value.FieldByName(name)
	if !field.IsValid() || !field.CanInterface() {
		return reflect.Value{}
	}
	return field
}

func ceilDiv(value, divisor int) int {
	if value == 0 {
		return 0
	}
	return ((value - 1) / divisor) + 1
}
