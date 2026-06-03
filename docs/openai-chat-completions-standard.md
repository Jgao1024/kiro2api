# OpenAI Chat Completions API 标准接口文档

> 基于 OpenAI 官方 OpenAPI Spec（https://github.com/openai/openai-openapi）整理。
> 这是业界事实标准，所有"OpenAI 兼容接口"均以此为基准。

---

## 基本信息

| 项目 | 内容 |
|------|------|
| 接口地址 | `POST https://api.openai.com/v1/chat/completions` |
| 认证方式 | `Authorization: Bearer <OPENAI_API_KEY>` |
| Content-Type | `application/json` |
| 响应方式 | JSON（非流式）或 SSE 流式（`text/event-stream`）|

---

## 请求 Headers

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `Authorization` | string | ✅ | `Bearer <API_KEY>` |
| `Content-Type` | string | ✅ | 固定 `application/json` |

---

## 请求体（Request Body）

### 顶层结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | ✅ | 模型 ID，如 `gpt-4o`、`o3`、`gpt-4o-mini` |
| `messages` | [Message](#message-结构)[] | ✅ | 对话消息列表，至少 1 条 |
| `stream` | boolean | ❌ | `true` 开启 SSE 流式，默认 `false` |
| `stream_options` | object | ❌ | 流式选项，`{ "include_usage": true }` 在最后一个 chunk 返回 token 用量 |
| `max_completion_tokens` | integer | ❌ | 最大输出 token 数上限（含 reasoning tokens），推荐使用此字段 |
| `max_tokens` | integer | ❌ | 同上，旧版字段名，已被 `max_completion_tokens` 取代，仍兼容 |
| `temperature` | number | ❌ | 采样温度，范围 `0~2`，默认 `1`，越高越随机。与 `top_p` 二选一 |
| `top_p` | number | ❌ | 核采样，范围 `0~1`，默认 `1`。与 `temperature` 二选一 |
| `n` | integer | ❌ | 每次请求生成的候选回复数量，默认 `1` |
| `stop` | string \| string[] | ❌ | 停止序列，最多 4 个，遇到时停止生成 |
| `frequency_penalty` | number | ❌ | 频率惩罚，范围 `-2.0~2.0`，默认 `0`，正值降低重复 |
| `presence_penalty` | number | ❌ | 存在惩罚，范围 `-2.0~2.0`，默认 `0`，正值鼓励新话题 |
| `seed` | integer | ❌ | 随机种子，相同 seed 尽量返回相同结果（尽力而为）|
| `tools` | [Tool](#tool-结构)[] | ❌ | 工具定义列表 |
| `tool_choice` | string \| [ToolChoice](#toolchoice-结构) | ❌ | 工具选择策略 |
| `parallel_tool_calls` | boolean | ❌ | 是否允许并行调用多个工具，默认 `true` |
| `reasoning_effort` | string | ❌ | 推理模型（o 系列）专用，`"low"` \| `"medium"` \| `"high"` |
| `response_format` | [ResponseFormat](#responseformat-结构) | ❌ | 输出格式控制 |
| `modalities` | string[] | ❌ | 输出模态，如 `["text"]`、`["text", "audio"]` |
| `logprobs` | boolean | ❌ | 是否返回 token 的对数概率，默认 `false` |
| `top_logprobs` | integer | ❌ | 每个 token 返回的最高概率候选数，范围 `0~20`，需 `logprobs: true` |
| `logit_bias` | object | ❌ | 调整特定 token 的生成概率，`{ "token_id": bias }` |
| `user` | string | ❌ | 终端用户标识，用于滥用检测 |
| `store` | boolean | ❌ | 是否存储本次对话，默认 `false` |
| `metadata` | object | ❌ | 自定义键值对，配合 `store: true` 使用 |
| `service_tier` | string | ❌ | 服务等级，`"auto"` \| `"default"` \| `"flex"` \| `"scale"` \| `"priority"` |
| `prediction` | object | ❌ | 预测输出内容，用于加速文件重生成场景 |

---

### Message 结构

消息按 `role` 分为五种类型：

#### 1. system 消息（系统提示）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | 固定 `"system"` |
| `content` | string \| [TextPart](#textpart)[] | ✅ | 系统提示内容 |
| `name` | string | ❌ | 参与者名称，用于区分同角色的不同参与者 |

#### 2. developer 消息（开发者指令，o 系列推理模型专用）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | 固定 `"developer"` |
| `content` | string \| [TextPart](#textpart)[] | ✅ | 开发者指令内容，替代 o 系列模型的 system 消息 |
| `name` | string | ❌ | 参与者名称 |

#### 3. user 消息

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | 固定 `"user"` |
| `content` | string \| [ContentPart](#contentpart)[] | ✅ | 用户消息内容，支持多模态 |
| `name` | string | ❌ | 参与者名称 |

#### 4. assistant 消息

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | 固定 `"assistant"` |
| `content` | string \| null | ❌ | 文本内容，有 `tool_calls` 时可为 `null` |
| `refusal` | string \| null | ❌ | 模型拒绝回答时的说明文本 |
| `name` | string | ❌ | 参与者名称 |
| `tool_calls` | [ToolCallItem](#toolcallitem-结构)[] | ❌ | 工具调用列表 |
| `audio` | object | ❌ | 引用之前的音频响应，`{ "id": "<audio_id>" }` |

#### 5. tool 消息（工具执行结果）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | 固定 `"tool"` |
| `tool_call_id` | string | ✅ | 对应的工具调用 ID |
| `content` | string \| [TextPart](#textpart)[] | ✅ | 工具执行结果 |

---

### ContentPart

user 消息的 `content` 为数组时，每个元素为以下类型之一：

#### TextPart

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"text"` |
| `text` | string | ✅ | 文本内容 |

#### ImagePart

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"image_url"` |
| `image_url.url` | string | ✅ | 图片 URL 或 `data:<mimeType>;base64,<data>` |
| `image_url.detail` | string | ❌ | `"auto"` \| `"low"` \| `"high"`，图片分辨率，默认 `"auto"` |

#### AudioPart

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"input_audio"` |
| `input_audio.data` | string | ✅ | Base64 编码的音频数据 |
| `input_audio.format` | string | ✅ | `"wav"` \| `"mp3"` |

#### FilePart

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"file"` |
| `file.file_id` | string | 条件必填 | 已上传文件的 ID |
| `file.file_data` | string | 条件必填 | Base64 编码的文件内容 |
| `file.filename` | string | ❌ | 文件名 |

---

### ToolCallItem 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | ✅ | 工具调用唯一 ID |
| `type` | string | ✅ | 固定 `"function"` |
| `function.name` | string | ✅ | 函数名称 |
| `function.arguments` | string | ✅ | JSON 字符串格式的参数，注意模型可能生成无效 JSON |

---

### Tool 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"function"` |
| `function.name` | string | ✅ | 函数名，仅含 `a-z A-Z 0-9 _ -`，最长 64 字符 |
| `function.description` | string | ❌ | 函数描述，模型依据此决定是否调用 |
| `function.parameters` | object | ❌ | JSON Schema 格式的参数定义 |
| `function.strict` | boolean \| null | ❌ | 是否启用严格模式（结构化输出），默认 `false` |

---

### ToolChoice 结构

| 值 | 说明 |
|----|------|
| `"none"` | 禁止调用工具，模型只生成文本 |
| `"auto"` | 模型自行决定（默认，有 tools 时）|
| `"required"` | 强制调用至少一个工具 |
| `{ "type": "function", "function": { "name": "xxx" } }` | 强制调用指定工具 |

---

### ResponseFormat 结构

| 值 | 说明 |
|----|------|
| `{ "type": "text" }` | 普通文本输出（默认）|
| `{ "type": "json_object" }` | JSON 模式，保证输出合法 JSON |
| `{ "type": "json_schema", "json_schema": { "name": "...", "schema": {...}, "strict": true } }` | 结构化输出，严格按 JSON Schema 生成 |

---

## 完整请求示例

### 普通对话（非流式）

```json
POST https://api.openai.com/v1/chat/completions
Content-Type: application/json
Authorization: Bearer sk-xxxxxxxx

{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "system",
      "content": "你是一个专业的代码助手。"
    },
    {
      "role": "user",
      "content": "用 Python 写一个冒泡排序"
    }
  ],
  "temperature": 0.7,
  "max_completion_tokens": 2048
}
```

### 工具调用（流式）

```json
POST https://api.openai.com/v1/chat/completions
Content-Type: application/json
Authorization: Bearer sk-xxxxxxxx

{
  "model": "gpt-4o",
  "stream": true,
  "stream_options": { "include_usage": true },
  "messages": [
    {
      "role": "user",
      "content": "帮我查一下当前目录有哪些文件"
    },
    {
      "role": "assistant",
      "content": null,
      "tool_calls": [
        {
          "id": "call_abc123",
          "type": "function",
          "function": {
            "name": "bash",
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
        "name": "bash",
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
  "tool_choice": "auto",
  "parallel_tool_calls": true
}
```

---

## 响应体（Response Body）

### 非流式响应示例

```json
{
  "id": "chatcmpl-B9MHDbslfkBeAs8l4bebGdFOJ6PeG",
  "object": "chat.completion",
  "created": 1741570283,
  "model": "gpt-4o-2024-08-06",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "当前目录包含以下文件：\n\n- `README.md`",
        "refusal": null,
        "tool_calls": null,
        "annotations": []
      },
      "finish_reason": "stop",
      "logprobs": null
    }
  ],
  "usage": {
    "prompt_tokens": 512,
    "completion_tokens": 32,
    "total_tokens": 544,
    "prompt_tokens_details": {
      "cached_tokens": 256,
      "audio_tokens": 0
    },
    "completion_tokens_details": {
      "reasoning_tokens": 0,
      "audio_tokens": 0,
      "accepted_prediction_tokens": 0,
      "rejected_prediction_tokens": 0
    }
  },
  "service_tier": "default",
  "system_fingerprint": "fp_fc9f1d7035"
}
```

### 响应顶层字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 唯一 ID，格式 `chatcmpl-xxx` |
| `object` | string | 固定 `"chat.completion"` |
| `created` | integer | Unix 时间戳（秒）|
| `model` | string | 实际使用的模型 ID |
| `choices` | [Choice](#choice-结构)[] | 回复列表，通常只有 `choices[0]` |
| `usage` | [Usage](#usage-结构) | Token 用量统计 |
| `service_tier` | string | 服务等级 |
| `system_fingerprint` | string | 后端配置指纹，配合 `seed` 判断确定性 |

### Choice 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | integer | 候选回复索引 |
| `message` | [ResponseMessage](#responsemessage-结构) | 模型回复消息 |
| `finish_reason` | string | 结束原因，见 [finish_reason 值](#finish_reason-值) |
| `logprobs` | object \| null | Token 对数概率（需请求时开启）|

### finish_reason 值

| 值 | 说明 |
|----|------|
| `"stop"` | 正常结束（命中自然停止点或 stop 序列）|
| `"length"` | 达到 `max_completion_tokens` 限制 |
| `"tool_calls"` | 模型请求调用工具 |
| `"content_filter"` | 内容被安全过滤器拦截 |
| `"function_call"` | 已废弃，旧版函数调用 |

### ResponseMessage 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `role` | string | 固定 `"assistant"` |
| `content` | string \| null | 文本内容，有 tool_calls 时为 `null` |
| `refusal` | string \| null | 模型拒绝回答时的说明 |
| `tool_calls` | [ToolCallItem](#toolcallitem-结构)[] \| null | 工具调用列表 |
| `annotations` | object[] | 引用注释（如 web search 结果的 URL）|
| `audio` | object \| null | 音频响应数据 |

### Usage 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `prompt_tokens` | integer | 输入 token 总数 |
| `completion_tokens` | integer | 输出 token 总数（含 reasoning tokens）|
| `total_tokens` | integer | 总 token 数 |
| `prompt_tokens_details` | [PromptTokensDetails](#prompttokensdetails-结构) | 输入 token 明细 |
| `completion_tokens_details` | [CompletionTokensDetails](#completiontokensdetails-结构) | 输出 token 明细 |

### PromptTokensDetails 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `cached_tokens` | integer | 命中 prompt cache 的 token 数（费用更低）|
| `audio_tokens` | integer | 音频输入 token 数 |

### CompletionTokensDetails 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `reasoning_tokens` | integer | 推理模型内部思考消耗的 token 数 |
| `audio_tokens` | integer | 音频输出 token 数 |
| `accepted_prediction_tokens` | integer | 预测输出中被采用的 token 数 |
| `rejected_prediction_tokens` | integer | 预测输出中被拒绝的 token 数 |

---

## 流式响应（SSE）

### 格式

```
data: <json>\n\n
data: <json>\n\n
...
data: [DONE]\n\n
```

- 每行以 `data: ` 开头，无 `event:` 行
- 空行 `\n\n` 为事件分隔符
- 流结束标志：`data: [DONE]`

### Chunk 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 唯一 ID，同一次请求所有 chunk 相同 |
| `object` | string | 固定 `"chat.completion.chunk"` |
| `created` | integer | Unix 时间戳 |
| `model` | string | 模型 ID |
| `choices` | [StreamChoice](#streamchoice-结构)[] | 增量内容，最后一个 usage chunk 时为空数组 |
| `usage` | [Usage](#usage-结构) \| null | 仅在最后一个 chunk 出现（需 `stream_options.include_usage: true`）|

### StreamChoice 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | integer | 候选索引 |
| `delta` | [Delta](#delta-结构) | 本次增量内容 |
| `finish_reason` | string \| null | 非最后 chunk 为 `null`，见 [finish_reason 值](#finish_reason-值) |
| `logprobs` | object \| null | Token 对数概率 |

### Delta 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `role` | string \| null | 仅首个 chunk 有值，固定 `"assistant"` |
| `content` | string \| null | 文本增量，拼接即可 |
| `refusal` | string \| null | 拒绝文本增量 |
| `tool_calls` | [ToolCallDelta](#toolcalldelta-结构)[] \| null | 工具调用增量 |

### ToolCallDelta 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | integer | 工具调用在列表中的位置，多工具并发时用于定位 |
| `id` | string \| null | 工具调用 ID，仅首个 chunk 有值 |
| `type` | string \| null | 固定 `"function"`，仅首个 chunk 有值 |
| `function.name` | string \| null | 函数名，仅首个 chunk 有值 |
| `function.arguments` | string | 参数 JSON 增量，需拼接后解析 |

---

## 完整流式响应示例

### 普通文本流

```
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"当前目录"},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"包含以下文件：\n\n- `README.md`"},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":512,"completion_tokens":32,"total_tokens":544,"prompt_tokens_details":{"cached_tokens":256,"audio_tokens":0},"completion_tokens_details":{"reasoning_tokens":0,"audio_tokens":0,"accepted_prediction_tokens":0,"rejected_prediction_tokens":0}}}

data: [DONE]
```

### 工具调用流

```
data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":null},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc123","type":"function","function":{"name":"bash","arguments":""}}]},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":"}}]},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"ls -la\"}"}}]},"finish_reason":null}],"usage":null}

data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","created":1741570283,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":null}

data: [DONE]
```

---

## 错误响应

```json
{
  "error": {
    "message": "Invalid API key provided.",
    "type": "invalid_request_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

| HTTP 状态码 | 说明 |
|-------------|------|
| `400` | 请求参数错误 |
| `401` | API Key 无效或缺失 |
| `403` | 无权限访问 |
| `404` | 模型不存在 |
| `422` | 请求体格式错误 |
| `429` | 请求频率超限（Rate Limit）|
| `500` | 服务端内部错误 |
| `503` | 服务暂时不可用 |

---

## 开发注意事项

1. **工具参数解析**：`function.arguments` 是增量 JSON 字符串，流式时需拼接完整后再解析，且模型可能生成无效 JSON，需做容错处理。
2. **多工具并发**：`delta.tool_calls` 是数组，用 `index` 字段区分不同工具调用，不能假设只有一个。
3. **usage 位置**：流式时 usage 在最后一个 chunk（`choices` 为空数组），需开启 `stream_options.include_usage: true`。
4. **content 为 null**：assistant 消息有 `tool_calls` 时 `content` 为 `null`，多轮对话回传时需保留此结构。
5. **finish_reason 检查**：流式时必须等到 `finish_reason` 非 null 的 chunk 才算完整，不能仅靠 `[DONE]` 判断。
6. **system vs developer**：o 系列推理模型（o1、o3 等）使用 `role: "developer"` 替代 `role: "system"`。
7. **Prompt Cache**：`prompt_tokens_details.cached_tokens` 表示命中缓存的 token，费用约为普通输入的 50%，OpenAI 自动管理，无需手动配置。
