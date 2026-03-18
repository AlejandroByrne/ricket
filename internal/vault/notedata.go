package vault

// noteData holds all parsed information for a single markdown note.
// Computed once during the single-pass vault walk, then shared across
// all analysis and detection functions.
type noteData struct {
	RelPath   string     // relative to vault root, forward slashes
	AbsPath   string     // absolute filesystem path
	Parsed    ParsedNote // frontmatter, content, raw
	Tags      []string   // extracted from frontmatter
	Wikilinks []string   // extracted from content via [[...]]
	Folder    string     // relative folder path with trailing slash, "" for root
	BaseName  string     // filename without .md extension
}

// analysisState holds intermediate results from the single-pass vault walk.
// Shared across analysis functions and PKM detectors.
type analysisState struct {
	root           string
	allNotes       []*noteData
	notesByFolder  map[string][]*noteData // abs dir → notes
	folderSet      map[string]bool        // lowercase top-level folder names
	allFolderSet   map[string]bool        // lowercase all relative folder paths
	tagFreq        map[string]int         // global tag frequency
	fmKeyFreq      map[string]int         // global frontmatter key frequency
	linkTargets    map[string]int         // wikilink target (lowercase) → inbound count
	totalLinks     int
	mocFiles       []string
	checkboxLines  int
	totalNotes     int
	maxFolderDepth int
}
