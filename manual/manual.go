// Package manual embeds the Sluss manual (ISSUE-133): plain Markdown, one
// file per goal, written for an administrator without specialist training.
// The same files are read in the repository, served as HTML on an instance
// at /manual, and served raw (for agents) at /manual/<page>.md. {{BASE}} is
// replaced with the instance's public address when served.
package manual

import "embed"

//go:embed *.md
var Files embed.FS

// Page is one entry in the manual's menu, in reading order.
type Page struct {
	Slug, Title string
}

// Pages is the menu. index is the landing page.
var Pages = []Page{
	{"index", "Start here"},
	{"quick-start", "Models and the first rule"},
	{"choosing-models", "How Sluss picks a model"},
	{"stop-chatgpt", "Stop ChatGPT, offer the company chat"},
	{"developer-tools", "Developer tools through Sluss"},
	{"prove-it", "Show that it works"},
	{"glossary", "Words used"},
	{"troubleshooting", "When something does not work"},
}
