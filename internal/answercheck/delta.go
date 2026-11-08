package answercheck

import "encoding/json"

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// deltaContent returns the text of an OpenAI-style stream chunk.
func deltaContent(data []byte) string {
	var c streamChunk
	if err := json.Unmarshal(data, &c); err != nil {
		return ""
	}
	out := ""
	for _, ch := range c.Choices {
		out += ch.Delta.Content + ch.Message.Content
	}
	return out
}
