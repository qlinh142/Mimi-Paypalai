package gemini

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"google.golang.org/genai"

	"github.com/smartlife-lab/mimi/backend/internal/assistant"
)

func TestToGenaiToolsAndSignatures(t *testing.T) {
	sig := base64.StdEncoding.EncodeToString([]byte("real-sig"))
	req := assistant.Request{
		System: "You are Mimi.",
		Contents: []assistant.Content{
			{Role: "user", Parts: []assistant.Part{{Text: "Tuần này cần mua gì?"}}},
			{Role: "model", Parts: []assistant.Part{
				{FunctionCall: &assistant.FunctionCall{ID: "c1", Name: "mimi_running_out", Args: map[string]any{"days": 7.0}}, ThoughtSignature: sig},
				{FunctionCall: &assistant.FunctionCall{ID: "c2", Name: "mimi_savings"}},
			}},
			{Role: "user", Parts: []assistant.Part{{FunctionResponse: &assistant.FunctionResponse{ID: "c1", Name: "mimi_running_out", Response: map[string]any{"result": "[]"}}}}},
			// Hội thoại cũ không còn chữ ký: lượt gọi tool đầu tiên nhận chữ ký giả.
			{Role: "model", Parts: []assistant.Part{{FunctionCall: &assistant.FunctionCall{ID: "c3", Name: "mimi_savings"}}}},
		},
		Tools:    []assistant.Tool{{Name: "mimi_running_out", Description: "d", Parameters: json.RawMessage(`{"type":"object","properties":{"days":{"type":"number"}}}`)}},
		ToolMode: "ANY", AllowedTools: []string{"mimi_running_out"},
	}
	contents, cfg, err := toGenai(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 4 || cfg.SystemInstruction.Parts[0].Text != "You are Mimi." {
		t.Fatalf("%+v", contents)
	}
	if string(contents[1].Parts[0].ThoughtSignature) != "real-sig" || contents[1].Parts[1].ThoughtSignature != nil {
		t.Fatalf("signatures: %q %q", contents[1].Parts[0].ThoughtSignature, contents[1].Parts[1].ThoughtSignature)
	}
	if string(contents[3].Parts[0].ThoughtSignature) != skipSignature {
		t.Fatalf("fallback signature: %q", contents[3].Parts[0].ThoughtSignature)
	}
	if contents[2].Parts[0].FunctionResponse.Name != "mimi_running_out" {
		t.Fatal("function response lost")
	}
	fd := cfg.Tools[0].FunctionDeclarations[0]
	if fd.Name != "mimi_running_out" || fd.ParametersJsonSchema.(map[string]any)["type"] != "object" {
		t.Fatalf("%+v", fd)
	}
	fc := cfg.ToolConfig.FunctionCallingConfig
	if fc.Mode != genai.FunctionCallingConfigModeAny || fc.AllowedFunctionNames[0] != "mimi_running_out" {
		t.Fatalf("%+v", fc)
	}
}

func TestToGenaiJSONResponse(t *testing.T) {
	_, cfg, err := toGenai(assistant.Request{
		Contents:   []assistant.Content{{Role: "user", Parts: []assistant.Part{{Text: "plan"}}}},
		JSONSchema: json.RawMessage(`{"type":"object"}`),
	})
	if err != nil || cfg.ResponseMIMEType != "application/json" || cfg.ResponseJsonSchema == nil || cfg.Tools != nil {
		t.Fatalf("%+v %v", cfg, err)
	}
}

func TestFromGenaiSkipsThoughtsKeepsSignature(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		ResponseID: "r1", ModelVersion: "gemini-3-flash",
		Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonStop, Content: &genai.Content{Parts: []*genai.Part{
			{Text: "đang nghĩ", Thought: true},
			{Text: "Mẹ cần mua sữa."},
			{FunctionCall: &genai.FunctionCall{ID: "c1", Name: "mimi_show_page", Args: map[string]any{"page": "prices"}}, ThoughtSignature: []byte("s")},
		}}}},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 10, CandidatesTokenCount: 5, ThoughtsTokenCount: 2, TotalTokenCount: 17},
	}
	out := fromGenai(resp)
	if len(out.Parts) != 2 || out.Parts[0].Text != "Mẹ cần mua sữa." || out.Parts[1].FunctionCall.Name != "mimi_show_page" {
		t.Fatalf("%+v", out.Parts)
	}
	if out.Parts[1].ThoughtSignature != base64.StdEncoding.EncodeToString([]byte("s")) || out.FinishReason != "STOP" || out.Usage.OutputTokens != 7 {
		t.Fatalf("%+v", out)
	}
}
