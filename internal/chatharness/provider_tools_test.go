package chatharness

import "testing"

func TestChatToolAccumulatorReassemblesStreamingArguments(t *testing.T) {
	acc := NewChatToolAccumulator()
	acc.AddChunk(map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{
			"delta": map[string]interface{}{
				"tool_calls": []interface{}{map[string]interface{}{
					"index": float64(0),
					"id":    "call_123",
					"function": map[string]interface{}{
						"name":      "search_web",
						"arguments": `{"query":"current`,
					},
				}},
			},
		}},
	})
	acc.AddChunk(map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{
			"delta": map[string]interface{}{
				"tool_calls": []interface{}{map[string]interface{}{
					"index": float64(0),
					"function": map[string]interface{}{
						"arguments": ` news"}`,
					},
				}},
			},
		}},
	})

	calls := acc.Calls()
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	if calls[0].ID != "call_123" || calls[0].Name != "search_web" {
		t.Fatalf("unexpected call identity: %#v", calls[0])
	}
	if calls[0].Arguments != `{"query":"current news"}` {
		t.Fatalf("arguments were not reassembled: %q", calls[0].Arguments)
	}
}

func TestChatToolAccumulatorReadsNonStreamingMessageToolCall(t *testing.T) {
	acc := NewChatToolAccumulator()
	if !acc.AddChunk(map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{
			"message": map[string]interface{}{
				"tool_calls": []interface{}{map[string]interface{}{
					"index": float64(0),
					"id":    "call_message",
					"function": map[string]interface{}{
						"name":      "get_current_time",
						"arguments": map[string]interface{}{},
					},
				}},
			},
		}},
	}) {
		t.Fatal("message.tool_calls was not detected")
	}
	calls := acc.Calls()
	if len(calls) != 1 || calls[0].ID != "call_message" || calls[0].Name != "get_current_time" || calls[0].Arguments != `{}` {
		t.Fatalf("unexpected non-streaming call: %#v", calls)
	}
}

func TestChatToolAccumulatorReadsLegacyFunctionCall(t *testing.T) {
	acc := NewChatToolAccumulator()
	if !acc.AddChunk(map[string]interface{}{
		"choices": []interface{}{map[string]interface{}{
			"delta": map[string]interface{}{
				"function_call": map[string]interface{}{
					"name":      "read_help",
					"arguments": `{"query":"memory"}`,
				},
			},
		}},
	}) {
		t.Fatal("legacy function_call was not detected")
	}
	calls := acc.Calls()
	if len(calls) != 1 || calls[0].Name != "read_help" || calls[0].Arguments != `{"query":"memory"}` {
		t.Fatalf("unexpected legacy call: %#v", calls)
	}
}

func TestParseProviderToolArgumentsEventUsesPriorNameAndRawJSONObject(t *testing.T) {
	call, parsed, ok := ParseProviderToolArgumentsEvent(map[string]interface{}{
		"type":      "tool_call.arguments",
		"call_id":   "call_stateful",
		"arguments": `{"question":"웹 검색"}`,
	}, "read_help")
	if !ok || call.ID != "call_stateful" || call.Name != "read_help" || call.Arguments != `{"question":"웹 검색"}` {
		t.Fatalf("unexpected stateful call: %#v, ok=%v", call, ok)
	}
	args, _ := parsed.(map[string]interface{})
	if args["question"] != "웹 검색" {
		t.Fatalf("arguments were not decoded: %#v", parsed)
	}
}

func TestParseProviderToolArgumentsEventMarshalsArgumentObject(t *testing.T) {
	call, parsed, ok := ParseProviderToolArgumentsEvent(map[string]interface{}{
		"tool_name": "get_current_time",
		"arguments": map[string]interface{}{},
	}, "")
	if !ok || call.Name != "get_current_time" || call.Arguments != `{}` {
		t.Fatalf("unexpected object arguments: %#v, ok=%v", call, ok)
	}
	if _, ok := parsed.(map[string]interface{}); !ok {
		t.Fatalf("parsed arguments have wrong type: %#v", parsed)
	}
}
