package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TemplateVars holds substitution values for template scaffolding.
type TemplateVars struct {
	Title string // note title (filename without .md)
	Date  string // YYYY-MM-DD
}

// LoadTemplate reads a template file from templatesDir.
// templateName should not include .md extension.
func LoadTemplate(templatesDir, templateName string) (string, error) {
	templatePath := filepath.Join(templatesDir, templateName+".md")
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to load template %q: %w", templateName, err)
	}
	return string(data), nil
}

var (
	reTitleVar = regexp.MustCompile(`<%\s*tp\.file\.title\s*%>`)
	reDateVar  = regexp.MustCompile(`<%\s*tp\.date\.now\("[^"]*"\)\s*%>`)
)

// ScaffoldNote replaces Obsidian Templater placeholders with vars.
// Handles:
//   - <% tp.file.title %> → vars.Title
//   - <% tp.date.now("...") %> → vars.Date
func ScaffoldNote(templateContent string, vars TemplateVars) string {
	result := reTitleVar.ReplaceAllString(templateContent, vars.Title)
	result = reDateVar.ReplaceAllString(result, vars.Date)
	return result
}

// MergeContentIntoTemplate merges existing note content into template sections.
// For each ## Section in the template, looks for matching content in existingContent.
// Missing sections are marked with <!-- TODO -->.
func MergeContentIntoTemplate(templateContent, existingContent string) string {
	templateLines := strings.Split(templateContent, "\n")
	existingLines := strings.Split(existingContent, "\n")

	// Extract ## sections from template
	type section struct {
		heading string
		index   int
	}
	var sections []section
	for i, line := range templateLines {
		if strings.HasPrefix(line, "## ") {
			sections = append(sections, section{heading: line, index: i})
		}
	}

	if len(sections) == 0 {
		// No sections — just append existing content
		return templateContent + "\n\n" + existingContent
	}

	// Extract ## sections from existing content
	existingSections := map[string]string{}
	for i, line := range existingLines {
		if strings.HasPrefix(line, "## ") {
			heading := line
			// Find end of section (next ## or end)
			end := len(existingLines)
			for j := i + 1; j < len(existingLines); j++ {
				if strings.HasPrefix(existingLines[j], "## ") {
					end = j
					break
				}
			}
			sectionContent := strings.TrimSpace(strings.Join(existingLines[i+1:end], "\n"))
			existingSections[heading] = sectionContent
		}
	}

	// Build output
	var output []string
	lastSectionEnd := 0

	for i, sec := range sections {
		nextSectionStart := len(templateLines)
		if i+1 < len(sections) {
			nextSectionStart = sections[i+1].index
		}

		// Add lines before this section
		output = append(output, templateLines[lastSectionEnd:sec.index]...)

		// Add section heading
		output = append(output, sec.heading)

		// Add content or TODO placeholder
		if content, ok := existingSections[sec.heading]; ok && content != "" {
			output = append(output, content)
		} else {
			output = append(output, "<!-- TODO -->")
		}

		lastSectionEnd = nextSectionStart
	}

	// Add remaining template lines
	output = append(output, templateLines[lastSectionEnd:]...)

	return strings.TrimRight(strings.Join(output, "\n"), "\n")
}
