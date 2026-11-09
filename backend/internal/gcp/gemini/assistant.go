package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"

	"github.com/smartlife-lab/mimi/backend/internal/assistant"
)

// Assistant chạy một lượt "Hỏi Mimi" (assistant.Model) trên Gemini, Vertex AI, bằng tài khoản
// dịch vụ của Cloud Run như Reader. Không bật Google Search: câu trả lời chỉ dựa trên dữ liệu
// mà tool của app trả về.
type Assistant struct {
	client   *genai.Client
	model    string
	thinking genai.ThinkingLevel
	maxOut   int32
}

var _ assistant.Model = (*Assistant)(nil)

// NewAssistant dùng chung client với Reader (cùng project, vùng, model).
func (r *Reader) NewAssistant(model string) *Assistant {
	if model == "" {
		model = r.model
	}
	return &Assistant{client: r.client, model: model, thinking: genai.ThinkingLevelLow, maxOut: 4096}
}

// skipSignature: chữ ký giả Gemini chấp nhận khi lượt gọi tool cũ không còn chữ ký thật
// (ví dụ hội thoại mở lại sau khi tải lại trang).
const skipSignature = "skip_thought_signature_validator"

func toGenai(req assistant.Request) ([]*genai.Content, *genai.GenerateContentConfig, error) {
	contents := make([]*genai.Content, 0, len(req.Contents))
	for _, c := range req.Contents {
		gc := &genai.Content{Role: c.Role}
		firstCall := true
		for _, p := range c.Parts {
			gp := &genai.Part{}
			switch {
			case p.FunctionCall != nil:
				gp.FunctionCall = &genai.FunctionCall{ID: p.FunctionCall.ID, Name: p.FunctionCall.Name, Args: p.FunctionCall.Args}
				sig := p.ThoughtSignature
				if sig == "" && firstCall {
					sig = base64.StdEncoding.EncodeToString([]byte(skipSignature))
				}
				firstCall = false
				if sig != "" {
					b, err := base64.StdEncoding.DecodeString(sig)
					if err != nil {
						return nil, nil, fmt.Errorf("thought signature: %w", err)
					}
					gp.ThoughtSignature = b
				}
			case p.FunctionResponse != nil:
				gp.FunctionResponse = &genai.FunctionResponse{ID: p.FunctionResponse.ID, Name: p.FunctionResponse.Name, Response: p.FunctionResponse.Response}
			default:
				gp.Text = p.Text
			}
			gc.Parts = append(gc.Parts, gp)
		}
		contents = append(contents, gc)
	}

	cfg := &genai.GenerateContentConfig{MaxOutputTokens: 0}
	if req.System != "" {
		cfg.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: req.System}}}
	}
	if len(req.Tools) > 0 {
		decls := make([]*genai.FunctionDeclaration, 0, len(req.Tools))
		for _, t := range req.Tools {
			d := &genai.FunctionDeclaration{Name: t.Name, Description: t.Description}
			if len(t.Parameters) > 0 {
				var schema any
				if err := json.Unmarshal(t.Parameters, &schema); err != nil {
					return nil, nil, fmt.Errorf("tool %s schema: %w", t.Name, err)
				}
				d.ParametersJsonSchema = schema
			}
			decls = append(decls, d)
		}
		cfg.Tools = []*genai.Tool{{FunctionDeclarations: decls}}
		mode := genai.FunctionCallingConfigModeAuto
		switch req.ToolMode {
		case "ANY":
			mode = genai.FunctionCallingConfigModeAny
		case "NONE":
			mode = genai.FunctionCallingConfigModeNone
		}
		cfg.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: mode, AllowedFunctionNames: req.AllowedTools}}
	}
	if len(req.JSONSchema) > 0 {
		var schema any
		if err := json.Unmarshal(req.JSONSchema, &schema); err != nil {
			return nil, nil, fmt.Errorf("json schema: %w", err)
		}
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseJsonSchema = schema
	}
	return contents, cfg, nil
}

func fromGenai(resp *genai.GenerateContentResponse) assistant.Response {
	out := assistant.Response{ID: resp.ResponseID, Model: resp.ModelVersion, Parts: []assistant.Part{}}
	if len(resp.Candidates) > 0 && resp.Candidates[0] != nil {
		c := resp.Candidates[0]
		out.FinishReason = string(c.FinishReason)
		if c.Content != nil {
			for _, p := range c.Content.Parts {
				if p == nil || p.Thought {
					continue // phần "suy nghĩ" không gửi về app
				}
				ap := assistant.Part{}
				if len(p.ThoughtSignature) > 0 {
					ap.ThoughtSignature = base64.StdEncoding.EncodeToString(p.ThoughtSignature)
				}
				switch {
				case p.FunctionCall != nil:
					ap.FunctionCall = &assistant.FunctionCall{ID: p.FunctionCall.ID, Name: p.FunctionCall.Name, Args: p.FunctionCall.Args}
				case p.Text != "":
					ap.Text = p.Text
				default:
					continue
				}
				out.Parts = append(out.Parts, ap)
			}
		}
	}
	if u := resp.UsageMetadata; u != nil {
		out.Usage = &assistant.Usage{InputTokens: int(u.PromptTokenCount), OutputTokens: int(u.CandidatesTokenCount + u.ThoughtsTokenCount), TotalTokens: int(u.TotalTokenCount)}
	}
	return out
}

func (a *Assistant) Turn(ctx context.Context, req assistant.Request) (assistant.Response, error) {
	contents, cfg, err := toGenai(req)
	if err != nil {
		return assistant.Response{}, err
	}
	cfg.MaxOutputTokens = a.maxOut
	cfg.ThinkingConfig = &genai.ThinkingConfig{ThinkingLevel: a.thinking}
	resp, err := a.client.Models.GenerateContent(ctx, a.model, contents, cfg)
	if err != nil {
		return assistant.Response{}, fmt.Errorf("gemini assistant: %w", err)
	}
	return fromGenai(resp), nil
}
