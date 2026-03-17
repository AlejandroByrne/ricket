package vault

import (
	"fmt"
	"os"
	"strings"
)

// UpdateMOCFile appends a wikilink to a MOC (Map of Content) file.
// Finds the last line containing "- [[" and inserts after it.
// If no such line exists, appends at end of file.
// Returns false if the MOC file does not exist (non-fatal).
func UpdateMOCFile(mocAbsPath, noteTitle, notePath string) (bool, error) {
	data, err := os.ReadFile(mocAbsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read MOC file %s: %w", mocAbsPath, err)
	}

	lines := strings.Split(string(data), "\n")
	insertIndex := len(lines)

	// Find last line containing - [[
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "- [[") {
			insertIndex = i + 1
			break
		}
	}

	newLink := fmt.Sprintf("- [[%s]]", noteTitle)

	// Insert at insertIndex
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertIndex]...)
	newLines = append(newLines, newLink)
	newLines = append(newLines, lines[insertIndex:]...)

	updated := strings.Join(newLines, "\n")
	if err := os.WriteFile(mocAbsPath, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("failed to write MOC file %s: %w", mocAbsPath, err)
	}

	return true, nil
}
