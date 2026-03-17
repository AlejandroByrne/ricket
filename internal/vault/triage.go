package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AlejandroByrne/ricket/internal/config"
)

// TriageProposal is a suggested filing action for a single inbox note.
type TriageProposal struct {
	Source       string   `json:"source"`
	Category     string   `json:"category"`
	Destination  string   `json:"destination"`
	Template     string   `json:"template,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	MOC          string   `json:"moc,omitempty"`
	Confidence   float64  `json:"confidence"`
	Signals      []string `json:"matchedSignals,omitempty"`
	NeedsApprove bool     `json:"needsApproval"`
}

// TriageUnresolved captures inbox notes that could not be confidently classified.
type TriageUnresolved struct {
	Source  string `json:"source"`
	Preview string `json:"preview"`
	Reason  string `json:"reason"`
}

// TriagePlan contains deterministic filing suggestions for inbox notes.
type TriagePlan struct {
	GeneratedAt string             `json:"generatedAt"`
	Proposals   []TriageProposal   `json:"proposals"`
	Unresolved  []TriageUnresolved `json:"unresolved"`
}

// PlanInboxTriage analyzes inbox notes and proposes filing actions.
func (v *Vault) PlanInboxTriage() (TriagePlan, error) {
	inbox, err := v.ListInbox()
	if err != nil {
		return TriagePlan{}, err
	}

	plan := TriagePlan{GeneratedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, n := range inbox {
		category, score, matches := classifyNote(n, v.cfg.Categories)
		if category == nil || score == 0 {
			preview := n.Parsed.Content
			if len([]rune(preview)) > 160 {
				preview = string([]rune(preview)[:160])
			}
			plan.Unresolved = append(plan.Unresolved, TriageUnresolved{
				Source:  n.Path,
				Preview: preview,
				Reason:  "No category signals matched",
			})
			continue
		}

		destination := v.suggestDestination(*category, n)
		confidence := float64(score) / float64(maxInt(1, len(uniqueLower(category.Signals))))
		if confidence > 1 {
			confidence = 1
		}

		plan.Proposals = append(plan.Proposals, TriageProposal{
			Source:       n.Path,
			Category:     category.Name,
			Destination:  destination,
			Template:     category.Template,
			Tags:         append([]string(nil), category.Tags...),
			MOC:          category.MOC,
			Confidence:   confidence,
			Signals:      matches,
			NeedsApprove: true,
		})
	}

	sort.Slice(plan.Proposals, func(i, j int) bool {
		return plan.Proposals[i].Confidence > plan.Proposals[j].Confidence
	})
	return plan, nil
}

func classifyNote(note VaultNote, categories []config.Category) (*config.Category, int, []string) {
	text := strings.ToLower(note.Name + "\n" + note.Parsed.Content)
	tags := uniqueLower(GetTags(note.Parsed))

	bestScore := 0
	bestIdx := -1
	bestMatches := []string{}

	for i, c := range categories {
		score := 0
		matches := make([]string, 0)
		seen := map[string]bool{}

		for _, sig := range uniqueLower(c.Signals) {
			if sig == "" {
				continue
			}
			if strings.Contains(text, sig) {
				score += 2
				if !seen[sig] {
					matches = append(matches, sig)
					seen[sig] = true
				}
			}
		}

		for _, t := range uniqueLower(c.Tags) {
			if containsString(tags, t) {
				score++
				if !seen[t] {
					matches = append(matches, t)
					seen[t] = true
				}
			}
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
			bestMatches = matches
		}
	}

	if bestIdx == -1 {
		return nil, 0, nil
	}
	return &categories[bestIdx], bestScore, bestMatches
}

func (v *Vault) suggestDestination(cat config.Category, note VaultNote) string {
	topic := slugifyTopic(note.Name)
	filename := cat.Naming
	if filename == "" {
		filename = "{topic}.md"
	}
	filename = strings.ReplaceAll(filename, "{topic}", topic)
	filename = strings.ReplaceAll(filename, "YYYY-MM-DD", time.Now().Format("2006-01-02"))
	if !strings.HasSuffix(strings.ToLower(filename), ".md") {
		filename += ".md"
	}

	candidate := filepath.ToSlash(filepath.Join(cat.Folder, filename))
	if !v.destinationExists(candidate) {
		return candidate
	}

	base := strings.TrimSuffix(filename, ".md")
	for i := 2; i <= 99; i++ {
		next := filepath.ToSlash(filepath.Join(cat.Folder, fmt.Sprintf("%s-%d.md", base, i)))
		if !v.destinationExists(next) {
			return next
		}
	}
	return candidate
}

func (v *Vault) destinationExists(rel string) bool {
	_, err := os.Stat(filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(rel)))
	return err == nil
}

func slugifyTopic(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "note"
	}
	return out
}

func uniqueLower(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		norm := strings.ToLower(strings.TrimSpace(v))
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
