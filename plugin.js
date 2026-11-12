// Adapted from the Apache-2.0 TypeSafe Task Plugin in QuantumNous/new-api-plugins.
// Google Decisions accepts up to 262,144 input tokens per request. Reserve the
// full context budget because the gateway cannot reliably tokenize all of its
// supported image, audio, video, document, and text inputs before submission.
const MAX_INPUT_TOKENS = 262144;
const MODEL = "gemini-decision-1";
const PLUGIN_KEY = "gemini-decisions";

export const meta = {
  apiVersion: 1,
  key: PLUGIN_KEY,
  name: "Gemini Decisions",
  version: "0.2.0",
  icon: "Gemini.Color",
  author: { name: "huangdijia", url: "https://github.com/huangdijia" },
  website: "https://github.com/huangdijia/new-api-plugin-gemini-decisions",
  description: {
    en: "Typed multimodal decisions with Gemini Decision 1",
    zh: "使用 Gemini Decision 1 进行多模态结构化判断",
  },
  baseUrl: "https://generativelanguage.googleapis.com",
  auth: "api_key",
  models: [MODEL],
  fetchMode: "per_task",
  upstreams: ["vendor", "new_api"],
  routes: [
    {
      method: "POST",
      path: "/v1beta/decisions",
      type: "submit",
      decode: "decodeDecision",
      render: "renderDecision",
      retainResult: false,
    },
  ],
  usageSchema: {
    input_tokens: {
      type: "number",
      unit: "token",
      description: { en: "Input token unit price", zh: "输入 token 单价" },
    },
  },
  usageExamples: [{ label: "Gemini Decision 1", facts: { input_tokens: 1000 } }],
};

function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isTextBlock(value) {
  return isObject(value) && value.type === "text" && typeof value.text === "string";
}

function validateRequest(body) {
  if (!isObject(body)) throw new Error("Request body must be a JSON object");
  if (typeof body.model !== "string" || !body.model.trim()) throw new Error("model is required");
  const input = body.input;
  if (
    (typeof input !== "string" && !isObject(input) && !Array.isArray(input)) ||
    (typeof input === "string" && input.length === 0) ||
    (Array.isArray(input) && input.length === 0)
  ) throw new Error("input must be nonempty text or content blocks");
  if (!isObject(body.questions)) throw new Error("questions must be an object");
  const keys = Object.keys(body.questions);
  if (keys.length < 1 || keys.length > 1000) throw new Error("questions must contain 1 to 1000 entries");
  for (const key of keys) {
    if (!/^[A-Za-z0-9_-]{1,64}$/.test(key))
      throw new Error("question key must contain 1 to 64 ASCII letters, digits, underscores, or hyphens");
    const question = body.questions[key];
    if (!isObject(question) || !["noul", "choice", "score"].includes(question.type))
      throw new Error("question type must be noul, choice, or score");
    const instructions = question.instructions;
    if (
      typeof instructions !== "string" &&
      !isTextBlock(instructions) &&
      !(Array.isArray(instructions) && instructions.length > 0 && instructions.every(isTextBlock))
    ) throw new Error("question instructions must be text or nonempty text content blocks");
    if (question.type === "choice") {
      if (!isObject(question.criteria)) throw new Error("choice criteria must contain 2 to 255 options");
      const options = Object.keys(question.criteria);
      if (options.length < 2 || options.length > 255 || options.some((option) => typeof question.criteria[option] !== "string"))
        throw new Error("choice criteria must contain 2 to 255 string descriptions");
    } else if (question.type === "score") {
      if (
        !Array.isArray(question.criteria) ||
        question.criteria.length < 2 ||
        question.criteria.length > 255 ||
        question.criteria.some((level) => typeof level !== "string" || level.length === 0)
      ) throw new Error("score criteria must contain 2 to 255 nonempty strings");
    }
  }
  return body;
}

function decisionRequest(ctx) {
  const body = validateRequest(ctx.requestBody);
  const model = ctx.upstreamModel || ctx.model || body.model;
  if (model !== MODEL) throw new Error("The upstream model must be gemini-decision-1");
  // Preserve the native JSON request, including future optional fields, while
  // replacing only the channel-mapped model. Object spread creates own data
  // properties even for a client-supplied __proto__ key.
  return { ...body, model: model };
}

export function buildSubmitRequest(ctx) {
  const body = decisionRequest(ctx);
  // The channel may be configured with the service root or its v1beta URL.
  const baseUrl = ctx.baseUrl.replace(/\/+$/, "").replace(/\/v1beta$/, "");
  const viaGateway = Boolean(ctx.upstream && ctx.upstream.kind === "new_api");
  const headers = { "Content-Type": "application/json", Accept: "application/json" };
  if (viaGateway) headers.Authorization = "Bearer " + ctx.apiKey;
  else headers["x-goog-api-key"] = ctx.apiKey;
  return {
    url: baseUrl + (viaGateway ? "/" + PLUGIN_KEY : "") + "/v1beta/decisions",
    method: "POST",
    headers: headers,
    body: body,
  };
}

export function parseSubmitResponse(ctx, response) {
  if (!ctx.publicTaskId) throw new Error("Missing gateway request ID");
  // The host invokes this hook only for 2xx responses. Preserve the body so
  // completion usage settles before the presenter checks the answer envelope.
  return {
    taskId: ctx.publicTaskId,
    taskData: response.body,
    immediate: { status: "SUCCESS", progress: "100%" },
  };
}

export function extractUsage(ctx) {
  decisionRequest(ctx);
  // A legacy per-call price is not an input-token multiplier. The token
  // reservation is used only with the host's task usage expressions.
  if (ctx.usagePurpose === "billing_ratios") return null;
  return { input_tokens: MAX_INPUT_TOKENS };
}

export function extractUsageOnComplete(ctx, result, body) {
  const tokens = body && body.usage && body.usage.total_input_tokens;
  // Invalid or absent usage leaves the reservation in place and is logged by
  // the host; silently replacing it with zero would underbill the request.
  if (!Number.isInteger(tokens) || tokens < 0 || tokens > MAX_INPUT_TOKENS)
    throw new Error("Decision input token usage is missing or outside the context limit");
  return { input_tokens: tokens };
}

// Task Plugin v1 requires these hooks even though Decisions is synchronous.
export function buildQueryRequest() {
  throw new Error("Gemini Decisions has no task retrieval endpoint");
}

export function parseTaskResult() {
  return { status: "UNKNOWN", reason: "Gemini Decisions has no asynchronous tasks" };
}

export const native = {
  decodeDecision: function (ctx) {
    if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
    const body = validateRequest(ctx.body.value);
    return { kind: "submit", model: body.model, action: "decisions", requestBody: body };
  },
  renderDecision: function (ctx, task) {
    // Native presenters run after the host settles billing. Individual answers
    // remain untouched, even if incomplete or a future type is returned.
    if (!isObject(task.data) || !isObject(task.data.answers)) {
      return { __status: 502, __body: { error: { code: 502, message: "Decision response is missing answers" } } };
    }
    // Wrapping also keeps an upstream __status field from being interpreted as
    // a host reply wrapper instead of part of the Google response.
    return { __status: 200, __body: task.data };
  },
  error: function (ctx, error) {
    // The host supplies a sanitized error, not the original HTTP response.
    // Some 4xx details are JSON; preserve their Google envelope when available.
    try {
      const detail = JSON.parse(error.message);
      if (isObject(detail) && isObject(detail.error)) {
        // Preserve the error body even if it contains a host wrapper field.
        return { __status: error.httpStatus, __body: detail };
      }
    } catch (_) {}
    return { error: { code: error.httpStatus, message: error.message } };
  },
};
