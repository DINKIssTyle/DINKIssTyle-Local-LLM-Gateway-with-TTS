package chatharness

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"dinkisstyle-chat/internal/computeruse"
)

type DesktopTaskStep struct {
	ID             int    `json:"id"`
	Title          string `json:"title"`
	DoneWhen       string `json:"done_when"`
	RequiresScreen bool   `json:"requires_screen"`
	Status         string `json:"status"`
	EvidenceIDs    []int  `json:"evidence_ids"`
}

func DesktopTaskLanguageInstruction(question string) string {
	return responseLanguageInstruction(question) + " Apply the same language to goal, step titles, done_when and answer."
}

type DesktopTask struct {
	AllowTextInput    bool              `json:"allow_text_input"`
	TargetApplication string            `json:"target_application,omitempty"`
	DesktopRequired   bool              `json:"desktop_required"`
	Kind              string            `json:"kind"`
	Goal              string            `json:"goal"`
	Steps             []DesktopTaskStep `json:"steps"`
	Status            string            `json:"status"`
	Reason            string            `json:"reason,omitempty"`
}

type DesktopReceipt struct {
	ID        int             `json:"id"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	Result    string          `json:"result"`
	Success   bool            `json:"success"`
	HasImage  bool            `json:"has_image"`
}

type DesktopDecision struct {
	Findings []string `json:"findings"`
	Outcome  string   `json:"action_type"`
	Steps    map[string]struct {
		Status      string `json:"status"`
		EvidenceIDs []int  `json:"evidence_ids"`
	} `json:"steps"`
	NextTool    string          `json:"next_tool"`
	Arguments   json.RawMessage `json:"arguments"`
	NextStep    int             `json:"next_step"`
	EvidenceIDs []int           `json:"evidence_ids"`
	Answer      string          `json:"answer"`
}

func (t *DesktopTask) Initialize() error {
	if !t.DesktopRequired {
		return nil
	}
	if t.Kind != "list_apps" && t.Kind != "desktop_work" {
		return fmt.Errorf("invalid desktop task kind")
	}
	if strings.TrimSpace(t.Goal) == "" || len(t.Steps) == 0 || len(t.Steps) > 6 {
		return fmt.Errorf("desktop task needs a goal and 1 to 6 steps")
	}
	t.Status = "running"
	for i := range t.Steps {
		s := &t.Steps[i]
		s.ID = i + 1
		if strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.DoneWhen) == "" {
			return fmt.Errorf("each step needs a title and observable completion criterion")
		}
		s.Status, s.EvidenceIDs = "pending", nil
		if t.Kind == "list_apps" {
			s.RequiresScreen = false
		}
	}
	// An app list can answer a list-only request; other desktop outcomes need
	// visual confirmation even if the planner forgets to request it.
	if t.Kind != "list_apps" {
		t.Steps[len(t.Steps)-1].RequiresScreen = true
	}
	return nil
}

// Apply verifies model judgments against app-owned execution receipts. Planning
// text and claimed actions are never receipts. Validate atomically before update.
func (t *DesktopTask) Apply(d DesktopDecision, receipts []DesktopReceipt, allowed map[string]bool) error {
	byID := map[int]DesktopReceipt{}
	for _, r := range receipts {
		byID[r.ID] = r
	}
	if len(d.Steps) != len(t.Steps) {
		return fmt.Errorf("review must report every planned step")
	}
	steps := append([]DesktopTaskStep(nil), t.Steps...)
	seen := map[int]bool{}
	for key, update := range d.Steps {
		id, err := strconv.Atoi(key)
		if err != nil || id < 1 || id > len(steps) || seen[id] {
			return fmt.Errorf("invalid or duplicate step ID")
		}
		seen[id] = true
		s := &steps[id-1]
		if update.Status != "pending" && update.Status != "in_progress" && update.Status != "completed" {
			return fmt.Errorf("invalid step status")
		}
		if update.Status == "completed" {
			if len(update.EvidenceIDs) == 0 {
				return fmt.Errorf("completed step %d lacks actual tool evidence", id)
			}
			visual := false
			for _, receiptID := range update.EvidenceIDs {
				r, ok := byID[receiptID]
				if !ok || !r.Success {
					return fmt.Errorf("step %d cites nonexistent or failed evidence", id)
				}
				visual = visual || (r.Tool == "computer_observe" && r.HasImage)
			}
			if s.RequiresScreen && !visual {
				return fmt.Errorf("step %d requires a successful screen observation", id)
			}
		}
		s.Status, s.EvidenceIDs = update.Status, update.EvidenceIDs
	}
	switch d.Outcome {
	case "continue":
		if !allowed[d.NextTool] || d.NextStep < 1 || d.NextStep > len(steps) || steps[d.NextStep-1].Status == "completed" {
			return fmt.Errorf("continuation needs an available tool and unfinished step")
		}
		var args map[string]any
		if json.Unmarshal(d.Arguments, &args) != nil || args == nil {
			return fmt.Errorf("next tool arguments must be a JSON object")
		}
		steps[d.NextStep-1].Status = "in_progress"
	case "completed":
		if len(d.Findings) == 0 {
			return fmt.Errorf("completion requires concrete observed findings, not a claim that information is displayed")
		}
		for _, fact := range d.Findings {
			if len([]rune(strings.TrimSpace(fact))) < 2 || !strings.Contains(d.Answer, strings.TrimSpace(fact)) {
				return fmt.Errorf("answer must include each concrete observed finding")
			}
		}
		for _, s := range steps {
			if s.Status != "completed" {
				return fmt.Errorf("cannot finish while a task step remains unfinished")
			}
		}
		if t.Kind != "list_apps" {
			lastInput, lastScreen := 0, 0
			for _, r := range receipts {
				if r.Tool == "computer_observe" && r.Success && r.HasImage {
					lastScreen = r.ID
				}
				if r.Tool != "computer_observe" && r.Tool != "computer_list_apps" {
					lastInput = r.ID
				}
			}
			if lastScreen == 0 || lastScreen < lastInput {
				return fmt.Errorf("completion requires a screen observation after the last action")
			}
			if !slices.Contains(steps[len(steps)-1].EvidenceIDs, lastScreen) {
				return fmt.Errorf("final step must cite the latest successful screen observation")
			}
			if t.TargetApplication != "" {
				var screen struct {
					Application string `json:"application"`
					PID         int    `json:"frontmost_pid"`
				}
				if json.Unmarshal([]byte(byID[lastScreen].Result), &screen) != nil || !DesktopApplicationMatches(t.TargetApplication, screen.Application) {
					return fmt.Errorf("latest screenshot is not from the requested application %q; switch to the target app before completing", t.TargetApplication)
				}
			}
		}
		if strings.TrimSpace(d.Answer) == "" {
			return fmt.Errorf("completion needs an evidence-based answer")
		}
	case "blocked":
		if strings.TrimSpace(d.Answer) == "" || len(d.EvidenceIDs) == 0 {
			return fmt.Errorf("blocker needs observed evidence and an explanation")
		}
		for _, id := range d.EvidenceIDs {
			if _, ok := byID[id]; !ok {
				return fmt.Errorf("blocker cites nonexistent evidence")
			}
		}
	default:
		return fmt.Errorf("invalid task outcome")
	}
	t.Steps = steps
	if d.Outcome != "continue" {
		t.Status = d.Outcome
		t.Reason = d.Answer
	}
	return nil
}

func DesktopApplicationMatches(target, actual string) bool {
	if alias := computeruse.ApplicationBundleAlias(target); alias != "" {
		return alias == computeruse.ApplicationBundleAlias(actual)
	}
	normalize := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), "")) }
	target, actual = normalize(target), normalize(actual)
	return target != "" && actual != "" && (target == actual || strings.Contains(actual, target))
}
