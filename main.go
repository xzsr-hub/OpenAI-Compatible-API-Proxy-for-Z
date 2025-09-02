// Cloudflare Worker: OpenAI to Z.ai Proxy (最终完整版)
// 该脚本整合了所有功能：严格的 OpenAI 格式匹配和动态模型列表获取。

// ================= Configuration ==================
// 建议在 Cloudflare 的环境变量中设置这些值 (Settings > Variables)
// SECRET_* 的值应该设置为机密 (Secrets)

const UPSTREAM_URL = "https://chat.z.ai/api/chat/completions";
// 下游客户端鉴权的 key
const DOWNSTREAM_KEY = "sk-xuzishiran91"; // 建议在环境变量中设置: env.DOWNSTREAM_KEY
// 上游 API 的备用 token
const UPSTREAM_TOKEN = "sk-xuzishiran91"; // 建议在环境变量中设置为机密: env.UPSTREAM_TOKEN
const DEBUG_MODE = true;
// 思考内容处理策略: "strip" | "think" | "raw"
const THINK_TAGS_MODE = "strip";
// 是否启用匿名 token
const ANON_TOKEN_ENABLED = true;
// ================================================

const ORIGIN_BASE = "https://chat.z.ai";
const SYSTEM_FINGERPRINT = "fp_generated_by_proxy"; // 静态系统指纹

// 伪装的前端头部信息
const FAKE_HEADERS = {
  "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36 Edg/139.0.0.0",
  "Accept": "application/json, text/event-stream",
  "Accept-Language": "zh-CN, en-US",
  "sec-ch-ua": "\"Not;A=Brand\";v=\"99\", \"Microsoft Edge\";v=\"139\", \"Chromium\";v=\"139\"",
  "sec-ch-ua-mobile": "?0",
  "sec-ch-ua-platform": "\"Windows\"",
  "X-FE-Version": "prod-fe-1.0.70",
  "Origin": ORIGIN_BASE,
};

function debugLog(...args) {
  if (DEBUG_MODE) {
    console.log("[DEBUG]", ...args);
  }
}

async function handleRequest(request, env) {
  const url = new URL(request.url);
  if (request.method === "OPTIONS") return handleCors();

  if (url.pathname === "/v1/models") {
    return handleModels(request, env);
  }
  if (url.pathname === "/v1/chat/completions") {
    return handleChatCompletions(request, env);
  }

  return new Response("Not Found", { status: 404 });
}

function handleCors() {
  const headers = {
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type, Authorization",
  };
  return new Response(null, { status: 204, headers });
}

function applyCors(response) {
    response.headers.set("Access-Control-Allow-Origin", "*");
    response.headers.set("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
    response.headers.set("Access-Control-Allow-Headers", "Content-Type, Authorization");
    return response;
}

async function handleModels(request, env) {
  // 为获取模型列表单独处理认证和头部信息
  let authToken = env.UPSTREAM_TOKEN || UPSTREAM_TOKEN;
  const useAnonToken = (env.ANON_TOKEN_ENABLED !== undefined) ? env.ANON_TOKEN_ENABLED === 'true' : ANON_TOKEN_ENABLED;
  if (useAnonToken) {
    try {
      const anonToken = await getAnonymousToken();
      authToken = anonToken;
      debugLog("匿名 token 获取成功 (for models)");
    } catch (err) {
      debugLog("匿名 token 获取失败 (for models)，回退固定 token:", err.message);
    }
  }

  const headers = {
    ...FAKE_HEADERS,
    "Accept": "application/json", // FIX: 为此端点使用正确的 Accept 头
    "Authorization": `Bearer ${authToken}`,
    "Referer": `${ORIGIN_BASE}/`,
    "Cookie": `token=${authToken}`,
  };

  let responseData;
  try {
    debugLog("正在从上游获取模型列表:", 'https://chat.z.ai/api/models');
    const upstreamResponse = await fetch('https://chat.z.ai/api/models', { headers });

    if (!upstreamResponse.ok) {
      const errorBody = await upstreamResponse.text();
      debugLog(`上游 models API 返回错误: ${upstreamResponse.status}`, errorBody);
      throw new Error(`上游 models API 返回状态 ${upstreamResponse.status}`);
    }

    const upstreamJson = await upstreamResponse.json();

    // 将上游响应转换为 OpenAI 格式
    const transformedModels = upstreamJson.data.map(model => ({
      id: model.id,
      object: "model",
      created: model.created_at, // 使用源数据中的 'created_at' 时间戳
      owned_by: model.owned_by,
    }));

    responseData = {
      object: "list",
      data: transformedModels,
    };

  } catch (error) {
    debugLog("获取或处理上游模型列表失败:", error);
    // 返回一个空列表作为备用方案，以避免客户端错误
    responseData = {
      object: "list",
      data: [],
    };
  }

  const response = new Response(JSON.stringify(responseData), {
    headers: { "Content-Type": "application/json" },
  });
  return applyCors(response);
}


async function handleChatCompletions(request, env) {
  debugLog("收到 chat completions 请求");

  const authHeader = request.headers.get("Authorization") || "";
  const apiKey = authHeader.replace("Bearer ", "");
  const expectedKey = env.DOWNSTREAM_KEY || DOWNSTREAM_KEY;

  if (apiKey !== expectedKey) {
    debugLog("无效的 API key:", apiKey);
    return applyCors(new Response("Invalid API key", { status: 401 }));
  }
  debugLog("API key 验证通过");

  let openaiReq;
  try {
    openaiReq = await request.json();
  } catch (e) {
    debugLog("JSON 解析失败:", e);
    return applyCors(new Response("Invalid JSON", { status: 400 }));
  }
  debugLog(`请求解析成功 - 模型: ${openaiReq.model}, 流式: ${openaiReq.stream}`);

  const chatID = `${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
  const upstreamReq = {
    stream: true, // 总是从上游以流式获取，便于统一处理
    chat_id: chatID,
    id: `${Date.now()}`,
    model: openaiReq.model, // 直接使用客户端指定的模型ID
    messages: openaiReq.messages.map(msg => ({
      role: msg.role === 'developer' ? 'system' : msg.role, // 角色规范化
      content: msg.content
    })),
    params: {},
    features: { enable_thinking: true },
    background_tasks: { title_generation: false, tags_generation: false },
  };

  let authToken = env.UPSTREAM_TOKEN || UPSTREAM_TOKEN;
  const useAnonToken = (env.ANON_TOKEN_ENABLED !== undefined) ? env.ANON_TOKEN_ENABLED === 'true' : ANON_TOKEN_ENABLED;
  if (useAnonToken) {
    try {
      const anonToken = await getAnonymousToken();
      authToken = anonToken;
      debugLog("匿名 token 获取成功");
    } catch (err) {
      debugLog("匿名 token 获取失败，回退固定 token:", err.message);
    }
  }
  
  const upstreamResponse = await callUpstream(upstreamReq, chatID, authToken);

  if (!upstreamResponse.ok) {
      const errorBody = await upstreamResponse.text();
      debugLog(`上游错误: ${upstreamResponse.status}`, errorBody);
      return applyCors(new Response("Upstream error", { status: 502 }));
  }

  if (openaiReq.stream) {
    const stream = processUpstreamStream(upstreamResponse.body, openaiReq);
    const response = new Response(stream, {
        headers: { "Content-Type": "text/event-stream", "Cache-Control": "no-cache", "Connection": "keep-alive" }
    });
    return applyCors(response);
  } else {
    const fullContent = await collectFullResponse(upstreamResponse.body);
    // 构造严格匹配的非流式响应
    const openaiResponse = {
        id: `chatcmpl-${Date.now()}`,
        object: "chat.completion",
        created: Math.floor(Date.now() / 1000),
        model: openaiReq.model,
        choices: [{
            index: 0,
            message: {
                role: "assistant",
                content: fullContent,
                refusal: null,
                annotations: [],
            },
            logprobs: null,
            finish_reason: "stop",
        }],
        usage: { // 填充 usage 结构
            prompt_tokens: 0,
            completion_tokens: 0,
            total_tokens: 0,
            prompt_tokens_details: { cached_tokens: 0, audio_tokens: 0 },
            completion_tokens_details: { reasoning_tokens: 0, audio_tokens: 0, accepted_prediction_tokens: 0, rejected_prediction_tokens: 0 }
        },
        service_tier: "default",
        system_fingerprint: SYSTEM_FINGERPRINT,
    };
    const response = new Response(JSON.stringify(openaiResponse), {
        headers: { 'Content-Type': 'application/json' }
    });
    return applyCors(response);
  }
}

async function getAnonymousToken() {
  const reqHeaders = { ...FAKE_HEADERS };
  delete reqHeaders.Accept; // getAnonymousToken 不需要指定 Accept
  reqHeaders.Referer = ORIGIN_BASE + "/";

  const response = await fetch(`${ORIGIN_BASE}/api/v1/auths/`, {
    method: "GET",
    headers: reqHeaders,
  });

  if (!response.ok) throw new Error(`anon token status=${response.status}`);
  const data = await response.json();
  if (!data.token) throw new Error("anon token empty");
  return data.token;
}

async function callUpstream(upstreamReq, refererChatID, authToken) {
  const reqBody = JSON.stringify(upstreamReq);
  debugLog("上游请求体:", reqBody);

  const headers = {
    ...FAKE_HEADERS,
    "Content-Type": "application/json",
    "Authorization": `Bearer ${authToken}`,
    "Referer": `${ORIGIN_BASE}/c/${refererChatID}`,
    "Cookie": `token=${authToken}`,
  };

  return fetch(UPSTREAM_URL, { method: "POST", headers, body: reqBody });
}

function processUpstreamStream(upstreamBody, openaiReq) {
  const encoder = new TextEncoder();
  const decoder = new TextDecoder();
  let buffer = '';
  const includeUsage = openaiReq.stream_options && openaiReq.stream_options.include_usage === true;

  return new ReadableStream({
    async start(controller) {
      const modelIdentifier = openaiReq.model;
      
      // 第一个 chunk
      const firstChunk = {
        id: `chatcmpl-${Date.now()}`,
        object: "chat.completion.chunk",
        created: Math.floor(Date.now() / 1000),
        model: modelIdentifier,
        system_fingerprint: SYSTEM_FINGERPRINT,
        choices: [{ index: 0, delta: { role: "assistant", content: "" }, logprobs: null, finish_reason: null }],
      };
      controller.enqueue(encoder.encode(`data: ${JSON.stringify(firstChunk)}\n\n`));
      
      const reader = upstreamBody.getReader();

      async function pump() {
        const { done, value } = await reader.read();
        if (done) {
          // 流结束
          const endChunk = {
            id: `chatcmpl-${Date.now()}`,
            object: "chat.completion.chunk",
            created: Math.floor(Date.now() / 1000),
            model: modelIdentifier,
            system_fingerprint: SYSTEM_FINGERPRINT,
            choices: [{ index: 0, delta: {}, logprobs: null, finish_reason: "stop" }],
          };
          controller.enqueue(encoder.encode(`data: ${JSON.stringify(endChunk)}\n\n`));

          if (includeUsage) {
            const usageChunk = {
              id: `chatcmpl-${Date.now()}`,
              object: "chat.completion.chunk",
              created: Math.floor(Date.now() / 1000),
              model: modelIdentifier,
              system_fingerprint: SYSTEM_FINGERPRINT,
              choices: [], // Usage chunk has empty choices
              usage: { prompt_tokens: 0, completion_tokens: 0, total_tokens: 0 },
            };
            controller.enqueue(encoder.encode(`data: ${JSON.stringify(usageChunk)}\n\n`));
          }

          controller.enqueue(encoder.encode("data: [DONE]\n\n"));
          controller.close();
          return;
        }

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop();

        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const dataStr = line.substring(6);
          if (!dataStr) continue;
          
          try {
            const upstreamData = JSON.parse(dataStr);
            if (upstreamData.error || (upstreamData.data && upstreamData.data.error)) {
              debugLog("上游错误:", upstreamData.error || upstreamData.data.error);
              continue; // 忽略错误块
            }

            if (upstreamData.data && upstreamData.data.delta_content) {
              let out = upstreamData.data.delta_content;
              if (upstreamData.data.phase === "thinking") {
                out = transformThinking(out);
              }
              if (out) {
                const chunk = {
                    id: `chatcmpl-${Date.now()}`,
                    object: "chat.completion.chunk",
                    created: Math.floor(Date.now() / 1000),
                    model: modelIdentifier,
                    system_fingerprint: SYSTEM_FINGERPRINT,
                    choices: [{ index: 0, delta: { content: out }, logprobs: null, finish_reason: null }],
                };
                controller.enqueue(encoder.encode(`data: ${JSON.stringify(chunk)}\n\n`));
              }
            }

            if (upstreamData.data && (upstreamData.data.done || upstreamData.data.phase === "done")) {
              // 上游已完成，但我们等待reader.read()的done信号来发送最终块
              // 这样可以确保所有缓冲的数据都已处理
              continue;
            }
          } catch (e) {
            debugLog("SSE 数据解析失败:", dataStr, e);
          }
        }
        return pump();
      }
      await pump();
    },
  });
}

async function collectFullResponse(upstreamBody) {
    const reader = upstreamBody.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let fullContent = '';

    while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop();

        for (const line of lines) {
            if (!line.startsWith("data: ")) continue;
            const dataStr = line.substring(6);
            if (!dataStr) continue;

            try {
                const upstreamData = JSON.parse(dataStr);
                if (upstreamData.data && upstreamData.data.delta_content) {
                    let out = upstreamData.data.delta_content;
                    if (upstreamData.data.phase === "thinking") {
                        out = transformThinking(out);
                    }
                    if (out) fullContent += out;
                }
                if (upstreamData.data && (upstreamData.data.done || upstreamData.data.phase === "done")) {
                    await reader.cancel();
                    return fullContent;
                }
            } catch (e) {}
        }
    }
    return fullContent;
}

function transformThinking(s) {
    if (!s) return "";
    s = s.replace(/<summary>.*?<\/summary>/gs, "");
    s = s.replace(/<\/thinking>|<Full>|<\/Full>/g, "");
    s = s.trim();

    const mode = THINK_TAGS_MODE; // Can be overridden by env vars
    switch (mode) {
        case "think":
            s = s.replace(/<details[^>]*>/g, "<think>").replace(/<\/details>/g, "</think>");
            break;
        case "strip":
            s = s.replace(/<details[^>]*>/g, "").replace(/<\/details>/g, "");
            break;
    }

    if (s.startsWith("> ")) s = s.substring(2);
    s = s.replace(/\n> /g, "\n");
    return s.trim();
}

export default {
  fetch: handleRequest,
};
