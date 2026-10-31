package manual

import (
	"os"
	"strings"
	"testing"
)

// The repository's llms.txt is the public index for agents: it must list
// every manual page, so a new page is never invisible to them.
func TestLLMSTxtListsEveryManualPage(t *testing.T) {
	b, err := os.ReadFile("../llms.txt")
	if err != nil {
		t.Fatalf("read llms.txt: %v", err)
	}
	for _, p := range Pages {
		if !strings.Contains(string(b), "manual/"+p.Slug+".md") {
			t.Errorf("llms.txt does not list manual/%s.md (%s)", p.Slug, p.Title)
		}
	}
}
