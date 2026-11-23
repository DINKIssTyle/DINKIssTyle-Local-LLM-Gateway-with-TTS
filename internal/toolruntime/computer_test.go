package toolruntime

import (
	"context"
	"dinkisstyle-chat/internal/computeruse"
	"encoding/json"
	"strings"
	"testing"
)

type testComputerBackend struct{ calls int }

func (*testComputerBackend) Available() bool { return true }
func (b *testComputerBackend) Observe(context.Context) (computeruse.Screen, []byte, error) {
	b.calls++
	return computeruse.Screen{Width: 100, Height: 100, DesktopWidth: 200, DesktopHeight: 200}, []byte("png"), nil
}
func (b *testComputerBackend) Execute(context.Context, string, computeruse.Request) (json.RawMessage, error) {
	b.calls++
	return json.RawMessage(`{"status":"ok"}`), nil
}

func TestComputerToolsRespectPolicyAndReturnImagesSeparately(t *testing.T) {
	b := &testComputerBackend{}
	r := NewRegistry()
	registerComputerTools(r, computeruse.New(b))
	ctx := ExecutionContext{UserID: "user", RequestID: "chat", DisabledTools: []string{"computer_observe"}}
	for _, d := range r.List(ctx) {
		if d.Name == "computer_observe" {
			t.Fatal("disabled capture exposed")
		}
	}
	if _, err := r.Call(context.Background(), ctx, "computer_observe", json.RawMessage(`{}`)); err == nil || b.calls != 0 {
		t.Fatal("disabled capture executed")
	}
	ctx.DisabledTools = nil
	out, err := r.Call(context.Background(), ctx, "computer_observe", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.ImageURLs) != 1 || !strings.HasPrefix(out.ImageURLs[0], "data:image/png;base64,") || strings.Contains(out.Content, "base64") {
		t.Fatalf("invalid screenshot transport: %+v", out)
	}
	if _, err := r.Call(context.Background(), ctx, "computer_click", json.RawMessage(`{"x":1,"y":2}`)); err == nil || b.calls != 1 {
		t.Fatal("input without observation ID executed")
	}
}

func TestDesktopStateAllowsRepeatedCallsWithoutChangingWebPolicy(t *testing.T) {
	for _, name := range ComputerToolNames {
		if !AllowsRepeatedComputerCall(name) {
			t.Fatalf("incorrect repeat policy: %s", name)
		}
	}
	if AllowsRepeatedComputerCall("search_web") || AllowsRepeatedComputerCall("execute_command") {
		t.Fatal("desktop exception changed other tool policies")
	}
}
