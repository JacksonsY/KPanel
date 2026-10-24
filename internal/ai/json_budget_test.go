package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderJSONBudgetCountsEscapesBeforeMarshal(t *testing.T) {
	for _, text := range []string{"plain", "\"\\\n\t\x00<>&\u2028\u2029", "中文😀"} {
		payload := map[string]any{"text": text, "array": []string{text}, "native": json.RawMessage(`{"text":"<>&"}`)}
		want, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		got, err := marshalProviderJSON(payload, len(want))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("exact escape reservation changed output: err=%v", err)
		}
		if _, err := marshalProviderJSON(payload, len(want)-1); !errors.Is(err, errProviderJSONBudget) {
			t.Fatalf("off-by-one budget: %v", err)
		}
	}
	if _, err := marshalProviderJSON(string([]byte{255}), 1024); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	for _, number := range []any{float64(-0.0000010000000000000002), float64(1e-9), float64(1e21), math.MaxFloat64, math.SmallestNonzeroFloat64, float32(1e-6), float32(1e21)} {
		wire, err := json.Marshal(number)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := marshalProviderJSON(number, len(wire)); err != nil {
			t.Fatalf("float bound mismatch for %s: %v", wire, err)
		}
		if _, err := marshalProviderJSON(number, len(wire)-1); err == nil {
			t.Fatalf("float escaped wire budget for %s", wire)
		}
	}
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	if _, err := marshalProviderJSON(cyclic, 1024); err == nil {
		t.Fatal("cyclic payload accepted")
	}
}

func TestProviderNativeBudgetAllProtocolsKeepsRequiredContext(t *testing.T) {
	for _, size := range []int{1024, 200 << 10} {
		large := strings.Repeat("<", size)
		calls := []ToolCall{{ID: "call", Name: "host_read", Arguments: json.RawMessage(`{}`)}}
		cases := []struct {
			name   string
			attach func() error
		}{
			{"openai-chat", func() error { return attachOpenAIChatReasoning("provider", "model", calls, "reasoning_content", large) }},
			{"responses", func() error {
				raw, _ := json.Marshal(map[string]any{"type": "reasoning", "encrypted_content": large})
				event := responsesCompletionEvent("provider", "model", map[int]*ToolCall{0: &calls[0]}, []json.RawMessage{raw}, false, Usage{})
				if event.err == nil {
					calls = event.ToolCalls
				}
				return event.err
			}},
			{"anthropic", func() error {
				return attachAnthropicContent("provider", "model", calls, map[int]*anthropicContentBlock{0: {Type: "thinking", Thinking: large, Signature: "signature"}}, false)
			}},
			{"gemini", func() error { return attachGeminiThoughtSignature("provider", "model", &calls[0], large) }},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s-%d", tc.name, size), func(t *testing.T) {
				calls[0].ProviderData = nil
				err := tc.attach()
				if size == 1024 {
					if err != nil || len(calls[0].ProviderData) == 0 {
						t.Fatalf("valid native context lost: %v", err)
					}
				} else if !errors.Is(err, errProviderJSONBudget) || len(calls[0].ProviderData) != 0 {
					t.Fatalf("encoded native expansion not rejected: bytes=%d err=%v", len(calls[0].ProviderData), err)
				}
			})
		}
	}
}

func TestProviderRequestBudgetPreservesEightMiBImageAllProtocols(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol ProviderProtocol
		mode     OpenAIAPIMode
		response string
	}{
		{"chat", ProtocolOpenAICompatible, OpenAIChatCompletions, "data: [DONE]\n\n"},
		{"responses", ProtocolOpenAICompatible, OpenAIResponses, "data: [DONE]\n\n"},
		{"anthropic", ProtocolAnthropic, "", "data: {\"type\":\"message_stop\"}\n\n"},
		{"gemini", ProtocolGemini, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var received int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				received, err = io.Copy(io.Discard, r.Body)
				if err != nil {
					t.Error(err)
				}
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			request := CompletionRequest{Model: "test", System: "system", NativeReasoning: true, ThinkingLevel: ThinkingMedium, Messages: []ChatMessage{{Role: "user", Content: "inspect", Attachments: []Attachment{{Name: "image.png", MimeType: "image/png", Kind: "image", Data: bytes.Repeat([]byte{'x'}, 8<<20)}}}}, Tools: []ToolDefinition{{Name: "host_read", Description: "read", Schema: json.RawMessage(`{"type":"object"}`)}}}
			provider := Provider{Protocol: tc.protocol, APIMode: tc.mode, BaseURL: server.URL, EndpointScope: EndpointPrivate}
			if err := NewHTTPModelClient().Stream(context.Background(), provider, "synthetic", request, func(CompletionEvent) error { return nil }); err != nil {
				t.Fatal(err)
			}
			if received < 10<<20 || received > maxProviderRequestBytes {
				t.Fatalf("8 MiB image wire bytes=%d", received)
			}
		})
	}
}

func TestProviderNativeEncodedExpansionFailsClosedOverHTTP(t *testing.T) {
	large, _ := json.Marshal(strings.Repeat("<", 200<<10))
	for _, tc := range []struct {
		name     string
		protocol ProviderProtocol
		mode     OpenAIAPIMode
		wire     string
	}{
		{"chat", ProtocolOpenAICompatible, OpenAIChatCompletions, `data: {"choices":[{"delta":{"reasoning_content":` + string(large) + `,"tool_calls":[{"index":0,"id":"call","function":{"name":"host_read","arguments":"{}"}}]}}]}` + "\n\ndata: [DONE]\n\n"},
		{"responses", ProtocolOpenAICompatible, OpenAIResponses, `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","encrypted_content":` + string(large) + `}}` + "\n\n"},
		{"anthropic", ProtocolAnthropic, "", `data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":` + string(large) + `,"signature":"signed"}}` + "\n\n" + `data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call","name":"host_read","input":{}}}` + "\n\ndata: {\"type\":\"message_stop\"}\n\n"},
		{"gemini", ProtocolGemini, "", `data: {"candidates":[{"content":{"parts":[{"thoughtSignature":` + string(large) + `,"functionCall":{"id":"call","name":"host_read","args":{}}}]}}]}` + "\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.wire) }))
			defer server.Close()
			provider := Provider{ID: "provider", Protocol: tc.protocol, APIMode: tc.mode, BaseURL: server.URL, EndpointScope: EndpointPrivate}
			emittedCalls := 0
			err := NewHTTPModelClient().Stream(context.Background(), provider, "synthetic", CompletionRequest{Model: "model"}, func(event CompletionEvent) error { emittedCalls += len(event.ToolCalls); return nil })
			if err == nil || emittedCalls != 0 {
				t.Fatalf("oversized encoded native state exposed: calls=%d err=%v", emittedCalls, err)
			}
		})
	}
}

func TestToolStateNewWritesFitEncodedReadBudget(t *testing.T) {
	s, session, run := contextFixture(t)
	call := ToolCall{ID: "call", RunID: run.ID, SessionID: session.ID, Name: "host_read", Arguments: json.RawMessage(`{}`), ProviderData: json.RawMessage(`{"text":"` + strings.Repeat("<", 200<<10) + `"}`), Status: ToolCompleted}
	if _, err := s.SaveToolCall(context.Background(), call); !errors.Is(err, errProviderJSONBudget) {
		t.Fatalf("new state with oversized native JSON expansion accepted: %v", err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM tool_calls WHERE id=?`, call.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed budget wrote state: %d %v", count, err)
	}
}

func TestToolStateTruncatedPreviewsRemainReadable(t *testing.T) {
	s, session, run := contextFixture(t)
	call, err := s.SaveToolCall(context.Background(), ToolCall{ID: "long-preview", RunID: run.ID, SessionID: session.ID, Name: "host_read", Arguments: json.RawMessage(`{"text":"` + strings.Repeat("x", 5000) + `"}`), ResultPreview: strings.Repeat("x", MaxToolResultBytes+100), Status: ToolCompleted})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(call.ArgumentsPreview, "[TRUNCATED]") || !strings.HasSuffix(call.ResultPreview, "[TRUNCATED]") {
		t.Fatal("fixture did not exercise complete truncation markers")
	}
	if _, err := s.ToolCall(context.Background(), run.ID, call.ID); err != nil {
		t.Fatal(err)
	}
	if calls, err := s.ToolCalls(context.Background(), run.ID); err != nil || len(calls) != 1 {
		t.Fatalf("valid bounded previews rejected: %d %v", len(calls), err)
	}
}

func TestToolStatePreservesHTMLArgumentsWithinReadBudget(t *testing.T) {
	s, session, run := contextFixture(t)
	args := json.RawMessage(`{"text":"` + strings.Repeat("<", 60000) + `"}`)
	call, err := s.SaveToolCall(context.Background(), ToolCall{ID: "html-args", RunID: run.ID, SessionID: session.ID, Name: "host_read", Arguments: args, ProviderData: json.RawMessage(`{"text":"` + strings.Repeat("x", 1000<<10) + `"}`), Status: ToolPendingApproval})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.ToolCall(context.Background(), run.ID, call.ID)
	if err != nil || !bytes.Equal(stored.Arguments, args) {
		t.Fatalf("valid arguments changed across encryption: bytes=%d err=%v", len(stored.Arguments), err)
	}
	stored.Status = ToolCompleted
	if _, err := s.SaveToolCall(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
}
