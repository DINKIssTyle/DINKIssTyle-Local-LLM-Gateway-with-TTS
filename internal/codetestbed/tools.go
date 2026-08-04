package codetestbed

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxReadBytes    = 256 * 1024
	maxSearchFiles  = 4000
	maxSearchResult = 120
	maxListEntries  = 400
)

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ToolExecutor struct {
	root          string
	rootResolved  string
	allowWrites   bool
	allowCommands bool
}

func NewToolExecutor(root string, allowWrites, allowCommands bool) (*ToolExecutor, error) {
	root, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("workspace must be a directory")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	return &ToolExecutor{root: root, rootResolved: resolved, allowWrites: allowWrites, allowCommands: allowCommands}, nil
}

func (e *ToolExecutor) Definitions() []ToolDefinition {
	definitions := []ToolDefinition{
		toolDefinition("list_files", "List files under a workspace-relative directory.", map[string]interface{}{
			"type": "object", "properties": map[string]interface{}{
				"path":      map[string]interface{}{"type": "string", "description": "Workspace-relative directory; use . for the root."},
				"max_depth": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 6},
			}, "required": []string{"path"},
		}),
		toolDefinition("search_text", "Search literal text in ordinary text files under the workspace.", map[string]interface{}{
			"type": "object", "properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string"},
				"path":  map[string]interface{}{"type": "string", "description": "Optional workspace-relative directory."},
			}, "required": []string{"query"},
		}),
		toolDefinition("read_file", "Read a bounded line range from a workspace file.", map[string]interface{}{
			"type": "object", "properties": map[string]interface{}{
				"path":       map[string]interface{}{"type": "string"},
				"start_line": map[string]interface{}{"type": "integer", "minimum": 1},
				"end_line":   map[string]interface{}{"type": "integer", "minimum": 1},
			}, "required": []string{"path"},
		}),
	}
	if e.allowWrites {
		definitions = append(definitions,
			toolDefinition("replace_text", "Replace one exact, unique text block in a workspace file. Read the file first.", map[string]interface{}{
				"type": "object", "properties": map[string]interface{}{
					"path":     map[string]interface{}{"type": "string"},
					"old_text": map[string]interface{}{"type": "string"},
					"new_text": map[string]interface{}{"type": "string"},
				}, "required": []string{"path", "old_text", "new_text"},
			}),
			toolDefinition("write_new_file", "Create a new file. This refuses to overwrite an existing path.", map[string]interface{}{
				"type": "object", "properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string"},
					"content": map[string]interface{}{"type": "string"},
				}, "required": []string{"path", "content"},
			}),
		)
	}
	if e.allowCommands {
		definitions = append(definitions, toolDefinition("run_command", "Run a safe allowlisted build, test, search, or git inspection command without a shell.", map[string]interface{}{
			"type": "object", "properties": map[string]interface{}{
				"argv":            map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "minItems": 1, "maxItems": 20},
				"cwd":             map[string]interface{}{"type": "string", "description": "Optional workspace-relative working directory."},
				"timeout_seconds": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 180},
			}, "required": []string{"argv"},
		}))
	}
	return definitions
}

func toolDefinition(name, description string, parameters map[string]interface{}) ToolDefinition {
	return ToolDefinition{Type: "function", Function: ToolFunction{Name: name, Description: description, Parameters: parameters}}
}

func (e *ToolExecutor) Execute(ctx context.Context, name string, raw json.RawMessage) (string, error) {
	switch strings.TrimSpace(name) {
	case "list_files":
		var args struct {
			Path     string `json:"path"`
			MaxDepth int    `json:"max_depth"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		return e.listFiles(args.Path, args.MaxDepth)
	case "search_text":
		var args struct {
			Query string `json:"query"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		return e.searchText(args.Query, args.Path)
	case "read_file":
		var args struct {
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		return e.readFile(args.Path, args.StartLine, args.EndLine)
	case "replace_text":
		if !e.allowWrites {
			return "", errors.New("writes are disabled")
		}
		var args struct {
			Path    string `json:"path"`
			OldText string `json:"old_text"`
			NewText string `json:"new_text"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		return e.replaceText(args.Path, args.OldText, args.NewText)
	case "write_new_file":
		if !e.allowWrites {
			return "", errors.New("writes are disabled")
		}
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		return e.writeNewFile(args.Path, args.Content)
	case "run_command":
		if !e.allowCommands {
			return "", errors.New("commands are disabled")
		}
		var args struct {
			Argv           []string `json:"argv"`
			CWD            string   `json:"cwd"`
			TimeoutSeconds int      `json:"timeout_seconds"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		return e.runCommand(ctx, args.Argv, args.CWD, args.TimeoutSeconds)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func (e *ToolExecutor) resolve(relative string, mustExist bool) (string, error) {
	relative = strings.TrimSpace(relative)
	if relative == "" || relative == "." {
		return e.rootResolved, nil
	}
	if filepath.IsAbs(relative) {
		return "", errors.New("absolute paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes workspace")
	}
	candidate := filepath.Join(e.rootResolved, clean)
	checkPath := candidate
	if !mustExist {
		checkPath = filepath.Dir(candidate)
	}
	resolved, err := filepath.EvalSymlinks(checkPath)
	if err != nil {
		return "", err
	}
	if !withinRoot(e.rootResolved, resolved) {
		return "", errors.New("symlink escapes workspace")
	}
	if mustExist {
		candidate = resolved
	}
	return candidate, nil
}

func withinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (e *ToolExecutor) listFiles(relative string, maxDepth int) (string, error) {
	base, err := e.resolve(relative, true)
	if err != nil {
		return "", err
	}
	if maxDepth <= 0 || maxDepth > 6 {
		maxDepth = 3
	}
	baseDepth := strings.Count(filepath.Clean(base), string(filepath.Separator))
	var entries []string
	err = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == base {
			return nil
		}
		rel, _ := filepath.Rel(e.rootResolved, path)
		depth := strings.Count(filepath.Clean(path), string(filepath.Separator)) - baseDepth
		if entry.IsDir() && ignoredDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		if depth > maxDepth {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			rel += "/"
		}
		entries = append(entries, filepath.ToSlash(rel))
		if len(entries) >= maxListEntries {
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	sort.Strings(entries)
	return strings.Join(entries, "\n"), nil
}

func (e *ToolExecutor) searchText(query, relative string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", errors.New("query is required")
	}
	if relative == "" {
		relative = "."
	}
	base, err := e.resolve(relative, true)
	if err != nil {
		return "", err
	}
	needle := strings.ToLower(query)
	var results []string
	filesSeen := 0
	err = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if path != base && ignoredDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		filesSeen++
		if filesSeen > maxSearchFiles {
			return io.EOF
		}
		info, err := entry.Info()
		if err != nil || info.Size() > maxReadBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		rel, _ := filepath.Rel(e.rootResolved, path)
		scanner := bufio.NewScanner(bytes.NewReader(data))
		line := 0
		for scanner.Scan() {
			line++
			if strings.Contains(strings.ToLower(scanner.Text()), needle) {
				preview := strings.TrimSpace(scanner.Text())
				if len(preview) > 240 {
					preview = preview[:240] + "…"
				}
				results = append(results, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), line, preview))
				if len(results) >= maxSearchResult {
					return io.EOF
				}
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if len(results) == 0 {
		return "No matches.", nil
	}
	return strings.Join(results, "\n"), nil
}

func (e *ToolExecutor) readFile(relative string, startLine, endLine int) (string, error) {
	path, err := e.resolve(relative, true)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > maxReadBytes {
		return "", errors.New("file is not a supported regular text file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", errors.New("binary files are not supported")
	}
	lines := strings.Split(string(data), "\n")
	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 || endLine > startLine+399 {
		endLine = startLine + 399
	}
	if startLine > len(lines) {
		return "", errors.New("start_line is past end of file")
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	var out strings.Builder
	for index := startLine; index <= endLine; index++ {
		fmt.Fprintf(&out, "%d\t%s\n", index, lines[index-1])
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

func (e *ToolExecutor) replaceText(relative, oldText, newText string) (string, error) {
	if oldText == "" {
		return "", errors.New("old_text is required")
	}
	path, err := e.resolve(relative, true)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	count := bytes.Count(data, []byte(oldText))
	if count != 1 {
		return "", fmt.Errorf("old_text must match exactly once; matched %d times", count)
	}
	updated := bytes.Replace(data, []byte(oldText), []byte(newText), 1)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, updated, info.Mode().Perm()); err != nil {
		return "", err
	}
	return fmt.Sprintf("Updated %s (%d bytes -> %d bytes).", filepath.ToSlash(relative), len(data), len(updated)), nil
}

func (e *ToolExecutor) writeNewFile(relative, content string) (string, error) {
	path, err := e.resolve(relative, false)
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Created %s (%d bytes).", filepath.ToSlash(relative), len(content)), nil
}

func (e *ToolExecutor) runCommand(parent context.Context, argv []string, cwd string, timeoutSeconds int) (string, error) {
	if err := validateCommand(argv); err != nil {
		return "", err
	}
	workdir := e.rootResolved
	if strings.TrimSpace(cwd) != "" && cwd != "." {
		resolved, err := e.resolve(cwd, true)
		if err != nil {
			return "", err
		}
		workdir = resolved
	}
	if timeoutSeconds <= 0 || timeoutSeconds > 180 {
		timeoutSeconds = 90
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = workdir
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var output limitedBuffer
	output.limit = 96 * 1024
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	text := strings.TrimSpace(output.String())
	if ctx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("command timed out after %ds", timeoutSeconds)
	}
	if err != nil {
		return text, fmt.Errorf("command failed: %w", err)
	}
	if text == "" {
		text = "Command completed with no output."
	}
	return text, nil
}

func validateCommand(argv []string) error {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return errors.New("argv is required")
	}
	for _, arg := range argv {
		if strings.ContainsRune(arg, 0) {
			return errors.New("invalid command argument")
		}
	}
	allowed := false
	switch argv[0] {
	case "go":
		allowed = len(argv) >= 2 && (argv[1] == "test" || argv[1] == "vet" || argv[1] == "build")
	case "npm":
		allowed = len(argv) >= 2 && (argv[1] == "test" || (argv[1] == "run" && len(argv) >= 3 && map[string]bool{"test": true, "build": true, "lint": true}[argv[2]]))
	case "git":
		allowed = len(argv) >= 2 && map[string]bool{"status": true, "diff": true, "grep": true, "log": true}[argv[1]]
	case "rg":
		allowed = true
	case "cargo":
		allowed = len(argv) >= 2 && (argv[1] == "test" || argv[1] == "check" || argv[1] == "build")
	case "swift":
		allowed = len(argv) >= 2 && (argv[1] == "test" || argv[1] == "build")
	case "python3", "python":
		allowed = len(argv) >= 3 && argv[1] == "-m" && (argv[2] == "pytest" || argv[2] == "unittest")
	}
	if !allowed {
		return fmt.Errorf("command is not allowlisted: %s", strings.Join(argv, " "))
	}
	return nil
}

func ignoredDirectory(name string) bool {
	return map[string]bool{".git": true, "node_modules": true, "vendor": true, ".cache": true, ".gocache": true, "build": true, "dist": true}[name]
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	original := len(data)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = b.buffer.Write(data)
	}
	return original, nil
}

func (b *limitedBuffer) String() string {
	text := b.buffer.String()
	if b.buffer.Len() >= b.limit {
		text += "\n… output truncated at " + strconv.Itoa(b.limit) + " bytes"
	}
	return text
}
