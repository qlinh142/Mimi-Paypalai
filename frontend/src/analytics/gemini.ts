/**
 * Adapter LLM cho AG Studio chạy bằng Gemini qua máy chủ Mimi (POST /api/ai/turn).
 * - Trình duyệt không giữ khóa nào: máy chủ gọi Vertex AI bằng tài khoản dịch vụ của Cloud Run,
 *   như khi đọc hóa đơn, và chỉ cho người đã đăng nhập dùng (có giới hạn lượt).
 * - Một lượt = một lần gọi generateContent (không stream). Adapter tự phát các sự kiện
 *   TEXT_MESSAGE_* / TOOL_CALL_* mà Studio đọc, từ câu trả lời trọn vẹn.
 * Dạng request/response bám theo REST của Gemini (contents, parts, functionCall...), nên máy chủ
 * chỉ cần chuyển tiếp, không phải dịch.
 */
import type {
  AgAiConversationItem, AgAiEvent, AgAiOutputItem, AgAiToolSchema, AgLlmAdapter, AgLlmRequest, AgLlmResponse, AgLlmResponseHandler,
} from "ag-studio";
import { apiSend } from "../api/client";

type Json = Record<string, unknown>;
type Part = { text?: string; thoughtSignature?: string; functionCall?: { id?: string; name: string; args?: Json }; functionResponse?: { id?: string; name: string; response: Json } };
type Content = { role: "user" | "model"; parts: Part[] };

export type AiTurnRequest = {
  system?: string;
  contents: Content[];
  tools?: { name: string; description: string; parameters: Json }[];
  tool_mode?: "AUTO" | "ANY" | "NONE";
  allowed_tools?: string[];
  json_schema?: Json;
  model?: string;
};
export type AiTurnResponse = {
  id: string;
  model?: string;
  parts: Part[];
  finish_reason?: string;
  usage?: { input_tokens?: number; output_tokens?: number; total_tokens?: number };
};

// ---------------- JSON Schema của Studio → tập con Gemini nhận ----------------

const isObj = (v: unknown): v is Json => typeof v === "object" && v !== null && !Array.isArray(v);
/** Studio đánh dấu "có thể vắng" bằng nhánh { not: {} } trong anyOf; Gemini không nhận `not`. */
const isUndefinedSentinel = (s: unknown) => isObj(s) && Object.keys(s).length === 1 && isObj(s.not) && Object.keys(s.not).length === 0;

export function toGeminiSchema(schema: unknown): unknown {
  if (Array.isArray(schema)) return schema.map(toGeminiSchema);
  if (!isObj(schema)) return schema;
  let s: Json = { ...schema };
  if (Array.isArray(s.anyOf)) {
    const branches = (s.anyOf as unknown[]).filter((b) => !isUndefinedSentinel(b));
    const { anyOf: _drop, ...rest } = s;
    s = branches.length === 1 && isObj(branches[0]) ? { ...(branches[0] as Json), ...rest } : { ...rest, anyOf: branches };
  }
  if ("const" in s) {
    const { const: c, ...rest } = s;
    s = { ...rest, enum: [c] };
    if (s.type == null && typeof c === "string") s.type = "string";
  }
  delete s.not;
  delete s.$schema;
  const out: Json = {};
  for (const [k, v] of Object.entries(s)) {
    if (k === "properties" && isObj(v)) out[k] = Object.fromEntries(Object.entries(v).map(([pk, pv]) => [pk, toGeminiSchema(pv)]));
    else if (k === "$defs" && isObj(v)) out[k] = Object.fromEntries(Object.entries(v).map(([dk, dv]) => [dk, toGeminiSchema(dv)]));
    else if (k === "items" || k === "additionalProperties" || k === "anyOf" || k === "oneOf" || k === "prefixItems") out[k] = toGeminiSchema(v);
    else out[k] = v;
  }
  return out;
}

// ---------------- Hội thoại Studio → contents của Gemini ----------------

/**
 * Gemini 3 gắn chữ ký (thoughtSignature) vào lượt gọi tool và đòi gửi lại ở lượt sau. Hội thoại
 * của Studio không có chỗ cho nó, nên giữ theo callId trong bộ nhớ của trang (cùng vòng đời với
 * hội thoại, vốn cũng chỉ nằm trong bộ nhớ). Mất chữ ký thì máy chủ dùng chữ ký thay thế.
 */
const signatures = new Map<string, string>();

const parseArgs = (s: string): Json => {
  try {
    const v = JSON.parse(s || "{}");
    return isObj(v) ? v : { value: v };
  } catch {
    return {};
  }
};

export function toContents(items: AgAiConversationItem[]): Content[] {
  const nameByCall = new Map<string, string>();
  const out: Content[] = [];
  const push = (role: Content["role"], part: Part) => {
    const last = out.at(-1);
    if (last && last.role === role) last.parts.push(part);
    else out.push({ role, parts: [part] });
  };
  for (const it of items) {
    if (it.type === "message" && it.kind === "input") {
      const text = it.content.map((c) => (c.type === "text" ? c.text : "")).filter(Boolean).join("\n");
      // Tin "system" giữa hội thoại (Studio gửi ngữ cảnh) đi như lời người dùng có đánh dấu.
      if (text) push("user", { text: it.role === "system" ? `[context]\n${text}` : text });
    } else if (it.type === "message" && it.kind === "output") {
      const text = it.content.map((c) => (c.type === "text" ? c.text : c.refusal)).join("");
      if (text) push("model", { text });
    } else if (it.type === "function_call") {
      nameByCall.set(it.callId, it.name);
      const sig = signatures.get(it.callId);
      push("model", { functionCall: { id: it.callId, name: it.name, args: parseArgs(it.arguments) }, ...(sig && { thoughtSignature: sig }) });
    } else if (it.type === "function_call_output") {
      const name = nameByCall.get(it.callId) ?? "tool";
      let response: Json;
      try {
        const v = JSON.parse(it.output);
        response = isObj(v) ? v : { result: v };
      } catch {
        response = { result: it.output };
      }
      push("user", { functionResponse: { id: it.callId, name, response } });
    }
    // reasoning: Gemini tự giữ, không gửi lại.
  }
  return out;
}

export function toTurnRequest(req: AgLlmRequest): AiTurnRequest {
  const tools = (req.tools ?? []).filter((t: AgAiToolSchema) => (t.kind ?? "client") !== "provided");
  const choice = req.toolChoice;
  return {
    system: req.instructions,
    contents: toContents(req.input),
    ...(tools.length > 0 && {
      tools: tools.map((t) => ({ name: t.name, description: t.description, parameters: toGeminiSchema(t.parameters) as Json })),
      tool_mode: choice === "none" ? "NONE" : choice === "required" || (choice && typeof choice === "object") ? "ANY" : "AUTO",
      ...(choice && typeof choice === "object" && { allowed_tools: [choice.name] }),
    }),
    ...(req.responseFormat.type === "json" && { json_schema: toGeminiSchema(req.responseFormat.schema) as Json }),
    ...(req.model?.id && { model: req.model.id }),
  };
}

// ---------------- Câu trả lời → sự kiện và kết quả của Studio ----------------

let seq = 0;
const newId = (p: string) => `${p}_${Date.now().toString(36)}_${(seq++).toString(36)}`;

export function fromTurnResponse(res: AiTurnResponse): { events: AgAiEvent[]; response: AgLlmResponse } {
  const events: AgAiEvent[] = [];
  const output: AgAiOutputItem[] = [];
  const text = res.parts.map((p) => p.text ?? "").join("");
  if (text) {
    const messageId = newId("msg");
    events.push({ type: "TEXT_MESSAGE_START", messageId, role: "assistant" }, { type: "TEXT_MESSAGE_CONTENT", messageId, delta: text }, { type: "TEXT_MESSAGE_END", messageId });
    output.push({ id: messageId, kind: "output", type: "message", role: "assistant", status: "completed", content: [{ type: "text", text, annotations: [] }] });
  }
  for (const p of res.parts) {
    if (!p.functionCall) continue;
    const callId = p.functionCall.id || newId("call");
    if (p.thoughtSignature) signatures.set(callId, p.thoughtSignature);
    const args = JSON.stringify(p.functionCall.args ?? {});
    events.push(
      { type: "TOOL_CALL_START", toolCallId: callId, toolCallName: p.functionCall.name },
      { type: "TOOL_CALL_ARGS", toolCallId: callId, delta: args },
      { type: "TOOL_CALL_END", toolCallId: callId },
    );
    output.push({ id: callId, kind: "output", type: "function_call", callId, name: p.functionCall.name, arguments: args, status: "completed" });
  }
  return {
    events,
    response: {
      id: res.id || newId("resp"),
      createdAt: Date.now(),
      model: res.model,
      output,
      status: res.finish_reason === "MAX_TOKENS" ? "incomplete" : "completed",
      ...(res.finish_reason === "MAX_TOKENS" && { incompleteDetails: { reason: "max_output_tokens" as const } }),
      usage: res.usage
        ? { inputTokens: res.usage.input_tokens ?? 0, outputTokens: res.usage.output_tokens ?? 0, totalTokens: res.usage.total_tokens ?? 0 }
        : undefined,
    } as AgLlmResponse,
  };
}

/** Một lượt hỏi: gửi lên máy chủ, đợi trọn câu trả lời, rồi phát sự kiện cho Studio. */
export function geminiAdapter(send: (body: AiTurnRequest, signal?: AbortSignal) => Promise<AiTurnResponse> = defaultSend): AgLlmAdapter {
  return {
    executeTurn(request, options): AgLlmResponseHandler {
      const result = send(toTurnRequest(request), options?.signal).then(fromTurnResponse);
      return {
        stream: {
          async *[Symbol.asyncIterator]() {
            const { events } = await result;
            yield* events;
          },
        },
        complete: result.then((r) => r.response),
      };
    },
  };
}

const AI_TIMEOUT_MS = 60_000;
const defaultSend = (body: AiTurnRequest) => apiSend<AiTurnResponse>("POST", "/ai/turn", body, undefined, AI_TIMEOUT_MS);
