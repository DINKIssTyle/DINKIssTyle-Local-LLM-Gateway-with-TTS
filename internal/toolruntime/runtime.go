// Package toolruntime owns provider-independent tool discovery, policy checks,
// validation, and in-process execution for the chat server.
package toolruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"dinkisstyle-chat/internal/mcp"
)

type Metadata struct {
	Category       string `json:"category,omitempty"`
	ReadOnly       bool   `json:"readOnly,omitempty"`
	SideEffecting  bool   `json:"sideEffecting,omitempty"`
	ParallelSafe   bool   `json:"parallelSafe,omitempty"`
	RequiresMemory bool   `json:"requiresMemory,omitempty"`
}

type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Metadata    Metadata        `json:"metadata,omitempty"`
}

type ExecutionContext struct {
	RequestID             string
	UserID                string
	EnableMemory          bool
	LocationInfo          string
	DisabledTools         []string
	DisallowedCommands    []string
	DisallowedDirectories []string
}

type Result struct {
	Content string                 `json:"content"`
	IsError bool                   `json:"isError,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

type Handler func(context.Context, ExecutionContext, json.RawMessage) (Result, error)

type registeredTool struct {
	definition Definition
	handler    Handler
}

type Registry struct {
	mu    sync.RWMutex
	order []string
	tools map[string]registeredTool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]registeredTool)}
}

func (r *Registry) Register(definition Definition, handler Handler) error {
	name := strings.TrimSpace(definition.Name)
	if name == "" {
		return fmt.Errorf("tool name is required")
	}
	if handler == nil {
		return fmt.Errorf("tool %q handler is required", name)
	}
	if len(definition.InputSchema) == 0 || !json.Valid(definition.InputSchema) {
		return fmt.Errorf("tool %q has an invalid input schema", name)
	}

	definition.Name = name
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q is already registered", name)
	}
	r.tools[name] = registeredTool{definition: definition, handler: handler}
	r.order = append(r.order, name)
	return nil
}

func (r *Registry) List(execCtx ExecutionContext) []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	disabled := stringSet(execCtx.DisabledTools)
	definitions := make([]Definition, 0, len(r.order))
	for _, name := range r.order {
		tool := r.tools[name]
		if disabled[name] || (tool.definition.Metadata.RequiresMemory && !execCtx.EnableMemory) {
			continue
		}
		definitions = append(definitions, tool.definition)
	}
	return definitions
}

func (r *Registry) Call(ctx context.Context, execCtx ExecutionContext, name string, arguments json.RawMessage) (result Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = Result{IsError: true}
			err = fmt.Errorf("tool %q panicked: %v", name, recovered)
		}
	}()
	name = strings.TrimSpace(name)
	name, normalizedArguments := mcp.NormalizeToolCall(name, arguments)
	arguments = json.RawMessage(normalizedArguments)
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return Result{IsError: true}, fmt.Errorf("tool not found: %s", name)
	}
	if stringSet(execCtx.DisabledTools)[name] {
		return Result{IsError: true}, fmt.Errorf("tool %q is disabled for this user", name)
	}
	if tool.definition.Metadata.RequiresMemory && !execCtx.EnableMemory {
		return Result{IsError: true}, fmt.Errorf("memory feature is disabled by user settings")
	}
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	if err := validateRequiredArguments(tool.definition.InputSchema, arguments); err != nil {
		return Result{IsError: true}, fmt.Errorf("invalid arguments for %s: %w", name, err)
	}
	if err := ctx.Err(); err != nil {
		return Result{IsError: true}, err
	}
	return tool.handler(ctx, execCtx, arguments)
}

func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	for _, legacy := range mcp.GetToolList() {
		if !supportedByGateway(legacy.Name) {
			continue
		}
		schema, err := json.Marshal(legacy.InputSchema)
		if err != nil {
			continue
		}
		definition := Definition{
			Name:        legacy.Name,
			Description: legacy.Description,
			InputSchema: schema,
			Metadata:    metadataFor(legacy.Name),
		}
		name := definition.Name
		_ = registry.Register(definition, func(ctx context.Context, execCtx ExecutionContext, arguments json.RawMessage) (Result, error) {
			content, callErr := mcp.ExecuteToolWithContext(name, arguments, mcp.ToolContext{
				RequestID:      execCtx.RequestID,
				UserID:         execCtx.UserID,
				EnableMemory:   execCtx.EnableMemory,
				DisabledTools:  execCtx.DisabledTools,
				LocationInfo:   execCtx.LocationInfo,
				DisallowedCmds: execCtx.DisallowedCommands,
				DisallowedDirs: execCtx.DisallowedDirectories,
			})
			return Result{Content: content, IsError: callErr != nil}, callErr
		})
	}
	return registry
}

func supportedByGateway(name string) bool {
	switch name {
	case "send_keys", "read_terminal_tail":
		return false
	default:
		return true
	}
}

func metadataFor(name string) Metadata {
	switch name {
	case "search_memory", "read_memory", "read_memory_context":
		return Metadata{Category: "memory", ReadOnly: true, ParallelSafe: true, RequiresMemory: true}
	case "delete_memory", "save_user_fact", "delete_user_fact":
		return Metadata{Category: "memory", SideEffecting: true, RequiresMemory: true}
	case "search_web", "search_web_multi", "read_web_page", "read_buffered_source", "read_help", "naver_search", "namu_wiki":
		return Metadata{Category: "web", ReadOnly: true, ParallelSafe: true}
	case "get_current_time", "get_current_location":
		return Metadata{Category: "context", ReadOnly: true, ParallelSafe: true}
	case "execute_command", "send_keys":
		return Metadata{Category: "system", SideEffecting: true}
	default:
		return Metadata{Category: "general"}
	}
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = true
		}
	}
	return set
}

func validateRequiredArguments(schemaJSON, argumentsJSON json.RawMessage) error {
	var schema struct {
		Type       string   `json:"type"`
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type     string `json:"type"`
			MinItems int    `json:"minItems"`
			MaxItems int    `json:"maxItems"`
			Items    *struct {
				Type string `json:"type"`
			} `json:"items"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return fmt.Errorf("invalid tool schema: %w", err)
	}
	var arguments map[string]interface{}
	if err := json.Unmarshal(argumentsJSON, &arguments); err != nil {
		return err
	}
	if strings.TrimSpace(schema.Type) == "object" && arguments == nil {
		return fmt.Errorf("arguments must be an object")
	}
	for _, name := range schema.Required {
		value, exists := arguments[name]
		if !exists || value == nil {
			return fmt.Errorf("argument %q is required", name)
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return fmt.Errorf("argument %q cannot be empty", name)
		}
	}
	for name, property := range schema.Properties {
		value, exists := arguments[name]
		if !exists || value == nil {
			continue
		}
		switch property.Type {
		case "string":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("argument %q must be a string", name)
			}
		case "integer":
			number, ok := value.(float64)
			if !ok || number != float64(int64(number)) {
				return fmt.Errorf("argument %q must be an integer", name)
			}
		case "array":
			items, ok := value.([]interface{})
			if !ok {
				return fmt.Errorf("argument %q must be an array", name)
			}
			if property.MinItems > 0 && len(items) < property.MinItems {
				return fmt.Errorf("argument %q must contain at least %d items", name, property.MinItems)
			}
			if property.MaxItems > 0 && len(items) > property.MaxItems {
				return fmt.Errorf("argument %q must contain at most %d items", name, property.MaxItems)
			}
			if property.Items != nil && property.Items.Type == "string" {
				for _, item := range items {
					if text, ok := item.(string); !ok || strings.TrimSpace(text) == "" {
						return fmt.Errorf("argument %q must contain non-empty strings", name)
					}
				}
			}
		}
	}
	return nil
}

var Default = NewDefaultRegistry()
