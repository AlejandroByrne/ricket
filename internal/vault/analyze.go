package vault

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// FolderEntry describes a directory that contains markdown notes directly.
type FolderEntry struct {
	Path        string   `json:"path"`        // relative to vault root, trailing slash
	NoteCount   int      `json:"noteCount"`
	SampleNames []string `json:"sampleNames,omitempty"` // up to 5 filenames
}

// TagCount tracks how often a tag appears across the vault.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// NamingPattern captures the filename convention detected in a folder.
type NamingPattern struct {
	Folder   string   `json:"folder"`          // relative, trailing slash
	Pattern  string   `json:"pattern"`         // e.g. "YYYY-MM-DD-{topic}.md"
	Type     string   `json:"type,omitempty"`  // e.g. "zettelkasten-uid", "date-topic"
	Examples []string `json:"examples,omitempty"`
}

// TemplateEntry describes a template file found in the templates directory.
type TemplateEntry struct {
	Name     string   `json:"name"`
	Sections []string `json:"sections"` // ## heading names
}

// InferredCategory is ricket's best-guess category derived from vault structure.
type InferredCategory struct {
	Name       string   `json:"name"`
	Folder     string   `json:"folder"`
	Template   string   `json:"template,omitempty"`
	Naming     string   `json:"naming,omitempty"`
	Tags       []string `json:"tags"`
	MOC        string   `json:"moc,omitempty"`
	Signals    []string `json:"signals"`
	Confidence float64  `json:"confidence"`
	Reasoning  string   `json:"reasoning"`
}

// VaultAnalysis is the complete result of analyzing a vault's structure.
// Produced by AnalyzeVaultRoot — does not require ricket.yaml.
type VaultAnalysis struct {
	VaultRoot             string             `json:"vaultRoot"`
	ObsidianVaultDetected bool               `json:"obsidianVaultDetected"`
	HasExistingConfig     bool               `json:"hasExistingConfig"`
	IsNewVault            bool               `json:"isNewVault"`
	TotalNoteCount        int                `json:"totalNoteCount"`
	Folders               []FolderEntry      `json:"folders"`
	TagFrequency          []TagCount         `json:"tagFrequency"`
	NamingPatterns        []NamingPattern    `json:"namingPatterns"`
	Templates             []TemplateEntry    `json:"templates"`
	InferredCategories    []InferredCategory `json:"inferredCategories"`
	MOCFiles              []string           `json:"mocFiles"`
	DetectedInbox         string             `json:"detectedInbox"`
	DetectedArchive       string             `json:"detectedArchive"`
	DetectedTemplatesDir  string             `json:"detectedTemplatesDir"`
	PKMSystem             *PKMSystemResult   `json:"pkmSystem,omitempty"`
	FrontmatterSchema     *FrontmatterSchema `json:"frontmatterSchema,omitempty"`
	LinkAnalysis          *LinkAnalysis      `json:"linkAnalysis,omitempty"`
	TagTaxonomy           *TagTaxonomy       `json:"tagTaxonomy,omitempty"`
}

// AnalyzeVaultRoot scans root and returns a VaultAnalysis.
// Works without ricket.yaml — safe to call in migration mode.
func AnalyzeVaultRoot(root string) (*VaultAnalysis, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("invalid vault root: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("vault root does not exist: %w", err)
	}

	a := &VaultAnalysis{VaultRoot: abs}

	// .obsidian/ folder is the definitive Obsidian vault signal
	if _, err := os.Stat(filepath.Join(abs, ".obsidian")); err == nil {
		a.ObsidianVaultDetected = true
	}
	if _, err := os.Stat(filepath.Join(abs, "ricket.yaml")); err == nil {
		a.HasExistingConfig = true
	}

	// Detect well-known special folders by name
	a.DetectedInbox = detectSpecialFolder(abs, []string{"Inbox", "inbox", "_inbox"})
	a.DetectedArchive = detectSpecialFolder(abs, []string{"Archive", "archive", "_archive"})
	a.DetectedTemplatesDir = detectSpecialFolder(abs, []string{"_templates", "Templates", "templates", "_Templates"})

	// Single-pass walk: read and parse every note once
	state := buildAnalysisState(abs, a.DetectedTemplatesDir)
	a.TotalNoteCount = state.totalNotes
	a.MOCFiles = state.mocFiles

	a.Folders = buildFolderEntries(state)
	a.TagFrequency = tagFrequencyFromState(state)
	a.NamingPatterns = buildNamingPatterns(state)

	if a.DetectedTemplatesDir != "" {
		a.Templates = loadTemplateEntries(filepath.Join(abs, filepath.FromSlash(a.DetectedTemplatesDir)))
	}

	// New analysis dimensions
	a.FrontmatterSchema = analyzeFrontmatterSchema(state)
	a.LinkAnalysis = analyzeLinkStructure(state)
	a.TagTaxonomy = analyzeTagTaxonomy(state)

	// PKM system detection
	a.PKMSystem = detectPKMSystem(state)

	// Category inference with PKM context boost
	a.InferredCategories = inferCategories(state, a, a.PKMSystem)
	a.IsNewVault = a.TotalNoteCount == 0

	return a, nil
}

// ── Filesystem walk ───────────────────────────────────────────────────────────

// buildAnalysisState performs a single-pass walk of the vault, reading and
// parsing each markdown note once. All aggregate metrics (tags, frontmatter
// keys, link targets, checkboxes) are computed during the walk.
func buildAnalysisState(root, templatesDir string) *analysisState {
	state := &analysisState{
		root:          root,
		notesByFolder: make(map[string][]*noteData),
		folderSet:     make(map[string]bool),
		allFolderSet:  make(map[string]bool),
		tagFreq:       make(map[string]int),
		fmKeyFreq:     make(map[string]int),
		linkTargets:   make(map[string]int),
	}

	var templatesAbs string
	if templatesDir != "" {
		templatesAbs = filepath.Clean(filepath.Join(root, filepath.FromSlash(templatesDir)))
	}

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if templatesAbs != "" && filepath.Clean(path) == templatesAbs {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil {
				rel = filepath.ToSlash(rel)
				if rel != "." {
					state.allFolderSet[strings.ToLower(rel)] = true
					parts := strings.Split(rel, "/")
					if len(parts) == 1 {
						state.folderSet[strings.ToLower(parts[0])] = true
					}
					if len(parts) > state.maxFolderDepth {
						state.maxFolderDepth = len(parts)
					}
				}
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		folder := filepath.ToSlash(filepath.Dir(rel))
		if folder == "." {
			folder = ""
		} else {
			folder += "/"
		}

		baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		parsed := ParseNote(string(data))
		tags := GetTags(parsed)
		links := ExtractWikilinks(parsed.Content)

		nd := &noteData{
			RelPath:   rel,
			AbsPath:   path,
			Parsed:    parsed,
			Tags:      tags,
			Wikilinks: links,
			Folder:    folder,
			BaseName:  baseName,
		}

		dir := filepath.Dir(path)
		state.notesByFolder[dir] = append(state.notesByFolder[dir], nd)
		state.allNotes = append(state.allNotes, nd)

		for _, tag := range tags {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag != "" {
				state.tagFreq[tag]++
			}
		}
		for key := range parsed.Frontmatter {
			state.fmKeyFreq[strings.ToLower(key)]++
		}
		for _, link := range links {
			state.linkTargets[strings.ToLower(link)]++
			state.totalLinks++
		}
		for _, line := range strings.Split(parsed.Content, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") {
				state.checkboxLines++
			}
		}

		return nil
	})

	state.totalNotes = len(state.allNotes)

	mocNames := map[string]bool{"moc": true, "index": true, "home": true}
	for _, nd := range state.allNotes {
		base := strings.ToLower(nd.BaseName)
		if mocNames[base] {
			state.mocFiles = append(state.mocFiles, nd.RelPath)
		}
	}
	sort.Strings(state.mocFiles)

	return state
}

// detectSpecialFolder returns the first candidate folder name (with trailing /)
// that exists as a directory under root, or "" if none match.
func detectSpecialFolder(root string, candidates []string) string {
	for _, name := range candidates {
		p := filepath.Join(root, name)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return name + "/"
		}
	}
	return ""
}

// ── Aggregate metrics ─────────────────────────────────────────────────────────

func buildFolderEntries(state *analysisState) []FolderEntry {
	var entries []FolderEntry
	for dir, notes := range state.notesByFolder {
		rel, err := filepath.Rel(state.root, dir)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			continue // skip vault-root level
		}
		samples := make([]string, 0, 5)
		for i, nd := range notes {
			if i >= 5 {
				break
			}
			samples = append(samples, filepath.Base(nd.AbsPath))
		}
		entries = append(entries, FolderEntry{
			Path:        rel + "/",
			NoteCount:   len(notes),
			SampleNames: samples,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

// tagFrequencyFromState converts the pre-computed tag frequency map to sorted TagCount slice.
func tagFrequencyFromState(state *analysisState) []TagCount {
	result := make([]TagCount, 0, len(state.tagFreq))
	for tag, count := range state.tagFreq {
		result = append(result, TagCount{Tag: tag, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Tag < result[j].Tag
	})
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// ── Naming patterns ───────────────────────────────────────────────────────────

var datePatternRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

func buildNamingPatterns(state *analysisState) []NamingPattern {
	var patterns []NamingPattern
	for dir, notes := range state.notesByFolder {
		if len(notes) < 2 {
			continue
		}
		rel, err := filepath.Rel(state.root, dir)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			continue
		}
		names := make([]string, len(notes))
		for i, nd := range notes {
			names[i] = nd.BaseName
		}
		pattern, examples := classifyNamingPattern(names)
		namingType := classifyNamingType(names)
		patterns = append(patterns, NamingPattern{
			Folder:   rel + "/",
			Pattern:  pattern,
			Type:     namingType,
			Examples: examples,
		})
	}
	sort.Slice(patterns, func(i, j int) bool { return patterns[i].Folder < patterns[j].Folder })
	return patterns
}

func classifyNamingPattern(names []string) (string, []string) {
	dateCount := 0
	useCount := 0
	total := len(names)

	for _, name := range names {
		if datePatternRE.MatchString(name) {
			dateCount++
		}
		if strings.HasPrefix(name, "use-") {
			useCount++
		}
	}

	examples := names
	if len(examples) > 3 {
		examples = examples[:3]
	}

	if dateCount*2 >= total {
		// Check whether filenames are date-only (journals) or date + topic (meetings)
		dateOnly := true
		for _, name := range names {
			if len(name) > 10 {
				dateOnly = false
				break
			}
		}
		if dateOnly {
			return "YYYY-MM-DD.md", examples
		}
		return "YYYY-MM-DD-{topic}.md", examples
	}
	if useCount*2 >= total {
		return "use-{topic}.md", examples
	}
	return "{topic}.md", examples
}

// classifyNamingType returns a type classification for the naming pattern.
func classifyNamingType(names []string) string {
	total := len(names)
	if total == 0 {
		return "kebab-case"
	}

	zettelCount := 0
	jdCount := 0
	adrCount := 0
	dateCount := 0
	sentenceCount := 0

	for _, name := range names {
		switch {
		case zettelUIDRE.MatchString(name):
			zettelCount++
		case jdFileRE.MatchString(name):
			jdCount++
		case strings.HasPrefix(name, "use-") || strings.HasPrefix(name, "adr-"):
			adrCount++
		case datePatternRE.MatchString(name):
			dateCount++
		default:
			words := strings.FieldsFunc(name, func(r rune) bool {
				return r == '-' || r == '_' || r == ' '
			})
			if len(words) >= 5 {
				sentenceCount++
			}
		}
	}

	if zettelCount*2 >= total {
		return "zettelkasten-uid"
	}
	if jdCount*2 >= total {
		return "johnny-decimal"
	}
	if adrCount*2 >= total {
		return "adr-prefix"
	}
	if dateCount*2 >= total {
		dateOnly := true
		for _, name := range names {
			if datePatternRE.MatchString(name) && len(name) > 10 {
				dateOnly = false
				break
			}
		}
		if dateOnly {
			return "date-only"
		}
		return "date-topic"
	}
	if sentenceCount*2 >= total {
		return "sentence-title"
	}
	return "kebab-case"
}

// ── Templates ─────────────────────────────────────────────────────────────────

func loadTemplateEntries(dir string) []TemplateEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var templates []TemplateEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var sections []string
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "## ") {
				sections = append(sections, strings.TrimPrefix(line, "## "))
			}
		}
		templates = append(templates, TemplateEntry{Name: name, Sections: sections})
	}
	return templates
}

// ── Category inference ────────────────────────────────────────────────────────

type categoryHint struct {
	keywords []string
	typeName string
	signals  []string
	template string
}

// categoryHints maps known folder/tag keywords to category types.
// Order matters: more specific hints should come first.
var categoryHints = []categoryHint{
	{
		keywords: []string{"decision", "decisions", "standard", "standards", "convention", "conventions", "adr"},
		typeName: "decision",
		signals:  []string{"decision", "standard", "convention", "rule", "architecture", "use", "not", "instead"},
		template: "decision",
	},
	{
		keywords: []string{"concept", "concepts", "explanation", "explanations", "knowledge"},
		typeName: "concept",
		signals:  []string{"concept", "explain", "understand", "definition", "how", "what", "why"},
		template: "concept",
	},
	{
		keywords: []string{"meeting", "meetings", "standup", "sync", "retro", "retrospective"},
		typeName: "meeting",
		signals:  []string{"meeting", "standup", "sync", "planning", "retro", "review", "agenda"},
		template: "meeting",
	},
	{
		keywords: []string{"project", "projects", "initiative", "initiatives", "epic"},
		typeName: "project",
		signals:  []string{"project", "task", "feature", "initiative", "epic", "ticket", "milestone"},
		template: "project",
	},
	{
		keywords: []string{"people", "person", "team", "stakeholder", "contact", "directory"},
		typeName: "people",
		signals:  []string{"person", "people", "stakeholder", "owner", "contact", "manager", "teammate"},
		template: "person",
	},
	{
		keywords: []string{"learning", "development", "skill", "training", "education", "study"},
		typeName: "learning",
		signals:  []string{"learning", "skill", "training", "course", "book", "reading", "study"},
		template: "learning",
	},
	{
		keywords: []string{"resource", "resources", "reference", "link", "doc", "docs"},
		typeName: "resource",
		signals:  []string{"reference", "resource", "link", "doc", "documentation"},
		template: "",
	},
	{
		keywords: []string{"journal", "daily", "log", "diary"},
		typeName: "journal",
		signals:  []string{"journal", "daily", "log", "today"},
		template: "journal",
	},
	{
		keywords: []string{"note", "notes", "capture"},
		typeName: "note",
		signals:  []string{"note", "capture", "thought", "idea"},
		template: "",
	},
}

// knownNonOrgTags are tags that indicate status or category type, not an org.
var knownNonOrgTags = map[string]bool{
	"active": true, "inactive": true, "archived": true, "draft": true,
	"complete": true, "completed": true, "reviewed": true, "stable": true,
	"deprecated": true, "todo": true, "done": true, "wip": true,
	"decision": true, "concept": true, "meeting": true, "project": true,
	"learning": true, "resource": true, "person": true, "people": true,
	"journal": true, "note": true, "notes": true,
}

func inferCategories(state *analysisState, a *VaultAnalysis, pkmResult *PKMSystemResult) []InferredCategory {
	skipPaths := buildSkipSet(a)
	usedNames := make(map[string]bool)

	var categories []InferredCategory
	for _, folder := range a.Folders {
		if folder.NoteCount < 2 {
			continue
		}
		if shouldSkipFolder(folder.Path, skipPaths) {
			continue
		}
		absDir := filepath.Clean(filepath.Join(state.root, filepath.FromSlash(strings.TrimSuffix(folder.Path, "/"))))
		notes := state.notesByFolder[absDir]

		cat := inferSingleCategory(folder, notes, a, pkmResult)
		if cat == nil {
			continue
		}

		// Ensure unique name; disambiguate using parent folder segment if needed
		name := cat.Name
		if usedNames[name] {
			parts := strings.Split(strings.TrimSuffix(folder.Path, "/"), "/")
			if len(parts) >= 2 {
				name = slugifySegment(parts[len(parts)-2]) + "-" + cat.Name
			}
			if usedNames[name] {
				continue // skip unresolvable duplicate
			}
		}
		cat.Name = name
		usedNames[name] = true
		categories = append(categories, *cat)
	}

	return categories
}

func inferSingleCategory(folder FolderEntry, notes []*noteData, a *VaultAnalysis, pkmResult *PKMSystemResult) *InferredCategory {
	pathParts := strings.Split(strings.ToLower(strings.TrimSuffix(folder.Path, "/")), "/")

	hint, matchedByName := findHintFromPath(pathParts)
	folderTags := folderTagFreqFromNotes(notes)

	if hint == nil {
		hint, matchedByName = findHintFromTags(folderTags)
		if hint == nil {
			return nil
		}
	}

	orgTag := findOrgTag(folderTags, hint)
	mocPath := findMOCInFolder(folder.Path, a.MOCFiles)

	naming := findNamingForFolder(folder.Path, a.NamingPatterns)
	if naming == "" {
		naming = defaultNaming(hint.typeName)
	}

	template := hint.template
	if !templateExists(template, a.Templates) {
		template = ""
	}

	var tags []string
	if hint.typeName != "" {
		tags = append(tags, hint.typeName)
	}
	if orgTag != "" {
		tags = append(tags, orgTag)
	}

	catName := hint.typeName
	if orgTag != "" {
		catName = orgTag + "-" + hint.typeName
	}

	conf, reasoning := calcConfidence(folder, matchedByName, hint, orgTag, mocPath, template, naming, pkmResult)

	return &InferredCategory{
		Name:       catName,
		Folder:     folder.Path,
		Template:   template,
		Naming:     naming,
		Tags:       tags,
		MOC:        mocPath,
		Signals:    hint.signals,
		Confidence: conf,
		Reasoning:  reasoning,
	}
}

func findHintFromPath(pathParts []string) (*categoryHint, bool) {
	for _, part := range pathParts {
		for i := range categoryHints {
			for _, kw := range categoryHints[i].keywords {
				if part == kw || strings.Contains(part, kw) {
					return &categoryHints[i], true
				}
			}
		}
	}
	return nil, false
}

func findHintFromTags(tags []TagCount) (*categoryHint, bool) {
	for _, tc := range tags {
		for i := range categoryHints {
			for _, kw := range categoryHints[i].keywords {
				if tc.Tag == kw {
					return &categoryHints[i], false
				}
			}
		}
	}
	return nil, false
}

// folderTagFreqFromNotes returns tags sorted by frequency for pre-parsed notes.
func folderTagFreqFromNotes(notes []*noteData) []TagCount {
	counts := make(map[string]int)
	for _, nd := range notes {
		for _, tag := range nd.Tags {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag != "" {
				counts[tag]++
			}
		}
	}
	result := make([]TagCount, 0, len(counts))
	for tag, count := range counts {
		result = append(result, TagCount{Tag: tag, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Tag < result[j].Tag
	})
	return result
}

// findOrgTag returns the most frequent tag that is not a category/status tag.
// Requires >= 2 occurrences to be considered a real org tag.
func findOrgTag(folderTags []TagCount, hint *categoryHint) string {
	skip := make(map[string]bool)
	skip[hint.typeName] = true
	for _, kw := range hint.keywords {
		skip[kw] = true
	}
	for _, sig := range hint.signals {
		skip[sig] = true
	}
	for t := range knownNonOrgTags {
		skip[t] = true
	}
	for _, tc := range folderTags {
		if skip[tc.Tag] {
			continue
		}
		if tc.Count >= 2 {
			return tc.Tag
		}
	}
	return ""
}

func findMOCInFolder(folderPath string, mocFiles []string) string {
	folder := strings.TrimSuffix(folderPath, "/") + "/"
	for _, moc := range mocFiles {
		dir := filepath.ToSlash(filepath.Dir(moc)) + "/"
		if dir == folder {
			return moc
		}
	}
	return ""
}

func findNamingForFolder(folderPath string, patterns []NamingPattern) string {
	for _, p := range patterns {
		if p.Folder == folderPath {
			return p.Pattern
		}
	}
	return ""
}

func defaultNaming(typeName string) string {
	switch typeName {
	case "meeting":
		return "YYYY-MM-DD-{topic}.md"
	case "decision":
		return "use-{topic}.md"
	case "journal":
		return "YYYY-MM-DD.md"
	default:
		return "{topic}.md"
	}
}

func templateExists(name string, templates []TemplateEntry) bool {
	if name == "" {
		return false
	}
	for _, t := range templates {
		if t.Name == name {
			return true
		}
	}
	return false
}

func calcConfidence(folder FolderEntry, matchedByName bool, hint *categoryHint, orgTag, mocPath, template, naming string, pkmResult *PKMSystemResult) (float64, string) {
	var conf float64
	var reasons []string

	if matchedByName {
		conf += 0.40
		reasons = append(reasons, fmt.Sprintf("folder name suggests %q", hint.typeName))
	} else {
		conf += 0.25
		reasons = append(reasons, fmt.Sprintf("tags suggest %q", hint.typeName))
	}
	if mocPath != "" {
		conf += 0.20
		reasons = append(reasons, "MOC file present")
	}
	if orgTag != "" {
		conf += 0.10
		reasons = append(reasons, fmt.Sprintf("org tag %q in notes", orgTag))
	}
	if template != "" {
		conf += 0.15
		reasons = append(reasons, fmt.Sprintf("template %q found", template))
	}
	if naming != "" && naming != "{topic}.md" {
		conf += 0.15
		reasons = append(reasons, fmt.Sprintf("naming pattern %q", naming))
	}
	if isPKMAligned(hint.typeName, folder.Path, pkmResult) {
		conf += 0.10
		reasons = append(reasons, fmt.Sprintf("aligned with %s system", pkmResult.Primary))
	}
	if conf > 1.0 {
		conf = 1.0
	}
	return conf, fmt.Sprintf("%d notes; %s", folder.NoteCount, strings.Join(reasons, "; "))
}

// ── Skip helpers ──────────────────────────────────────────────────────────────

func buildSkipSet(a *VaultAnalysis) map[string]bool {
	skip := make(map[string]bool)
	for _, p := range []string{a.DetectedInbox, a.DetectedArchive, a.DetectedTemplatesDir} {
		if p != "" {
			skip[strings.ToLower(p)] = true
		}
	}
	return skip
}

func shouldSkipFolder(path string, skip map[string]bool) bool {
	return skip[strings.ToLower(path)]
}

// slugifySegment converts a folder name segment to lowercase-hyphenated form.
func slugifySegment(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
