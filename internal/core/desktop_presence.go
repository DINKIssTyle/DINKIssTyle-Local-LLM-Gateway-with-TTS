package core

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/toolruntime"
)

var koreanWindowPresence = regexp.MustCompile(`(?i)^(?:현재\s+|지금\s+)?(?:컴퓨터\s*|주\s*디스플레이\s*)?화면에\s*(.{1,60}?)(\s*창)?(?:이|가)\s*(?:있나|있어|있나요|있습니까|보이나|보여|보이나요|보입니까)[?!.]*$`)
var englishWindowPresence = regexp.MustCompile(`(?i)^is\s+(?:there (?:an? )?)?(.{1,60}?)(\s+window)?\s+(?:visible\s+)?on\s+(?:the\s+)?(?:computer\s+|primary\s+)?screen[?!.]*$`)

func desktopWindowPresenceTarget(question string) (string, bool) {
	for _, pattern := range []*regexp.Regexp{koreanWindowPresence, englishWindowPresence} {
		if match := pattern.FindStringSubmatch(strings.TrimSpace(question)); match != nil {
			target := strings.TrimSpace(match[1])
			if match[2] == "" {
				switch desktopWindowOwnerKey(target) {
				case "googlechrome", "safari", "microsoftedge", "firefox", "lmstudio", "finder":
				default:
					return "", false
				}
			}
			// A request for window content needs a different observation.
			for _, cue := range []string{"다운로드", "진행", "오류", "탭", "download", "progress", "error", " tab", " and ", " 또는 ", " 하고 "} {
				if strings.Contains(strings.ToLower(target), cue) {
					return "", false
				}
			}
			return target, true
		}
	}
	return "", false
}

func desktopWindowOwnerKey(name string) string {
	key := strings.ToLower(strings.Join(strings.Fields(name), ""))
	switch key {
	case "chrome", "chromebrowser", "googlechromebrowser", "크롬", "크롬브라우저", "구글크롬", "구글크롬브라우저":
		return "googlechrome"
	case "사파리", "사파리브라우저":
		return "safari"
	case "엣지", "edge", "엣지브라우저", "마이크로소프트엣지":
		return "microsoftedge"
	case "파이어폭스":
		return "firefox"
	case "엘엠스튜디오":
		return "lmstudio"
	}
	return key
}

func runDesktopWindowPresence(ctx context.Context, target string, korean bool, execCtx toolruntime.ExecutionContext, emit func(string, map[string]any)) string {
	goal, step, done := "Check the "+target+" window", "Inspect primary display window list", "Read the current window list and check the app name"
	if korean {
		goal, step, done = target+" 창 확인", "주 디스플레이 창 목록 확인", "현재 창 목록에서 앱 이름 대조"
	}
	task := chatharness.DesktopTask{
		DesktopRequired: true, TargetApplication: target, Kind: "desktop_work", Goal: goal,
		Steps: []chatharness.DesktopTaskStep{{Title: step, DoneWhen: done}},
	}
	_ = task.Initialize()
	task.Steps[0].Status = "in_progress"
	emit("task.progress", map[string]any{"task": task})
	result, reason := callDiagnosticTool(ctx, toolruntime.Default, execCtx, func(kind, tool, callID string, payload map[string]any) {
		payload["tool"], payload["call_id"] = tool, callID
		emit(kind, payload)
	}, "computer_observe", "{}")

	var observation struct {
		FrontmostPID int `json:"frontmost_pid"`
		Windows      *[]struct {
			Owner string `json:"owner"`
			PID   int    `json:"pid"`
		} `json:"windows"`
	}
	if reason == "" && (json.Unmarshal([]byte(result.Content), &observation) != nil || observation.Windows == nil || len(result.ImageURLs) == 0) {
		reason = "The observation returned no usable window metadata."
		if korean {
			reason = "화면 관찰에서 유효한 창 목록을 받지 못했습니다."
		}
	}
	if reason != "" {
		task.Status = "blocked"
		task.Steps[0].Status = "blocked"
		task.Reason = reason
		emit("task.progress", map[string]any{"task": task})
		return reason
	}
	answer := fmt.Sprintf("The primary display window list contains no %s window.", target)
	if korean {
		answer = fmt.Sprintf("주 디스플레이의 창 목록에 %s 창이 없습니다.", target)
	}
	for _, window := range *observation.Windows {
		if desktopWindowOwnerKey(window.Owner) != desktopWindowOwnerKey(target) {
			continue
		}
		answer = fmt.Sprintf("Yes, %s has a window on the primary display. It is in the background.", window.Owner)
		if korean {
			answer = fmt.Sprintf("네, 주 디스플레이에 %s 창이 있습니다. 현재 앞에 있는 앱은 아닙니다.", window.Owner)
		}
		if window.PID > 0 && window.PID == observation.FrontmostPID {
			answer = fmt.Sprintf("Yes, %s has a window on the primary display and is the active app.", window.Owner)
			if korean {
				answer = fmt.Sprintf("네, 주 디스플레이에 %s 창이 있고 현재 활성 앱입니다.", window.Owner)
			}
		}
		break
	}
	task.Status = "completed"
	task.Steps[0].Status = "completed"
	task.Steps[0].EvidenceIDs = []int{1}
	emit("task.progress", map[string]any{"task": task})
	return answer
}
