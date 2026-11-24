package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"dinkisstyle-chat/internal/chatharness"
	"dinkisstyle-chat/internal/toolruntime"
)

const maxStoredComputerScreenshots = 100

type attachedScreenshot struct {
	URL     string
	Caption string
}

var computerScreenshots = &computerScreenshotStore{}

type computerScreenshotStore struct {
	mu      sync.Mutex
	dirFunc func() string
}

func (s *computerScreenshotStore) dir() string {
	if s.dirFunc != nil {
		return s.dirFunc()
	}
	return filepath.Join(GetAppDataDir(), "computer_screenshots")
}

func screenshotOwnerPrefix(userID string) string {
	sum := sha256.Sum256([]byte("dkst-shot:" + strings.TrimSpace(userID)))
	return hex.EncodeToString(sum[:6])
}

func isSafeScreenshotID(id string) bool {
	if len(id) < 20 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || r == '-') {
			return false
		}
	}
	return strings.Count(id, "-") == 1
}

func (s *computerScreenshotStore) SaveDataURL(userID, dataURL string) (string, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		return "", fmt.Errorf("unsupported screenshot data URL")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, prefix))
	if err != nil {
		return "", fmt.Errorf("decode screenshot base64: %w", err)
	}
	return s.SavePNG(userID, raw)
}

func (s *computerScreenshotStore) SavePNG(userID string, pngBytes []byte) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", fmt.Errorf("authenticated user required to store screenshot")
	}
	if len(pngBytes) == 0 {
		return "", fmt.Errorf("empty screenshot")
	}
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	id := screenshotOwnerPrefix(userID) + "-" + hex.EncodeToString(token[:])

	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.dir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, id+".png")
	if err := os.WriteFile(path, pngBytes, 0600); err != nil {
		return "", err
	}
	s.pruneLocked(dir)
	return "/api/computer/screenshots/" + id + ".png", nil
}

func (s *computerScreenshotStore) pruneLocked(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) <= maxStoredComputerScreenshots {
		return
	}
	type fileEntry struct {
		name string
		mod  int64
	}
	files := make([]fileEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		if info, err := e.Info(); err == nil {
			files = append(files, fileEntry{name: e.Name(), mod: info.ModTime().UnixNano()})
		}
	}
	if len(files) <= maxStoredComputerScreenshots {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod < files[j].mod })
	for _, f := range files[:len(files)-maxStoredComputerScreenshots] {
		_ = os.Remove(filepath.Join(dir, f.name))
	}
}

func (s *computerScreenshotStore) Load(userID, id string) ([]byte, bool) {
	userID = strings.TrimSpace(userID)
	if userID == "" || !isSafeScreenshotID(id) || !strings.HasPrefix(id, screenshotOwnerPrefix(userID)+"-") {
		return nil, false
	}
	raw, err := os.ReadFile(filepath.Join(s.dir(), id+".png"))
	if err != nil {
		return nil, false
	}
	return raw, true
}

// PurgeUser immediately deletes all stored screenshot files belonging to userID.
func (s *computerScreenshotStore) PurgeUser(userID string) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}
	prefix := screenshotOwnerPrefix(userID) + "-"
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".png") {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

func handleComputerScreenshot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/computer/screenshots/")
		if !strings.HasSuffix(name, ".png") || strings.Contains(name, "/") {
			http.NotFound(w, r)
			return
		}
		id := strings.TrimSuffix(name, ".png")
		raw, ok := computerScreenshots.Load(userID, id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store, private")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(raw)
	}
}

func appendScreenshotMarkdown(answer string, shots []attachedScreenshot) string {
	if len(shots) == 0 {
		return answer
	}
	answer = strings.TrimSpace(answer)
	seen := map[string]bool{}
	var parts []string
	for _, shot := range shots {
		if shot.URL == "" || seen[shot.URL] || strings.Contains(answer, shot.URL) {
			continue
		}
		seen[shot.URL] = true
		alt := strings.TrimSpace(shot.Caption)
		if alt == "" {
			alt = "화면 캡처"
		}
		alt = strings.NewReplacer("[", "(", "]", ")", "\n", " ", "\r", " ").Replace(alt)
		parts = append(parts, fmt.Sprintf("![%s](%s)", alt, shot.URL))
	}
	if len(parts) == 0 {
		return answer
	}
	if answer != "" {
		return answer + "\n\n" + strings.Join(parts, "\n\n")
	}
	return strings.Join(parts, "\n\n")
}

var simpleFullScreenshot = regexp.MustCompile(`(?i)^(?:(?:\$(?:builtin:)?computer-use|컴퓨터(?:\s*유즈(?:\s*스킬로)?)?)\s+)?(?:(?:현재\s*|전체\s*)?화면(?:을)?\s*(?:찍어서|캡처해서|캡처해(?:서)?)\s*(?:보내|첨부해)\s*(?:줘|주세요)?|(?:take|attach|send)(?: me)? (?:a |the )?(?:full[- ]screen )?screenshot(?: of (?:the |my )?screen)?)[.!?]*$`)

func normalizeScreenshotRequest(question string) string {
	return strings.ReplaceAll(strings.TrimSpace(question), "캡쳐", "캡처")
}

func isSimpleFullScreenshotRequest(question string) bool {
	return simpleFullScreenshot.MatchString(normalizeScreenshotRequest(question))
}

func screenshotAttachmentRequested(question string) bool {
	if isSimpleFullScreenshotRequest(question) {
		return true
	}
	q := strings.ToLower(normalizeScreenshotRequest(question))
	capture := strings.Contains(q, "스크린샷") || strings.Contains(q, "캡처") || strings.Contains(q, "screenshot") || (strings.Contains(q, "화면") && strings.Contains(q, "찍"))
	delivery := strings.Contains(q, "첨부") || strings.Contains(q, "보내") || strings.Contains(q, "attach") || strings.Contains(q, "send")
	return capture && delivery
}

func saveDesktopScreenshot(userID, caption string, result toolruntime.Result) (attachedScreenshot, error) {
	if len(result.ImageURLs) == 0 {
		return attachedScreenshot{}, fmt.Errorf("screenshot tool returned no image")
	}
	url, err := computerScreenshots.SaveDataURL(userID, result.ImageURLs[0])
	if err != nil {
		return attachedScreenshot{}, fmt.Errorf("save screenshot attachment: %w", err)
	}
	return attachedScreenshot{URL: url, Caption: caption}, nil
}

func captureFullDesktopScreenshot(ctx context.Context, question string, execCtx toolruntime.ExecutionContext, emit func(string, map[string]any)) string {
	const tool = "computer_screenshot"
	task := chatharness.DesktopTask{
		DesktopRequired: true, Kind: "desktop_work", Goal: question, Status: "running",
		Steps: []chatharness.DesktopTaskStep{{ID: 1, Title: "화면 캡처 첨부", DoneWhen: "저장된 이미지에 첨부 URL 발급", Status: "in_progress"}},
	}
	progress := func() { emit("task.progress", map[string]any{"task": task}) }
	progress()
	emit("tool_call.start", map[string]any{"tool": tool})
	emit("tool_call.arguments", map[string]any{"tool": tool, "arguments": map[string]any{}})
	result, err := toolruntime.Default.Call(ctx, execCtx, tool, []byte("{}"))
	if err == nil && result.IsError {
		err = fmt.Errorf("%s", result.Content)
	}
	var shot attachedScreenshot
	if err == nil {
		shot, err = saveDesktopScreenshot(execCtx.UserID, "", result)
	}
	if err != nil {
		task.Status, task.Steps[0].Status = "blocked", "blocked"
		task.Reason = "화면 캡처를 첨부하지 못했습니다. " + err.Error()
		emit("tool_call.failure", map[string]any{"tool": tool, "reason": err.Error()})
		progress()
		return task.Reason
	}
	emit("tool_call.success", map[string]any{"tool": tool})
	task.Status, task.Steps[0].Status = "completed", "completed"
	task.Steps[0].EvidenceIDs = []int{1}
	progress()
	return appendScreenshotMarkdown("화면을 캡처해 첨부했습니다.", []attachedScreenshot{shot})
}
