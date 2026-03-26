package tui

import (
	"fmt"
	"regexp"
	"strings"
)

// LaunchItem holds a parsed prompt with auto-generated agent/branch names.
type LaunchItem struct {
	Prompt    string
	AgentName string
	Branch    string
}

var (
	// Matches numbered list markers: "1.", "2)", "3:", "1 -", etc.
	numberedRe = regexp.MustCompile(`^\s*\d+[\.\)\:\-]\s*`)
	// Matches bullet markers: "- ", "* ", "+ "
	bulletRe = regexp.MustCompile(`^\s*[\-\*\+]\s+`)
	// Matches non-alphanumeric characters (for slugifying)
	nonAlphaRe = regexp.MustCompile(`[^a-z0-9]+`)
	// Matches multiple consecutive hyphens
	multiHyphenRe = regexp.MustCompile(`-{2,}`)
)

// stopWords are common English words to strip when generating slugs.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "for": true,
	"in": true, "on": true, "with": true, "and": true, "or": true,
	"is": true, "it": true, "of": true, "that": true, "this": true,
	"be": true, "as": true, "at": true, "by": true, "from": true,
}

// ParsePrompts splits raw user input into individual LaunchItems.
// It detects numbered lists, bullet lists, and plain newline-separated lines.
func ParsePrompts(input string) []LaunchItem {
	lines := strings.Split(input, "\n")
	var rawPrompts []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Strip list markers
		cleaned := numberedRe.ReplaceAllString(trimmed, "")
		cleaned = bulletRe.ReplaceAllString(cleaned, "")
		cleaned = strings.TrimSpace(cleaned)

		if cleaned != "" {
			rawPrompts = append(rawPrompts, cleaned)
		}
	}

	// Generate names, deduplicating as we go
	var existing []string
	items := make([]LaunchItem, 0, len(rawPrompts))
	for _, prompt := range rawPrompts {
		name, branch := GenerateNames(prompt, existing)
		existing = append(existing, name)
		items = append(items, LaunchItem{
			Prompt:    prompt,
			AgentName: name,
			Branch:    branch,
		})
	}

	return items
}

// Slugify converts a prompt string into a short, URL-safe slug.
// It lowercases, removes stop words, takes the first 5 meaningful words,
// and truncates to 30 characters.
func Slugify(prompt string) string {
	lower := strings.ToLower(prompt)

	// Replace non-alpha with spaces for word splitting
	spaced := nonAlphaRe.ReplaceAllString(lower, " ")
	words := strings.Fields(spaced)

	// Remove stop words
	var meaningful []string
	for _, w := range words {
		if !stopWords[w] && len(w) > 0 {
			meaningful = append(meaningful, w)
		}
	}

	// Take first 5 words
	if len(meaningful) > 5 {
		meaningful = meaningful[:5]
	}

	slug := strings.Join(meaningful, "-")

	// Collapse any remaining multi-hyphens
	slug = multiHyphenRe.ReplaceAllString(slug, "-")

	// Truncate to 30 chars
	if len(slug) > 30 {
		slug = slug[:30]
	}

	// Trim trailing hyphens
	slug = strings.TrimRight(slug, "-")

	return slug
}

// GenerateNames produces an agent name and branch name from a prompt.
// It deduplicates against the existing list by appending -2, -3, etc.
func GenerateNames(prompt string, existing []string) (agentName, branch string) {
	base := Slugify(prompt)
	if base == "" {
		base = "agent"
	}

	agentName = base
	suffix := 2
	for contains(existing, agentName) {
		agentName = fmt.Sprintf("%s-%d", base, suffix)
		suffix++
	}

	branch = "fleet/" + agentName
	return agentName, branch
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
