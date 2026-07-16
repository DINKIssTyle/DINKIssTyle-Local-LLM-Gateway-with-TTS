package chatharness

import "testing"

func TestIsValidResponseID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"resp_abc123", true},
		{"  resp_abc123  ", true},
		{"", false},
		{"resp_", false},
		{"call_abc123", false},
		{"invalid", false},
	}

	for _, test := range tests {
		if got := IsValidResponseID(test.id); got != test.want {
			t.Errorf("IsValidResponseID(%q) = %v, want %v", test.id, got, test.want)
		}
	}
}

func TestPrepareToolFollowupRequestOmitsInvalidResponseID(t *testing.T) {
	req, _, err := PrepareToolFollowupRequest(ToolFollowupInput{
		LLMMode:        "stateful",
		ModelID:        "test-model",
		LastResponseID: "call_wrong_kind",
		ToolName:       "test_tool",
		ToolResult:     "ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := req["previous_response_id"]; exists {
		t.Fatal("invalid previous_response_id was forwarded")
	}
}
