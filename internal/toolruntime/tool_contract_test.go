package toolruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"dinkisstyle-chat/internal/mcp"
)

func callToolForContract(t *testing.T, execCtx ExecutionContext, name string, arguments string) string {
	t.Helper()
	result, err := Default.Call(context.Background(), execCtx, name, json.RawMessage(arguments))
	if err != nil {
		t.Fatalf("%s failed: %v (result=%#v)", name, err, result)
	}
	if result.IsError {
		t.Fatalf("%s returned an error result: %#v", name, result)
	}
	if strings.TrimSpace(result.Content) == "" {
		t.Fatalf("%s returned empty content", name)
	}
	return result.Content
}

func TestDefaultRegistryNonNetworkToolContracts(t *testing.T) {
	if err := mcp.InitDB(filepath.Join(t.TempDir(), "tool-contract.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mcp.CloseDB)

	const userID = "tool-contract-user"
	execCtx := ExecutionContext{
		RequestID:    "tool-contract-request",
		UserID:       userID,
		EnableMemory: true,
		LocationInfo: "Seoul, South Korea",
	}

	if got := callToolForContract(t, execCtx, "get_current_time", `{}`); !strings.Contains(got, "Current Local Time:") {
		t.Fatalf("unexpected current time result: %s", got)
	}
	if got := callToolForContract(t, execCtx, "get_current_location", `{}`); got != execCtx.LocationInfo {
		t.Fatalf("unexpected location result: %s", got)
	}

	helpHandle := callToolForContract(t, execCtx, "read_help", `{"question":"인증서 설정"}`)
	sourceMatch := regexp.MustCompile(`Source ID:\s*(src_[0-9a-f]+)`).FindStringSubmatch(helpHandle)
	if len(sourceMatch) != 2 {
		t.Fatalf("read_help did not return a buffered source handle:\n%s", helpHandle)
	}
	bufferArgs, _ := json.Marshal(map[string]interface{}{
		"source_id":  sourceMatch[1],
		"query":      "인증서",
		"max_chunks": 2,
	})
	if got := callToolForContract(t, execCtx, "read_buffered_source", string(bufferArgs)); !strings.Contains(strings.ToLower(got), "certificate") && !strings.Contains(got, "인증서") {
		t.Fatalf("buffered help lookup returned unrelated content:\n%s", got)
	}

	memoryID, err := mcp.InsertMemory(userID, "도구 계약 검증용 기억입니다. 고유 표식은 DKST_MEMORY_SENTINEL 입니다.")
	if err != nil {
		t.Fatal(err)
	}
	if got := callToolForContract(t, execCtx, "search_memory", `{"query":"DKST_MEMORY_SENTINEL"}`); !strings.Contains(got, fmt.Sprintf("MEMORY ID: %d", memoryID)) {
		t.Fatalf("search_memory did not return inserted memory:\n%s", got)
	}
	readArgs := fmt.Sprintf(`{"memory_id":%d}`, memoryID)
	if got := callToolForContract(t, execCtx, "read_memory", readArgs); !strings.Contains(got, "DKST_MEMORY_SENTINEL") {
		t.Fatalf("read_memory returned wrong content:\n%s", got)
	}
	contextArgs := fmt.Sprintf(`{"memory_id":%d,"chunk_index":0}`, memoryID)
	if got := callToolForContract(t, execCtx, "read_memory_context", contextArgs); !strings.Contains(got, "DKST_MEMORY_SENTINEL") {
		t.Fatalf("read_memory_context returned wrong content:\n%s", got)
	}

	const factKey = "codex_tool_contract_fact"
	callToolForContract(t, execCtx, "save_user_fact", `{"fact_key":"codex_tool_contract_fact","fact_value":"verified","category":"general"}`)
	facts, err := mcp.GetUserProfileFacts(userID)
	if err != nil {
		t.Fatal(err)
	}
	foundFact := false
	for _, fact := range facts {
		if fact.FactKey == factKey && fact.FactValue == "verified" {
			foundFact = true
			break
		}
	}
	if !foundFact {
		t.Fatalf("save_user_fact did not persist %q: %#v", factKey, facts)
	}
	callToolForContract(t, execCtx, "delete_user_fact", `{"fact_key":"codex_tool_contract_fact"}`)

	if got := callToolForContract(t, execCtx, "execute_command", `{"command":"printf DKST_COMMAND_SENTINEL"}`); got != "DKST_COMMAND_SENTINEL" {
		t.Fatalf("execute_command returned %q", got)
	}

	callToolForContract(t, execCtx, "delete_memory", readArgs)
	if _, err := Default.Call(context.Background(), execCtx, "read_memory", json.RawMessage(readArgs)); err == nil {
		t.Fatal("deleted memory was still readable")
	}
}

func TestDefaultRegistryExposesExpectedAppTools(t *testing.T) {
	expected := map[string]bool{
		"search_web": true, "search_web_multi": true, "read_web_page": true,
		"read_buffered_source": true, "read_help": true, "get_current_time": true,
		"search_memory": true, "read_memory": true, "read_memory_context": true,
		"delete_memory": true, "save_user_fact": true, "delete_user_fact": true,
		"naver_search": true, "namu_wiki": true, "get_current_location": true,
		"execute_command": true,
	}
	for _, definition := range Default.List(ExecutionContext{EnableMemory: true}) {
		if !expected[definition.Name] {
			t.Fatalf("unexpected app tool exposed: %s", definition.Name)
		}
		delete(expected, definition.Name)
	}
	if len(expected) != 0 {
		t.Fatalf("expected app tools were not exposed: %#v", expected)
	}
}
