# OpenAI Chat Completions API 接口文档

---

## 基本信息

| 项目 | 内容 |
|------|------|
| 接口地址 | `POST https://api.openai.com/v1/chat/completions` |
| 认证方式 | Header: `Authorization: Bearer <API_KEY>` |
| Content-Type | `application/json` |
| 响应方式 | SSE 流式（`text/event-stream`）|

---

## 请求 Headers

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `Authorization` | string | ✅ | `Bearer <API_KEY>` |
| `Content-Type` | string | ✅ | 固定 `application/json` |
| `session_id` | string | ❌ | 会话亲和 ID（部分代理支持）|
| `x-client-request-id` | string | ❌ | 同上，部分代理用 |
| `x-session-affinity` | string | ❌ | 同上，部分代理用 |

---

## 请求体（Request Body）

### 顶层结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | ✅ | 模型 ID，如 `gpt-4o`、`o3` |
| `messages` | Message[] | ✅ | 对话消息列表 |
| `stream` | boolean | ✅ | 固定 `true` |
| `stream_options` | object | ❌ | `{ "include_usage": true }` 开启流式 token 用量统计，标准 OpenAI 必加 |
| `max_completion_tokens` | number | ❌ | 最大输出 token（标准 OpenAI 及大多数兼容端点）|
| `max_tokens` | number | ❌ | 最大输出 token（旧端点兼容，如 DeepSeek、Moonshot、Chutes）|
| `temperature` | number | ❌ | 温度，范围 `0~2`，**开启 reasoning 时不可用** |
| `tools` | Tool[] | ❌ | 工具定义列表 |
| `tool_choice` | ToolChoice | ❌ | 工具选择策略 |
| `store` | boolean | ❌ | 是否存储对话，标准 OpenAI 传 `false` 关闭，非标准端点不传 |
| `reasoning_effort` | string | ❌ | 推理强度，OpenAI 原生：`"low"` \| `"medium"` \| `"high"` |
| `thinking` | object | ❌ | DeepSeek 专用思考配置，见下 |
| `enable_thinking` | boolean | ❌ | Qwen / Z.AI 专用，开启思考 |
| `chat_template_kwargs` | object | ❌ | Qwen chat template 专用 |
| `reasoning` | object | ❌ | OpenRouter / Together 专用推理配置 |
| `tool_stream` | boolean | ❌ | Z.AI 专用，开启工具流式 |
| `prompt_cache_key` | string | ❌ | OpenAI 原生 prompt cache key，最长 64 字符，仅含 `[a-zA-Z0-9_-]` |
| `prompt_cache_retention` | string | ❌ | `"24h"` 长缓存（OpenAI 原生）|
| `provider` | object | ❌ | OpenRouter 路由偏好 |
| `providerOptions` | object | ❌ | Vercel AI Gateway 路由偏好 |

---

### Message 结构

消息列表按 `role` 分为四种类型：

#### system / developer 消息

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | `"system"` 普通模型 \| `"developer"` 推理模型（o 系列）|
| `content` | string \| TextBlock[] | ✅ | 系统提示内容 |

#### user 消息

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | 固定 `"user"` |
| `content` | string \| ContentPart[] | ✅ | 纯文本或多模态内容块数组 |

**ContentPart 类型：**

TextPart：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"text"` |
| `text` | string | ✅ | 文本内容 |
| `cache_control` | CacheControl | ❌ | Anthropic 兼容代理（如 OpenRouter）专用缓存控制 |

ImagePart：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"image_url"` |
| `image_url.url` | string | ✅ | `data:<mimeType>;base64,<data>` 格式的 Base64 图片 |

#### assistant 消息

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | 固定 `"assistant"` |
| `content` | string \| null | ❌ | 文本内容，有 tool_calls 时可为 `null`（部分端点要求空字符串）|
| `tool_calls` | ToolCallItem[] | ❌ | 工具调用列表 |
| `reasoning_content` | string | ❌ | DeepSeek 专用，回传思考内容（多轮对话必须原样回传）|
| `reasoning_details` | object[] | ❌ | OpenAI Responses API 专用，加密推理签名 |
| `[reasoning_field]` | string | ❌ | llama.cpp / opencode-go 专用，字段名为 `reasoning_content` 或 `reasoning` |

**ToolCallItem 结构：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | ✅ | 工具调用 ID，最长 40 字符，仅含 `[a-zA-Z0-9_-]` |
| `type` | string | ✅ | 固定 `"function"` |
| `function.name` | string | ✅ | 工具名称 |
| `function.arguments` | string | ✅ | JSON 字符串格式的参数 |

#### tool 消息（工具结果）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | 固定 `"tool"` |
| `tool_call_id` | string | ✅ | 对应的工具调用 ID |
| `content` | string | ✅ | 工具执行结果文本 |
| `name` | string | ❌ | 工具名称（部分端点要求，如 Together）|

---

### Tool 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"function"` |
| `function.name` | string | ✅ | 工具名称 |
| `function.description` | string | ✅ | 工具描述 |
| `function.parameters` | object | ✅ | JSON Schema 格式参数定义 |
| `function.strict` | boolean | ❌ | 严格模式，固定 `false`（部分端点不支持，不传）|
| `cache_control` | CacheControl | ❌ | Anthropic 兼容代理专用，只加在最后一个 tool 上 |

---

### ToolChoice 结构

| 值 | 说明 |
|----|------|
| `"auto"` | 模型自行决定是否调用工具 |
| `"none"` | 禁止调用工具 |
| `"required"` | 强制调用某个工具 |
| `{ "type": "function", "function": { "name": "xxx" } }` | 指定调用某个工具 |

---

### 各端点差异速查

| 端点 / 提供商 | `max_tokens` 字段 | `store` | `developer` role | `reasoning_effort` | 思考格式 |
|---|---|---|---|---|---|
| OpenAI 原生 | `max_completion_tokens` | ✅ 传 `false` | ✅ 推理模型 | ✅ | `reasoning_effort` |
| DeepSeek | `max_tokens` | ❌ | ❌ | ❌ | `thinking: { type: "enabled" }` |
| Qwen | `max_completion_tokens` | ❌ | ❌ | ❌ | `enable_thinking: true` |
| OpenRouter | `max_completion_tokens` | ❌ | ❌ | ❌ | `reasoning: { effort: "high" }` |
| Together | `max_tokens` | ❌ | ❌ | ✅ | `reasoning: { enabled: true }` |
| Moonshot | `max_tokens` | ❌ | ❌ | ❌ | 无 |
| Z.AI | `max_completion_tokens` | ❌ | ❌ | ❌ | `enable_thinking: true` |
| Cerebras | `max_completion_tokens` | ❌ | ❌ | ❌ | 无 |
| xAI (Grok) | `max_completion_tokens` | ❌ | ❌ | ❌ | 无 |

---

## 完整请求示例

```json
POST https://api.openai.com/v1/chat/completions
Content-Type: application/json
Authorization: Bearer sk-xxxxxxxx

{
  "model": "gpt-4o",
  "stream": true,
  "stream_options": { "include_usage": true },
  "max_completion_tokens": 8192,
  "store": false,
  "messages": [
    {
      "role": "system",
      "content": "你是一个专业的代码助手。"
    },
    {
      "role": "user",
      "content": "帮我查一下当前目录有哪些文件"
    },
    {
      "role": "assistant",
      "content": "我来帮你查看当前目录的文件。",
      "tool_calls": [
        {
          "id": "call_abc123",
          "type": "function",
          "function": {
            "name": "Bash",
            "arguments": "{\"command\": \"ls -la\"}"
          }
        }
      ]
    },
    {
      "role": "tool",
      "tool_call_id": "call_abc123",
      "content": "total 48\ndrwxr-xr-x  8 user staff  256 May 30 21:00 .\n-rw-r--r--  1 user staff 1234 May 30 20:00 README.md"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "Bash",
        "description": "在 shell 中执行命令",
        "parameters": {
          "type": "object",
          "properties": {
            "command": {
              "type": "string",
              "description": "要执行的 shell 命令"
            }
          },
          "required": ["command"]
        },
        "strict": false
      }
    }
  ],
  "tool_choice": "auto"
}
```

---

## 响应（SSE 流式）

每个 SSE 事件格式：
```
data: <json_payload>\n\n
```

> OpenAI 的 SSE 只有 `data:` 行，没有 `event:` 行（与 Anthropic 不同）。流结束时发送 `data: [DONE]`。

---

### chunk 结构（每个 data 事件）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 本次对话唯一 ID，所有 chunk 相同，格式 `chatcmpl-xxx` |
| `model` | string | 实际使用的模型 ID（可能与请求不同，如别名解析后）|
| `choices` | Choice[] | 内容选项，通常只有 `choices[0]` |
| `usage` | Usage \| null | token 用量，仅在最后一个 chunk 出现（需开启 `stream_options.include_usage`）|

---

### Choice 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | number | 固定 `0` |
| `delta` | Delta | 本次增量内容 |
| `finish_reason` | string \| null | 结束原因，非最后 chunk 为 `null` |
| `usage` | Usage \| null | 部分端点（如 Moonshot）在 choice 级别返回用量 |

---

### Delta 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `content` | string \| null | 文本增量，拼接即可 |
| `tool_calls` | ToolCallDelta[] \| null | 工具调用增量 |
| `reasoning_content` | string \| null | 思考内容增量（DeepSeek、llama.cpp）|
| `reasoning` | string \| null | 思考内容增量（部分兼容端点）|
| `reasoning_text` | string \| null | 思考内容增量（部分兼容端点）|
| `reasoning_details` | object[] \| null | OpenAI Responses API 加密推理签名 |

> 代码中按 `reasoning_content` → `reasoning` → `reasoning_text` 优先级取第一个非空字段，避免重复。

---

### ToolCallDelta 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | number | 工具调用在列表中的位置，用于多工具并发时定位 |
| `id` | string \| null | 工具调用 ID，仅首个 chunk 有值 |
| `type` | string | 固定 `"function"` |
| `function.name` | string \| null | 工具名称，仅首个 chunk 有值 |
| `function.arguments` | string | 参数 JSON 增量，需拼接后解析 |

---

### finish_reason 值

| 值 | 映射 | 说明 |
|----|------|------|
| `"stop"` / `"end"` | stop | 正常结束 |
| `"length"` | length | 达到 max_tokens 限制 |
| `"tool_calls"` / `"function_call"` | toolUse | 模型请求调用工具 |
| `"content_filter"` | error | 内容被安全过滤 |
| `"network_error"` | error | 网络错误 |

---

### Usage 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `prompt_tokens` | number | 输入 token 总数（含缓存命中）|
| `completion_tokens` | number | 输出 token 数（含 reasoning tokens）|
| `prompt_tokens_details.cached_tokens` | number | 缓存命中 token 数（OpenAI 原生）|
| `prompt_tokens_details.cache_write_tokens` | number | 缓存写入 token 数（OpenRouter 兼容）|
| `prompt_cache_hit_tokens` | number | 缓存命中 token 数（DeepSeek 等兼容端点）|

**实际计费 input token 计算：**
```
input = prompt_tokens - cached_tokens - cache_write_tokens
```

---

## 完整响应流示例

```
data: {"id":"chatcmpl-abc123","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-abc123","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"当前目录"},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-abc123","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"包含以下文件：\n\n- `README.md`"},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-abc123","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}

data: {"id":"chatcmpl-abc123","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":512,"completion_tokens":32,"prompt_tokens_details":{"cached_tokens":256,"cache_write_tokens":0}}}

data: [DONE]
```

**工具调用响应流示例：**

```
data: {"id":"chatcmpl-xyz","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc123","type":"function","function":{"name":"Bash","arguments":""}}]},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-xyz","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"comm"}}]},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-xyz","model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"and\": \"ls -la\"}"}}]},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-xyz","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":null}

data: [DONE]
```

---

## 与 Anthropic API 的关键差异

| 对比项 | OpenAI | Anthropic |
|--------|--------|-----------|
| SSE 格式 | 只有 `data:` 行，无 `event:` | 有 `event:` + `data:` |
| 流结束标志 | `data: [DONE]` | `message_stop` 事件 |
| 工具结果角色 | `role: "tool"` | `role: "user"` + `type: "tool_result"` |
| 思考内容 | delta 扩展字段（非标准）| 标准 `thinking` block |
| 系统提示 | messages 第一条 | 独立 `system` 字段 |
| token 用量 | 最后一个 chunk | `message_start` + `message_delta` |
| 内容块索引 | 无，靠 `tool_calls[].index` | 每个 block 有 `index` |
