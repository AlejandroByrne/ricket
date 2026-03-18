package vault

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// PKMSystemResult holds the detected PKM methodology with confidence and evidence.
type PKMSystemResult struct {
	Primary             string   `json:"primary,omitempty"`
	Confidence          float64  `json:"confidence,omitempty"`
	Evidence            []string `json:"evidence,omitempty"`
	Secondary           string   `json:"secondary,omitempty"`
	SecondaryConfidence float64  `json:"secondaryConfidence,omitempty"`
	IsHybrid            bool     `json:"isHybrid,omitempty"`
}

// FrontmatterSchema holds frontmatter key frequency and notable key combinations.
type FrontmatterSchema struct {
	KeyFrequency  []KeyCount     `json:"keyFrequency"`
	NotableCombos []NotableCombo `json:"notableCombos,omitempty"`
}

// KeyCount tracks how often a frontmatter key appears.
type KeyCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// NotableCombo is a known frontmatter key combination that signals a PKM system.
type NotableCombo struct {
	Keys   []string `json:"keys"`
	Signal string   `json:"signal"`
	Count  int      `json:"count"`
}

// LinkAnalysis holds wikilink structure statistics.
type LinkAnalysis struct {
	TotalLinks     int      `json:"totalLinks"`
	AverageDensity float64  `json:"averageDensity"`
	HubNotes       []string `json:"hubNotes,omitempty"`
	MOCLikeCount   int      `json:"mocLikeCount"`
	OrphanCount    int      `json:"orphanCount"`
}

// TagTaxonomy holds tag structure analysis.
type TagTaxonomy struct {
	MaxDepth    int         `json:"maxDepth"`
	Prefixes    []TagPrefix `json:"prefixes,omitempty"`
	ContextTags []string    `json:"contextTags,omitempty"`
}

// TagPrefix tracks a nested tag prefix and its frequency.
type TagPrefix struct {
	Prefix string `json:"prefix"`
	Count  int    `json:"count"`
}

// ── PKM system detection ──────────────────────────────────────────────────────

type pkmScore struct {
	system   string
	score    float64
	evidence []string
}

var (
	zettelUIDRE = regexp.MustCompile(`^\d{12,14}`)
	jdFolderRE  = regexp.MustCompile(`^\d{2}-\d{2}$`)
	jdFileRE    = regexp.MustCompile(`^\d{2}\.\d{2}`)
)

// detectPKMSystem runs all detectors and resolves to primary (and optional hybrid) result.
func detectPKMSystem(state *analysisState) *PKMSystemResult {
	detectors := []func(*analysisState) pkmScore{
		detectPARA,
		detectLYT,
		detectACE,
		detectZettelkasten,
		detectJohnnyDecimal,
		detectGTD,
		detectBASB,
		detectEvergreen,
	}

	var scores []pkmScore
	for _, d := range detectors {
		s := d(state)
		if s.score > 0 {
			scores = append(scores, s)
		}
	}

	if len(scores) == 0 {
		return nil
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	top := scores[0]
	if top.score < 0.25 {
		return nil // Organic / undetectable → nil
	}

	result := &PKMSystemResult{
		Primary:    top.system,
		Confidence: top.score,
		Evidence:   top.evidence,
	}

	if len(scores) > 1 {
		second := scores[1]
		if second.score > 0.30 && (top.score-second.score) < 0.25 {
			result.IsHybrid = true
			result.Secondary = second.system
			result.SecondaryConfidence = second.score
		}
	}

	return result
}

// ── Individual detectors ──────────────────────────────────────────────────────

func detectPARA(state *analysisState) pkmScore {
	folders := []string{"projects", "areas", "resources", "archive"}
	var found []string
	for _, f := range folders {
		if state.folderSet[f] {
			found = append(found, f+"/")
		}
	}
	if len(found) == 0 {
		return pkmScore{}
	}
	score := float64(len(found)) * 0.175 // 4 folders = 0.70
	return pkmScore{
		system:   "PARA",
		score:    clampScore(score, 0.70),
		evidence: found,
	}
}

func detectLYT(state *analysisState) pkmScore {
	var score float64
	var evidence []string

	accessFolders := []string{"atlas", "calendar", "cards", "sources", "spaces"}
	for _, f := range accessFolders {
		if state.folderSet[f] {
			score += 0.15
			evidence = append(evidence, f+"/")
		}
	}

	mocCount := len(state.mocFiles)
	if mocCount > 3 {
		score += 0.15
		evidence = append(evidence, fmt.Sprintf("%d MOC files", mocCount))
	}

	if state.fmKeyFreq["up"] > 0 {
		ratio := float64(state.fmKeyFreq["up"]) / float64(max(state.totalNotes, 1))
		if ratio > 0.1 {
			score += 0.20
			evidence = append(evidence, fmt.Sprintf("up: key in %d notes", state.fmKeyFreq["up"]))
		}
	}

	return pkmScore{
		system:   "LYT/ACCESS",
		score:    clampScore(score, 0.95),
		evidence: evidence,
	}
}

func detectACE(state *analysisState) pkmScore {
	var score float64
	var evidence []string

	// Efforts/ is the unique differentiator from LYT
	if !state.folderSet["efforts"] {
		return pkmScore{}
	}

	aceFolders := map[string]float64{
		"atlas":    0.25,
		"calendar": 0.25,
		"efforts":  0.40,
	}
	for f, s := range aceFolders {
		if state.folderSet[f] {
			score += s
			evidence = append(evidence, f+"/")
		}
	}

	return pkmScore{
		system:   "ACE",
		score:    clampScore(score, 0.90),
		evidence: evidence,
	}
}

func detectZettelkasten(state *analysisState) pkmScore {
	var score float64
	var evidence []string

	// Timestamp UIDs in filenames
	uidCount := 0
	for _, nd := range state.allNotes {
		if zettelUIDRE.MatchString(nd.BaseName) {
			uidCount++
		}
	}
	if state.totalNotes > 0 {
		ratio := float64(uidCount) / float64(state.totalNotes)
		if ratio > 0.3 {
			score += 0.35
			evidence = append(evidence, fmt.Sprintf("%d/%d notes with timestamp UIDs", uidCount, state.totalNotes))
		}
	}

	// type: permanent/literature/fleeting frontmatter
	typeCount := 0
	for _, nd := range state.allNotes {
		if t, ok := nd.Parsed.Frontmatter["type"]; ok {
			ts := strings.ToLower(fmt.Sprintf("%v", t))
			if ts == "permanent" || ts == "literature" || ts == "fleeting" {
				typeCount++
			}
		}
	}
	if typeCount > 0 {
		score += 0.25
		evidence = append(evidence, fmt.Sprintf("type: permanent/literature/fleeting in %d notes", typeCount))
	}

	// Flat structure
	if state.maxFolderDepth <= 2 {
		score += 0.15
		evidence = append(evidence, "flat folder structure")
	}

	// High link density
	if state.totalNotes > 0 {
		avgLinks := float64(state.totalLinks) / float64(state.totalNotes)
		if avgLinks > 3.0 {
			score += 0.25
			evidence = append(evidence, fmt.Sprintf("high link density (%.1f links/note)", avgLinks))
		}
	}

	return pkmScore{
		system:   "Zettelkasten",
		score:    clampScore(score, 1.0),
		evidence: evidence,
	}
}

func detectJohnnyDecimal(state *analysisState) pkmScore {
	var score float64
	var evidence []string

	// NN-NN folder names
	jdFolderCount := 0
	for f := range state.allFolderSet {
		parts := strings.Split(f, "/")
		for _, part := range parts {
			if jdFolderRE.MatchString(part) {
				jdFolderCount++
			}
		}
	}
	if jdFolderCount >= 3 {
		score += 0.50
		evidence = append(evidence, fmt.Sprintf("%d folders with NN-NN pattern", jdFolderCount))
	}

	// NN.NN file IDs
	jdFileCount := 0
	for _, nd := range state.allNotes {
		if jdFileRE.MatchString(nd.BaseName) {
			jdFileCount++
		}
	}
	if state.totalNotes > 0 && jdFileCount > 0 {
		ratio := float64(jdFileCount) / float64(state.totalNotes)
		if ratio > 0.2 {
			score += 0.50
			evidence = append(evidence, fmt.Sprintf("%d/%d files with NN.NN IDs", jdFileCount, state.totalNotes))
		}
	}

	return pkmScore{
		system:   "Johnny.Decimal",
		score:    clampScore(score, 1.0),
		evidence: evidence,
	}
}

func detectGTD(state *analysisState) pkmScore {
	var score float64
	var evidence []string

	// GTD folder names
	gtdFolders := []string{"next actions", "next-actions", "waiting for", "waiting-for", "someday", "someday-maybe"}
	for _, f := range gtdFolders {
		if state.folderSet[f] || state.allFolderSet[f] {
			score += 0.25
			evidence = append(evidence, f+"/")
		}
	}

	// @context tags
	contextCount := 0
	for tag := range state.tagFreq {
		if strings.HasPrefix(tag, "@") {
			contextCount++
		}
	}
	if contextCount > 0 {
		score += 0.30
		evidence = append(evidence, fmt.Sprintf("%d @context tags", contextCount))
	}

	// Heavy checkbox usage
	if state.totalNotes > 0 {
		avgCheckboxes := float64(state.checkboxLines) / float64(state.totalNotes)
		if avgCheckboxes > 2.0 {
			score += 0.20
			evidence = append(evidence, fmt.Sprintf("%.1f checkboxes/note avg", avgCheckboxes))
		}
	}

	return pkmScore{
		system:   "GTD",
		score:    clampScore(score, 0.95),
		evidence: evidence,
	}
}

func detectBASB(state *analysisState) pkmScore {
	var score float64
	var evidence []string

	// BASB builds on PARA — check for PARA folders first
	paraFolders := []string{"projects", "areas", "resources", "archive"}
	paraCount := 0
	for _, f := range paraFolders {
		if state.folderSet[f] {
			paraCount++
		}
	}
	if paraCount >= 2 {
		score += 0.25
		evidence = append(evidence, fmt.Sprintf("%d/4 PARA folders", paraCount))
	}

	// source-type frontmatter key
	if state.fmKeyFreq["source-type"] > 0 {
		score += 0.25
		evidence = append(evidence, fmt.Sprintf("source-type key in %d notes", state.fmKeyFreq["source-type"]))
	}

	// summary or progressive-summary key
	summaryCount := state.fmKeyFreq["summary"] + state.fmKeyFreq["progressive-summary"]
	if summaryCount > 0 {
		score += 0.25
		evidence = append(evidence, fmt.Sprintf("summary/progressive-summary in %d notes", summaryCount))
	}

	return pkmScore{
		system:   "BASB",
		score:    clampScore(score, 0.75),
		evidence: evidence,
	}
}

func detectEvergreen(state *analysisState) pkmScore {
	var score float64
	var evidence []string

	// Assertion-style titles: start with verb, "How", "Why", "What", or are long
	assertionCount := 0
	for _, nd := range state.allNotes {
		if isAssertionTitle(nd.BaseName) {
			assertionCount++
		}
	}
	if state.totalNotes > 0 {
		ratio := float64(assertionCount) / float64(state.totalNotes)
		if ratio > 0.3 {
			score += 0.35
			evidence = append(evidence, fmt.Sprintf("%d/%d assertion-style titles", assertionCount, state.totalNotes))
		}
	}

	// Flat structure
	if state.maxFolderDepth <= 2 {
		score += 0.20
		evidence = append(evidence, "flat folder structure")
	}

	// Ultra-high link density
	if state.totalNotes > 0 {
		avgLinks := float64(state.totalLinks) / float64(state.totalNotes)
		if avgLinks > 5.0 {
			score += 0.40
			evidence = append(evidence, fmt.Sprintf("ultra-high link density (%.1f links/note)", avgLinks))
		}
	}

	return pkmScore{
		system:   "Evergreen",
		score:    clampScore(score, 0.95),
		evidence: evidence,
	}
}

// ── Analysis dimension functions ──────────────────────────────────────────────

func analyzeFrontmatterSchema(state *analysisState) *FrontmatterSchema {
	if len(state.fmKeyFreq) == 0 {
		return nil
	}

	fs := &FrontmatterSchema{}

	for key, count := range state.fmKeyFreq {
		fs.KeyFrequency = append(fs.KeyFrequency, KeyCount{Key: key, Count: count})
	}
	sort.Slice(fs.KeyFrequency, func(i, j int) bool {
		if fs.KeyFrequency[i].Count != fs.KeyFrequency[j].Count {
			return fs.KeyFrequency[i].Count > fs.KeyFrequency[j].Count
		}
		return fs.KeyFrequency[i].Key < fs.KeyFrequency[j].Key
	})
	if len(fs.KeyFrequency) > 30 {
		fs.KeyFrequency = fs.KeyFrequency[:30]
	}

	fs.NotableCombos = detectNotableCombos(state)
	return fs
}

// knownCombos defines frontmatter key combinations that signal specific PKM systems.
var knownCombos = []struct {
	keys         []string
	signal       string
	valueMatcher func(map[string]interface{}) bool // nil = just check key presence
}{
	{
		keys:   []string{"up", "related"},
		signal: "LYT",
	},
	{
		keys:   []string{"type"},
		signal: "Zettelkasten",
		valueMatcher: func(fm map[string]interface{}) bool {
			if t, ok := fm["type"]; ok {
				ts := strings.ToLower(fmt.Sprintf("%v", t))
				return ts == "permanent" || ts == "literature" || ts == "fleeting"
			}
			return false
		},
	},
	{
		keys:   []string{"source-type"},
		signal: "BASB",
	},
	{
		keys:   []string{"project", "area"},
		signal: "PARA-metadata",
	},
	{
		keys:   []string{"cssclasses"},
		signal: "Dataview",
	},
}

func detectNotableCombos(state *analysisState) []NotableCombo {
	var combos []NotableCombo

	for _, kc := range knownCombos {
		count := 0
		for _, nd := range state.allNotes {
			if kc.valueMatcher != nil {
				if kc.valueMatcher(nd.Parsed.Frontmatter) {
					count++
				}
				continue
			}
			allPresent := true
			for _, key := range kc.keys {
				if _, ok := nd.Parsed.Frontmatter[key]; !ok {
					allPresent = false
					break
				}
			}
			if allPresent {
				count++
			}
		}
		if count > 0 {
			combos = append(combos, NotableCombo{
				Keys:   kc.keys,
				Signal: kc.signal,
				Count:  count,
			})
		}
	}

	return combos
}

func analyzeLinkStructure(state *analysisState) *LinkAnalysis {
	if state.totalNotes == 0 {
		return nil
	}

	la := &LinkAnalysis{
		TotalLinks:     state.totalLinks,
		AverageDensity: float64(state.totalLinks) / float64(state.totalNotes),
	}

	// Hub notes: > 10 outbound links
	for _, nd := range state.allNotes {
		if len(nd.Wikilinks) > 10 {
			la.HubNotes = append(la.HubNotes, nd.RelPath)
		}
	}
	sort.Strings(la.HubNotes)

	// MOC-like: link-to-prose ratio > 0.5 and at least 3 links
	for _, nd := range state.allNotes {
		lines := strings.Split(nd.Parsed.Content, "\n")
		if len(lines) == 0 {
			continue
		}
		linkLines := 0
		for _, line := range lines {
			if strings.Contains(line, "[[") {
				linkLines++
			}
		}
		ratio := float64(linkLines) / float64(len(lines))
		if ratio > 0.5 && len(nd.Wikilinks) > 3 {
			la.MOCLikeCount++
		}
	}

	// Orphans: notes not linked from any other note
	linkedTargets := make(map[string]bool)
	for _, nd := range state.allNotes {
		for _, link := range nd.Wikilinks {
			linkedTargets[strings.ToLower(link)] = true
		}
	}
	for _, nd := range state.allNotes {
		if !linkedTargets[strings.ToLower(nd.BaseName)] {
			la.OrphanCount++
		}
	}

	return la
}

func analyzeTagTaxonomy(state *analysisState) *TagTaxonomy {
	if len(state.tagFreq) == 0 {
		return nil
	}

	tt := &TagTaxonomy{}

	prefixCounts := make(map[string]int)
	for tag, count := range state.tagFreq {
		depth := strings.Count(tag, "/") + 1
		if depth > tt.MaxDepth {
			tt.MaxDepth = depth
		}
		if strings.Contains(tag, "/") {
			prefix := strings.SplitN(tag, "/", 2)[0] + "/"
			prefixCounts[prefix] += count
		}
		if strings.HasPrefix(tag, "@") {
			tt.ContextTags = append(tt.ContextTags, tag)
		}
	}

	for prefix, count := range prefixCounts {
		tt.Prefixes = append(tt.Prefixes, TagPrefix{Prefix: prefix, Count: count})
	}
	sort.Slice(tt.Prefixes, func(i, j int) bool {
		return tt.Prefixes[i].Count > tt.Prefixes[j].Count
	})
	sort.Strings(tt.ContextTags)

	return tt
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func clampScore(score, maxVal float64) float64 {
	if score > maxVal {
		return maxVal
	}
	return score
}

func isAssertionTitle(baseName string) bool {
	words := strings.FieldsFunc(baseName, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(words) < 4 {
		return false
	}
	first := strings.ToLower(words[0])
	assertionStarters := map[string]bool{
		"how": true, "why": true, "what": true, "when": true, "where": true,
		"the": true, "every": true, "always": true, "never": true,
		"use": true, "avoid": true, "prefer": true, "consider": true,
	}
	return assertionStarters[first]
}

// isPKMAligned checks whether a category hint aligns with the detected PKM system.
func isPKMAligned(typeName, folderPath string, pkm *PKMSystemResult) bool {
	if pkm == nil {
		return false
	}
	sys := strings.ToLower(pkm.Primary)
	folderLower := strings.ToLower(folderPath)

	switch sys {
	case "para":
		return strings.Contains(folderLower, "projects") ||
			strings.Contains(folderLower, "areas") ||
			strings.Contains(folderLower, "resources")
	case "lyt/access":
		return strings.Contains(folderLower, "atlas") ||
			strings.Contains(folderLower, "cards") ||
			strings.Contains(folderLower, "spaces")
	case "ace":
		return strings.Contains(folderLower, "atlas") ||
			strings.Contains(folderLower, "efforts")
	case "zettelkasten":
		return typeName == "concept" || typeName == "note"
	case "gtd":
		return typeName == "project" || strings.Contains(folderLower, "next")
	case "basb":
		return strings.Contains(folderLower, "projects") ||
			strings.Contains(folderLower, "areas") ||
			strings.Contains(folderLower, "resources")
	}
	return false
}
