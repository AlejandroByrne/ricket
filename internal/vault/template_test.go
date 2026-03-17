package vault_test

import (
	"strings"
	"testing"

	"github.com/AlejandroByrne/ricket/internal/vault"
)

func TestScaffoldNote(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     vault.TemplateVars
		want     string
	}{
		{
			name:     "title_substitution",
			template: "# <% tp.file.title %>",
			vars:     vault.TemplateVars{Title: "My Note", Date: "2024-01-01"},
			want:     "# My Note",
		},
		{
			name:     "date_substitution",
			template: `Created: <% tp.date.now("YYYY-MM-DD") %>`,
			vars:     vault.TemplateVars{Title: "x", Date: "2024-06-15"},
			want:     "Created: 2024-06-15",
		},
		{
			name: "both_substitutions",
			template: `---
title: <% tp.file.title %>
date: <% tp.date.now("YYYY-MM-DD") %>
---
Content`,
			vars: vault.TemplateVars{Title: "Test Note", Date: "2024-03-17"},
			want: "---\ntitle: Test Note\ndate: 2024-03-17\n---\nContent",
		},
		{
			name:     "no_vars",
			template: "Just plain text",
			vars:     vault.TemplateVars{Title: "x", Date: "2024-01-01"},
			want:     "Just plain text",
		},
		{
			name:     "spaces_in_placeholder",
			template: "<%  tp.file.title  %>",
			vars:     vault.TemplateVars{Title: "Spaced", Date: "2024-01-01"},
			want:     "Spaced",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := vault.ScaffoldNote(tc.template, tc.vars)
			if got != tc.want {
				t.Errorf("ScaffoldNote:\n  got  %q\n  want %q", got, tc.want)
			}
		})
	}
}

func TestMergeContentIntoTemplate(t *testing.T) {
	t.Run("no_sections_appends", func(t *testing.T) {
		tmpl := "# Template\nNo sections here."
		existing := "Some existing content."
		got := vault.MergeContentIntoTemplate(tmpl, existing)
		if !strings.Contains(got, "No sections here.") {
			t.Errorf("template content missing: %q", got)
		}
		if !strings.Contains(got, "Some existing content.") {
			t.Errorf("existing content missing: %q", got)
		}
	})

	t.Run("matching_section_filled", func(t *testing.T) {
		tmpl := "# Title\n\n## Summary\n\n## Notes\n"
		existing := "## Summary\nThis is the summary."
		got := vault.MergeContentIntoTemplate(tmpl, existing)
		if !strings.Contains(got, "This is the summary.") {
			t.Errorf("summary content missing: %q", got)
		}
	})

	t.Run("missing_section_gets_todo", func(t *testing.T) {
		tmpl := "# Title\n\n## Summary\n\n## Notes\n"
		existing := "## Summary\nSome summary."
		got := vault.MergeContentIntoTemplate(tmpl, existing)
		if !strings.Contains(got, "<!-- TODO -->") {
			t.Errorf("expected <!-- TODO --> for missing ## Notes: %q", got)
		}
	})

	t.Run("all_sections_present", func(t *testing.T) {
		tmpl := "## A\n\n## B\n"
		existing := "## A\nContent A.\n\n## B\nContent B."
		got := vault.MergeContentIntoTemplate(tmpl, existing)
		if strings.Contains(got, "<!-- TODO -->") {
			t.Errorf("unexpected TODO when all sections present: %q", got)
		}
		if !strings.Contains(got, "Content A.") || !strings.Contains(got, "Content B.") {
			t.Errorf("section content missing: %q", got)
		}
	})
}
