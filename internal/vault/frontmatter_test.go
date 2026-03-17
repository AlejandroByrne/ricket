package vault_test

import (
	"reflect"
	"testing"

	"github.com/AlejandroByrne/ricket/internal/vault"
)

func TestParseNote(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantContent     string
		wantFMKeys      []string
		wantFMTagsCount int
	}{
		{
			name:        "no_frontmatter",
			input:       "# Hello\n\nSome content.",
			wantContent: "# Hello\n\nSome content.",
		},
		{
			name: "with_frontmatter",
			input: `---
title: My Note
tags: [foo, bar]
---
# Body

Content here.`,
			wantContent:     "# Body\n\nContent here.",
			wantFMKeys:      []string{"title", "tags"},
			wantFMTagsCount: 2,
		},
		{
			name:        "empty_frontmatter",
			input:       "---\n---\nContent only.",
			wantContent: "Content only.",
		},
		{
			name:        "unclosed_frontmatter",
			input:       "---\ntitle: oops\nno closing",
			wantContent: "---\ntitle: oops\nno closing",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			note := vault.ParseNote(tc.input)
			if note.Content != tc.wantContent {
				t.Errorf("Content = %q, want %q", note.Content, tc.wantContent)
			}
			for _, k := range tc.wantFMKeys {
				if _, ok := note.Frontmatter[k]; !ok {
					t.Errorf("missing frontmatter key %q", k)
				}
			}
		})
	}
}

func TestSerializeNote(t *testing.T) {
	t.Run("no_frontmatter", func(t *testing.T) {
		out := vault.SerializeNote(map[string]interface{}{}, "Hello")
		if out != "Hello" {
			t.Errorf("got %q, want %q", out, "Hello")
		}
	})

	t.Run("with_frontmatter_roundtrip", func(t *testing.T) {
		fm := map[string]interface{}{"title": "Test", "tags": []interface{}{"a", "b"}}
		content := "# Body\n\nSome text."
		serialized := vault.SerializeNote(fm, content)

		// Must start with ---
		if len(serialized) < 3 || serialized[:3] != "---" {
			t.Errorf("serialized should start with ---, got: %q", serialized[:10])
		}

		// Round-trip parse
		reparsed := vault.ParseNote(serialized)
		if reparsed.Content != content {
			t.Errorf("round-trip content = %q, want %q", reparsed.Content, content)
		}
		if reparsed.Frontmatter["title"] != "Test" {
			t.Errorf("round-trip title = %v", reparsed.Frontmatter["title"])
		}
	})
}

func TestMergeTags(t *testing.T) {
	tests := []struct {
		existing []string
		toAdd    []string
		want     []string
	}{
		{nil, nil, []string{}},
		{[]string{"a", "b"}, []string{"c"}, []string{"a", "b", "c"}},
		{[]string{"a", "b"}, []string{"a", "c"}, []string{"a", "b", "c"}},
		{[]string{"x"}, []string{"x"}, []string{"x"}},
		{nil, []string{"a"}, []string{"a"}},
	}

	for _, tc := range tests {
		got := vault.MergeTags(tc.existing, tc.toAdd)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("MergeTags(%v, %v) = %v, want %v", tc.existing, tc.toAdd, got, tc.want)
		}
	}
}

func TestAddFrontmatterTags(t *testing.T) {
	t.Run("adds_to_empty", func(t *testing.T) {
		note := vault.ParseNote("Hello")
		updated := vault.AddFrontmatterTags(note, []string{"foo", "bar"})
		tags := vault.GetTags(updated)
		if !reflect.DeepEqual(tags, []string{"foo", "bar"}) {
			t.Errorf("got tags %v, want [foo bar]", tags)
		}
	})

	t.Run("merges_without_duplicates", func(t *testing.T) {
		raw := "---\ntags: [existing]\n---\nContent"
		note := vault.ParseNote(raw)
		updated := vault.AddFrontmatterTags(note, []string{"existing", "new"})
		tags := vault.GetTags(updated)
		if len(tags) != 2 {
			t.Errorf("expected 2 tags, got %v", tags)
		}
	})

	t.Run("preserves_other_frontmatter", func(t *testing.T) {
		raw := "---\ntitle: My Note\ntags: []\n---\nContent"
		note := vault.ParseNote(raw)
		updated := vault.AddFrontmatterTags(note, []string{"new"})
		if updated.Frontmatter["title"] != "My Note" {
			t.Errorf("title was lost: %v", updated.Frontmatter["title"])
		}
	})
}

func TestExtractWikilinks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"none", "No links here.", nil},
		{"simple", "See [[Note Name]] for more.", []string{"Note Name"}},
		{"pipe", "See [[Note Name|display text]].", []string{"Note Name"}},
		{"multiple", "[[A]] and [[B|bee]] and [[C]]", []string{"A", "B", "C"}},
		{"nested_brackets", "text [[link]] end", []string{"link"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := vault.ExtractWikilinks(tc.content)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ExtractWikilinks(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestGetTags(t *testing.T) {
	t.Run("no_tags", func(t *testing.T) {
		note := vault.ParseNote("Content")
		if tags := vault.GetTags(note); tags != nil {
			t.Errorf("expected nil, got %v", tags)
		}
	})

	t.Run("string_tags", func(t *testing.T) {
		note := vault.ParseNote("---\ntags: [a, b, c]\n---\nContent")
		tags := vault.GetTags(note)
		if len(tags) != 3 {
			t.Errorf("expected 3 tags, got %v", tags)
		}
	})
}
