package vault

import (
	"testing"
)

// ── PKM system detection tests ────────────────────────────────────────────────

func TestDetectPARA(t *testing.T) {
	state := &analysisState{
		folderSet:    map[string]bool{"projects": true, "areas": true, "resources": true, "archive": true},
		allFolderSet: map[string]bool{},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{},
		linkTargets:  map[string]int{},
	}
	s := detectPARA(state)
	if s.system != "PARA" {
		t.Errorf("expected PARA, got %q", s.system)
	}
	if s.score < 0.65 {
		t.Errorf("4 PARA folders should give score >= 0.65, got %.2f", s.score)
	}
}

func TestDetectPARA_Partial(t *testing.T) {
	state := &analysisState{
		folderSet:    map[string]bool{"projects": true, "archive": true},
		allFolderSet: map[string]bool{},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{},
		linkTargets:  map[string]int{},
	}
	s := detectPARA(state)
	if s.score < 0.30 || s.score > 0.40 {
		t.Errorf("2 PARA folders should give score ~0.35, got %.2f", s.score)
	}
}

func TestDetectPARA_None(t *testing.T) {
	state := &analysisState{
		folderSet:    map[string]bool{"notes": true, "daily": true},
		allFolderSet: map[string]bool{},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{},
		linkTargets:  map[string]int{},
	}
	s := detectPARA(state)
	if s.score != 0 {
		t.Errorf("no PARA folders should give 0 score, got %.2f", s.score)
	}
}

func TestDetectLYT(t *testing.T) {
	state := &analysisState{
		folderSet:    map[string]bool{"atlas": true, "calendar": true, "cards": true, "sources": true, "spaces": true},
		allFolderSet: map[string]bool{},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{"up": 15},
		linkTargets:  map[string]int{},
		mocFiles:     []string{"a/MOC.md", "b/MOC.md", "c/MOC.md", "d/MOC.md"},
		totalNotes:   100,
	}
	s := detectLYT(state)
	if s.system != "LYT/ACCESS" {
		t.Errorf("expected LYT/ACCESS, got %q", s.system)
	}
	if s.score < 0.80 {
		t.Errorf("full ACCESS folders + MOCs + up: key should give high score, got %.2f", s.score)
	}
}

func TestDetectACE(t *testing.T) {
	state := &analysisState{
		folderSet:    map[string]bool{"atlas": true, "calendar": true, "efforts": true},
		allFolderSet: map[string]bool{},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{},
		linkTargets:  map[string]int{},
	}
	s := detectACE(state)
	if s.system != "ACE" {
		t.Errorf("expected ACE, got %q", s.system)
	}
	if s.score < 0.85 {
		t.Errorf("all ACE folders should give score >= 0.85, got %.2f", s.score)
	}
}

func TestDetectACE_NoEfforts(t *testing.T) {
	state := &analysisState{
		folderSet:    map[string]bool{"atlas": true, "calendar": true},
		allFolderSet: map[string]bool{},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{},
		linkTargets:  map[string]int{},
	}
	s := detectACE(state)
	if s.score != 0 {
		t.Errorf("without Efforts/, ACE score should be 0, got %.2f", s.score)
	}
}

func TestDetectZettelkasten(t *testing.T) {
	notes := make([]*noteData, 10)
	for i := range notes {
		notes[i] = &noteData{
			BaseName: "202301011200",
			Parsed: ParsedNote{
				Frontmatter: map[string]interface{}{"type": "permanent"},
			},
			Tags:      []string{},
			Wikilinks: []string{"note-a", "note-b", "note-c", "note-d"},
		}
	}

	state := &analysisState{
		allNotes:       notes,
		folderSet:      map[string]bool{},
		allFolderSet:   map[string]bool{},
		tagFreq:        map[string]int{},
		fmKeyFreq:      map[string]int{},
		linkTargets:    map[string]int{},
		totalLinks:     40,
		totalNotes:     10,
		maxFolderDepth: 1,
	}
	s := detectZettelkasten(state)
	if s.system != "Zettelkasten" {
		t.Errorf("expected Zettelkasten, got %q", s.system)
	}
	if s.score < 0.90 {
		t.Errorf("strong Zettelkasten signals should give high score, got %.2f", s.score)
	}
}

func TestDetectJohnnyDecimal(t *testing.T) {
	notes := make([]*noteData, 5)
	for i := range notes {
		notes[i] = &noteData{BaseName: "11.01 Budget"}
	}

	state := &analysisState{
		allNotes:     notes,
		folderSet:    map[string]bool{},
		allFolderSet: map[string]bool{"10-19": true, "10-19/11 finance": true, "20-29": true, "20-29/21 admin": true},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{},
		linkTargets:  map[string]int{},
		totalNotes:   5,
	}
	s := detectJohnnyDecimal(state)
	if s.system != "Johnny.Decimal" {
		t.Errorf("expected Johnny.Decimal, got %q", s.system)
	}
	if s.score < 0.50 {
		t.Errorf("JD folders + file IDs should give decent score, got %.2f", s.score)
	}
}

func TestDetectGTD(t *testing.T) {
	notes := make([]*noteData, 20)
	for i := range notes {
		notes[i] = &noteData{
			BaseName: "task",
			Parsed:   ParsedNote{Frontmatter: map[string]interface{}{}},
		}
	}
	state := &analysisState{
		allNotes:      notes,
		folderSet:    map[string]bool{"next actions": true, "someday": true},
		allFolderSet: map[string]bool{"next actions": true, "someday": true},
		tagFreq:      map[string]int{"@home": 5, "@work": 3, "@errands": 2},
		fmKeyFreq:    map[string]int{},
		linkTargets:  map[string]int{},
		checkboxLines: 50,
		totalNotes:    20,
	}
	s := detectGTD(state)
	if s.system != "GTD" {
		t.Errorf("expected GTD, got %q", s.system)
	}
	if s.score < 0.70 {
		t.Errorf("GTD folders + @tags + checkboxes should give high score, got %.2f", s.score)
	}
}

func TestDetectBASB(t *testing.T) {
	state := &analysisState{
		folderSet:    map[string]bool{"projects": true, "areas": true, "resources": true},
		allFolderSet: map[string]bool{},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{"source-type": 8, "summary": 5},
		linkTargets:  map[string]int{},
	}
	s := detectBASB(state)
	if s.system != "BASB" {
		t.Errorf("expected BASB, got %q", s.system)
	}
	if s.score < 0.70 {
		t.Errorf("PARA folders + source-type + summary should give high score, got %.2f", s.score)
	}
}

func TestDetectEvergreen(t *testing.T) {
	notes := make([]*noteData, 20)
	for i := range notes {
		notes[i] = &noteData{
			BaseName:  "how-to-write-effective-assertions-in-notes",
			Wikilinks: []string{"a", "b", "c", "d", "e", "f"},
		}
	}

	state := &analysisState{
		allNotes:       notes,
		folderSet:      map[string]bool{},
		allFolderSet:   map[string]bool{},
		tagFreq:        map[string]int{},
		fmKeyFreq:      map[string]int{},
		linkTargets:    map[string]int{},
		totalLinks:     120,
		totalNotes:     20,
		maxFolderDepth: 1,
	}
	s := detectEvergreen(state)
	if s.system != "Evergreen" {
		t.Errorf("expected Evergreen, got %q", s.system)
	}
	if s.score < 0.80 {
		t.Errorf("assertion titles + flat + high links should give high score, got %.2f", s.score)
	}
}

// ── Hybrid and organic detection ──────────────────────────────────────────────

func TestDetectPKMSystem_Hybrid(t *testing.T) {
	// PARA + BASB signals together → hybrid
	notes := make([]*noteData, 10)
	for i := range notes {
		notes[i] = &noteData{
			BaseName: "note",
			Parsed:   ParsedNote{Frontmatter: map[string]interface{}{}},
		}
	}
	state := &analysisState{
		allNotes:     notes,
		folderSet:    map[string]bool{"projects": true, "areas": true, "resources": true, "archive": true},
		allFolderSet: map[string]bool{},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{"source-type": 5, "summary": 3},
		linkTargets:  map[string]int{},
		totalNotes:   10,
	}
	result := detectPKMSystem(state)
	if result == nil {
		t.Fatal("expected non-nil result for PARA+BASB signals")
	}
	if !result.IsHybrid {
		t.Error("expected hybrid detection for PARA + BASB")
	}
}

func TestDetectPKMSystem_Organic(t *testing.T) {
	notes := make([]*noteData, 5)
	for i := range notes {
		notes[i] = &noteData{
			BaseName: "note",
			Parsed:   ParsedNote{Frontmatter: map[string]interface{}{}},
		}
	}
	state := &analysisState{
		allNotes:     notes,
		folderSet:    map[string]bool{"random": true, "stuff": true},
		allFolderSet: map[string]bool{},
		tagFreq:      map[string]int{},
		fmKeyFreq:    map[string]int{},
		linkTargets:  map[string]int{},
		totalNotes:   5,
	}
	result := detectPKMSystem(state)
	if result != nil {
		t.Errorf("expected nil for organic vault, got %+v", result)
	}
}

// ── Analysis dimension tests ──────────────────────────────────────────────────

func TestAnalyzeFrontmatterSchema_Empty(t *testing.T) {
	state := &analysisState{fmKeyFreq: map[string]int{}}
	result := analyzeFrontmatterSchema(state)
	if result != nil {
		t.Error("expected nil for empty frontmatter")
	}
}

func TestAnalyzeFrontmatterSchema_WithKeys(t *testing.T) {
	state := &analysisState{
		fmKeyFreq: map[string]int{"tags": 10, "date": 8, "type": 3},
		allNotes: []*noteData{
			{Parsed: ParsedNote{Frontmatter: map[string]interface{}{"tags": []string{"a"}, "up": "parent", "related": "sibling"}}},
			{Parsed: ParsedNote{Frontmatter: map[string]interface{}{"tags": []string{"b"}, "up": "parent2", "related": "sibling2"}}},
		},
	}
	result := analyzeFrontmatterSchema(state)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.KeyFrequency) == 0 {
		t.Error("expected non-empty key frequency")
	}
	// Should be sorted by count descending
	for i := 1; i < len(result.KeyFrequency); i++ {
		if result.KeyFrequency[i].Count > result.KeyFrequency[i-1].Count {
			t.Errorf("key frequency not sorted at index %d", i)
		}
	}
}

func TestAnalyzeFrontmatterSchema_NotableCombos(t *testing.T) {
	state := &analysisState{
		fmKeyFreq: map[string]int{"up": 5, "related": 5},
		allNotes: []*noteData{
			{Parsed: ParsedNote{Frontmatter: map[string]interface{}{"up": "parent", "related": "sibling"}}},
			{Parsed: ParsedNote{Frontmatter: map[string]interface{}{"up": "parent2", "related": "sibling2"}}},
		},
	}
	result := analyzeFrontmatterSchema(state)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	found := false
	for _, c := range result.NotableCombos {
		if c.Signal == "LYT" {
			found = true
			if c.Count != 2 {
				t.Errorf("expected LYT combo count 2, got %d", c.Count)
			}
		}
	}
	if !found {
		t.Error("expected LYT notable combo for up+related keys")
	}
}

func TestAnalyzeLinkStructure_Empty(t *testing.T) {
	state := &analysisState{totalNotes: 0}
	result := analyzeLinkStructure(state)
	if result != nil {
		t.Error("expected nil for empty vault")
	}
}

func TestAnalyzeLinkStructure_Basic(t *testing.T) {
	notes := []*noteData{
		{RelPath: "a.md", BaseName: "a", Wikilinks: []string{"b", "c"}, Parsed: ParsedNote{Content: "Some [[b]] and [[c]] text"}},
		{RelPath: "b.md", BaseName: "b", Wikilinks: []string{"a"}, Parsed: ParsedNote{Content: "Link to [[a]]"}},
		{RelPath: "c.md", BaseName: "c", Wikilinks: nil, Parsed: ParsedNote{Content: "No links here"}},
		{RelPath: "d.md", BaseName: "d", Wikilinks: nil, Parsed: ParsedNote{Content: "Nobody links to me"}},
	}
	state := &analysisState{
		allNotes:   notes,
		totalLinks: 3,
		totalNotes: 4,
	}
	result := analyzeLinkStructure(state)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.TotalLinks != 3 {
		t.Errorf("TotalLinks = %d, want 3", result.TotalLinks)
	}
	if result.OrphanCount != 1 {
		t.Errorf("OrphanCount = %d, want 1 (d is orphaned)", result.OrphanCount)
	}
}

func TestAnalyzeTagTaxonomy_Empty(t *testing.T) {
	state := &analysisState{tagFreq: map[string]int{}}
	result := analyzeTagTaxonomy(state)
	if result != nil {
		t.Error("expected nil for empty tags")
	}
}

func TestAnalyzeTagTaxonomy_Nested(t *testing.T) {
	state := &analysisState{
		tagFreq: map[string]int{
			"status/active":    5,
			"status/archived":  2,
			"type/concept":     3,
			"@home":            1,
			"@work":            2,
			"simple":           4,
		},
	}
	result := analyzeTagTaxonomy(state)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result.MaxDepth != 2 {
		t.Errorf("MaxDepth = %d, want 2", result.MaxDepth)
	}
	if len(result.Prefixes) == 0 {
		t.Error("expected non-empty prefixes for nested tags")
	}
	if len(result.ContextTags) != 2 {
		t.Errorf("ContextTags count = %d, want 2", len(result.ContextTags))
	}
}

// ── Naming type classification ────────────────────────────────────────────────

func TestClassifyNamingType_ZettelkastenUID(t *testing.T) {
	names := []string{"202301011200", "202301021300", "202301031400"}
	result := classifyNamingType(names)
	if result != "zettelkasten-uid" {
		t.Errorf("expected zettelkasten-uid, got %q", result)
	}
}

func TestClassifyNamingType_JohnnyDecimal(t *testing.T) {
	names := []string{"11.01 Budget", "11.02 Taxes", "11.03 Insurance"}
	result := classifyNamingType(names)
	if result != "johnny-decimal" {
		t.Errorf("expected johnny-decimal, got %q", result)
	}
}

func TestClassifyNamingType_ADRPrefix(t *testing.T) {
	names := []string{"use-sqlite", "use-cobra", "adr-logging"}
	result := classifyNamingType(names)
	if result != "adr-prefix" {
		t.Errorf("expected adr-prefix, got %q", result)
	}
}

func TestClassifyNamingType_DateTopic(t *testing.T) {
	names := []string{"2023-01-01-meeting-notes", "2023-01-02-standup", "2023-01-03-retro"}
	result := classifyNamingType(names)
	if result != "date-topic" {
		t.Errorf("expected date-topic, got %q", result)
	}
}

func TestClassifyNamingType_DateOnly(t *testing.T) {
	names := []string{"2023-01-01", "2023-01-02", "2023-01-03"}
	result := classifyNamingType(names)
	if result != "date-only" {
		t.Errorf("expected date-only, got %q", result)
	}
}

func TestClassifyNamingType_KebabCase(t *testing.T) {
	names := []string{"my-note", "another", "third"}
	result := classifyNamingType(names)
	if result != "kebab-case" {
		t.Errorf("expected kebab-case, got %q", result)
	}
}

// ── Helper tests ──────────────────────────────────────────────────────────────

func TestIsAssertionTitle(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"how-to-write-good-notes-consistently", true},
		{"why-zettelkasten-works-for-learning", true},
		{"the-importance-of-spaced-repetition-for-memory", true},
		{"my-note", false},
		{"meeting", false},
	}
	for _, tc := range tests {
		got := isAssertionTitle(tc.name)
		if got != tc.want {
			t.Errorf("isAssertionTitle(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsPKMAligned(t *testing.T) {
	tests := []struct {
		typeName string
		folder   string
		pkm      *PKMSystemResult
		want     bool
	}{
		{"project", "Projects/acme/", &PKMSystemResult{Primary: "PARA"}, true},
		{"concept", "Knowledge/", &PKMSystemResult{Primary: "Zettelkasten"}, true},
		{"meeting", "Areas/meetings/", &PKMSystemResult{Primary: "PARA"}, true},
		{"decision", "decisions/", nil, false},
		{"concept", "Atlas/concepts/", &PKMSystemResult{Primary: "LYT/ACCESS"}, true},
	}
	for _, tc := range tests {
		got := isPKMAligned(tc.typeName, tc.folder, tc.pkm)
		if got != tc.want {
			t.Errorf("isPKMAligned(%q, %q, %v) = %v, want %v", tc.typeName, tc.folder, tc.pkm, got, tc.want)
		}
	}
}

func TestClampScore(t *testing.T) {
	if clampScore(0.5, 1.0) != 0.5 {
		t.Error("clampScore(0.5, 1.0) should return 0.5")
	}
	if clampScore(1.5, 1.0) != 1.0 {
		t.Error("clampScore(1.5, 1.0) should return 1.0")
	}
}
