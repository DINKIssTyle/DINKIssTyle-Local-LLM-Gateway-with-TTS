package chatharness

import (
	"encoding/json"
	"testing"
)

func taskFixture(t *testing.T) DesktopTask {
	t.Helper()
	task := DesktopTask{DesktopRequired: true, Kind: "desktop_work", Goal: "Read task status", Steps: []DesktopTaskStep{{Title: "Find app", DoneWhen: "App PID found"}, {Title: "Read progress", DoneWhen: "Progress visible"}}}
	if err := task.Initialize(); err != nil {
		t.Fatal(err)
	}
	return task
}
func decisionFixture(t *testing.T, raw string) DesktopDecision {
	t.Helper()
	var d DesktopDecision
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	return d
}
func TestTaskCannotFinishFromPromisesOrDiscovery(t *testing.T) {
	completed := `{"answer":"Done","steps":{"1":{"status":"completed","evidence_ids":[1]},"2":{"status":"completed","evidence_ids":[1]}},"action_type":"completed","findings":["Done"]}`
	for _, receipts := range [][]DesktopReceipt{nil, {{ID: 1, Tool: "computer_list_apps", Success: true}}, {{ID: 1, Tool: "computer_observe", Success: false, HasImage: true}}} {
		task := taskFixture(t)
		if err := task.Apply(decisionFixture(t, completed), receipts, nil); err == nil {
			t.Fatal("unverified task accepted as completed")
		}
		if task.Steps[0].Status != "pending" || task.Status != "running" {
			t.Fatal("invalid review partially changed task")
		}
	}
}
func TestTaskRequiresPostActionObservationAndEveryStep(t *testing.T) {
	task := taskFixture(t)
	receipts := []DesktopReceipt{{ID: 1, Tool: "computer_list_apps", Success: true}, {ID: 2, Tool: "computer_observe", Success: true, HasImage: true}, {ID: 3, Tool: "computer_click", Success: true}}
	d := decisionFixture(t, `{"answer":"42%","steps":{"1":{"status":"completed","evidence_ids":[1]},"2":{"status":"completed","evidence_ids":[2]}},"action_type":"completed","findings":["42%"]}`)
	if err := task.Apply(d, receipts, nil); err == nil {
		t.Fatal("stale screenshot completed a changed desktop")
	}
	receipts = append(receipts, DesktopReceipt{ID: 4, Tool: "computer_observe", Success: true, HasImage: true})
	update := d.Steps["2"]
	update.EvidenceIDs = []int{4}
	d.Steps["2"] = update
	if err := task.Apply(d, receipts, nil); err != nil {
		t.Fatal(err)
	}
	if task.Status != "completed" {
		t.Fatal("verified task did not complete")
	}
}
func TestTaskAllowsAvailableActionsButNotInventedTools(t *testing.T) {
	task := taskFixture(t)
	d := decisionFixture(t, `{"next_tool":"computer_list_apps","arguments":{},"next_step":1,"steps":{"1":{"status":"pending","evidence_ids":[]},"2":{"status":"pending","evidence_ids":[]}},"action_type":"continue"}`)
	if err := task.Apply(d, nil, map[string]bool{}); err == nil {
		t.Fatal("unavailable tool allowed")
	}
	if err := task.Apply(d, nil, map[string]bool{"computer_list_apps": true}); err != nil {
		t.Fatal(err)
	}
	if task.Steps[0].Status != "in_progress" {
		t.Fatal("next step not activated")
	}
}
func TestTaskProgressSurvivesSessionSnapshot(t *testing.T) {
	task := taskFixture(t)
	snapshot := ParseUISnapshot("{}")
	updateTurnSnapshot(&snapshot, "turn1", "assistant", "task.progress", map[string]interface{}{"task": task})
	restored := ParseUISnapshot(EncodeUISnapshot(snapshot))
	if len(restored.Turns) != 1 || restored.Turns[0].Task == nil || restored.Turns[0].Task.Goal != task.Goal {
		t.Fatalf("task plan lost on reload: %+v", restored)
	}
}

func TestTaskCannotCompleteFromAnotherApplicationScreen(t *testing.T) {
	task := taskFixture(t)
	task.TargetApplication = "LM Studio"
	d := decisionFixture(t, `{"action_type":"completed","findings":["42%"],"answer":"42%","steps":{"1":{"status":"completed","evidence_ids":[1]},"2":{"status":"completed","evidence_ids":[1]}}}`)
	receipts := []DesktopReceipt{{ID: 1, Tool: "computer_observe", Success: true, HasImage: true, Result: `{"application":"DKST LLM Chat Server","frontmost_pid":7}`}}
	if err := task.Apply(d, receipts, nil); err == nil {
		t.Fatal("assistant's own UI completed an LM Studio inspection")
	}
	if task.Status != "running" || task.Steps[0].Status != "pending" {
		t.Fatal("rejected completion changed task state")
	}
	receipts[0].Result = `{"application":"LM Studio","frontmost_pid":42}`
	if err := task.Apply(d, receipts, nil); err != nil {
		t.Fatal(err)
	}
}
