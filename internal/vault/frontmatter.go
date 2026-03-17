// Package vault provides core Obsidian vault operations.
package vault

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ParsedNote holds a note split into its frontmatter, content, and raw text.
type ParsedNote struct {
	Frontmatter map[string]interface{} // parsed YAML metadata
	Content     string                 // everything after the frontmatter
	Raw         string                 // original full text
}

// ParseNote splits a markdown note into frontmatter and content.
// Frontmatter is YAML between --- delimiters at the start.
func ParseNote(raw string) ParsedNote {
	lines := strings.Split(raw, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ParsedNote{Frontmatter: map[string]interface{}{}, Content: raw, Raw: raw}
	}

	// Find closing ---
	closeIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIndex = i
			break
		}
	}

	if closeIndex == -1 {
		return ParsedNote{Frontmatter: map[string]interface{}{}, Content: raw, Raw: raw}
	}

	// Parse YAML frontmatter
	fmText := strings.Join(lines[1:closeIndex], "\n")
	frontmatter := map[string]interface{}{}
	if strings.TrimSpace(fmText) != "" {
		if err := yaml.Unmarshal([]byte(fmText), &frontmatter); err != nil {
			frontmatter = map[string]interface{}{}
		}
	}

	// Content is everything after closing ---
	content := ""
	if closeIndex+1 < len(lines) {
		content = strings.TrimLeft(strings.Join(lines[closeIndex+1:], "\n"), "\n")
	}

	return ParsedNote{Frontmatter: frontmatter, Content: content, Raw: raw}
}

// SerializeNote reconstructs markdown from frontmatter and content.
func SerializeNote(frontmatter map[string]interface{}, content string) string {
	if len(frontmatter) == 0 {
		return content
	}

	data, err := yaml.Marshal(frontmatter)
	if err != nil {
		return content
	}

	return "---\n" + string(data) + "---\n" + content
}

// MergeTags deduplicates and merges two tag slices, preserving order.
func MergeTags(existing, toAdd []string) []string {
	seen := map[string]bool{}
	result := []string{}

	for _, t := range existing {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	for _, t := range toAdd {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

// AddFrontmatterTags adds tags to a note's frontmatter, merging without duplicates.
func AddFrontmatterTags(note ParsedNote, tags []string) ParsedNote {
	current := note.Frontmatter["tags"]
	var currentSlice []string

	switch v := current.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				currentSlice = append(currentSlice, s)
			}
		}
	case []string:
		currentSlice = v
	case string:
		if v != "" {
			currentSlice = []string{v}
		}
	}

	merged := MergeTags(currentSlice, tags)

	newFM := make(map[string]interface{}, len(note.Frontmatter))
	for k, v := range note.Frontmatter {
		newFM[k] = v
	}
	newFM["tags"] = merged

	return ParsedNote{
		Frontmatter: newFM,
		Content:     note.Content,
		Raw:         note.Raw,
	}
}

// ExtractWikilinks returns all [[link]] targets from text.
func ExtractWikilinks(content string) []string {
	var links []string
	remaining := content
	for {
		start := strings.Index(remaining, "[[")
		if start == -1 {
			break
		}
		end := strings.Index(remaining[start:], "]]")
		if end == -1 {
			break
		}
		inner := remaining[start+2 : start+end]
		// Handle [[link|display]] — take only the link part
		if pipe := strings.Index(inner, "|"); pipe != -1 {
			inner = inner[:pipe]
		}
		links = append(links, strings.TrimSpace(inner))
		remaining = remaining[start+end+2:]
	}
	return links
}

// GetTags returns the tags from a parsed note's frontmatter as a []string.
func GetTags(note ParsedNote) []string {
	current := note.Frontmatter["tags"]
	switch v := current.(type) {
	case []interface{}:
		tags := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				tags = append(tags, s)
			}
		}
		return tags
	case []string:
		return v
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}
