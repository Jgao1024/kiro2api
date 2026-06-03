# AI 协议接口文档（Responses 中枢 / OpenAI Chat Completions / Anthropic Messages）

> 本文档基于 `backend/internal/pkg/apicompat` 包源码整理，描述三套协议的字段定义、相互映射规则与流式事件序列。
> 用途：为后续接入新上游（如 Kiro 转发）提供协议对照与转换规范。
>
> 来源文件：
> - `types.go` — 全部结构体定义
> - `anthropic_to_responses.go` / `responses_to_anthropic.go` — Anthropic ⇄ Responses
> - `responses_to_anthropic_request.go` / `anthropic_to_responses_response.go` — 反方向补全
> - `chatcompletions_to_responses.go` / `responses_to_chatcompletions.go` — Chat ⇄ Responses
> - `chatcompletions_responses_bridge.go` — Responses → Chat 请求与流式桥接

---

## 0. 总体架构

本系统以 **OpenAI Responses API** 作为「中枢格式（hub format）」。所有协议互转都经过 Responses，避免 N×N 两两转换：

```
                    ┌──────────────────────┐
  Anthropic  ◄─────►│                      │◄─────► OpenAI Chat
  Messages          │   Responses (中枢)    │        Completions
  /v1/messages      │   /v1/responses      │        /v1/chat/completions
                    └──────────────────────┘
```

转换链路（函数级）：

| 方向 | 第一跳 | 第二跳 |
|------|--------|--------|
| Anthropic 请求 → OpenAI 上游 | `AnthropicToResponses` | （直接发 Responses 上游）|
| OpenAI Responses 响应 → Anthropic 客户端 | `ResponsesToAnthropic` / 流式 `ResponsesEventToAnthropicEvents` | — |
| Chat 请求 → Responses 上游 | `ChatCompletionsToResponses` | — |
| Responses 响应 → Chat 客户端 | `ResponsesToChatCompletions` / 流式 `ResponsesEventToChatChunks` | — |
| Responses 请求 → Anthropic 上游 | `ResponsesToAnthropicRequest` | — |
| Responses 请求 → Chat 上游 | `ResponsesToChatCompletionsRequest` | — |
| Anthropic 响应 → Responses | `AnthropicToResponsesResponse` | — |
| Chat 请求 → Anthropic | `ChatCompletionsToResponses` → `ResponsesToAnthropicRequest` | 两跳 |

> 对接 Kiro 时，最小工作量是实现 **Kiro 协议 ⇄ Responses** 一对转换，即可复用上述全部链路对接到 Anthropic / Chat 客户端。

---

## 1. Responses 协议（中枢格式 = OpenAI 新格式 = Codex 格式）

> **重要澄清**：OpenAI 只有两套对话协议——老的 Chat Completions（`/v1/chat/completions`，见 [第 2 节](#2-openai-chat-completions-协议老格式)）和新的 **Responses API**（`/v1/responses`）。
> **Responses 就是 OpenAI 的最新格式，也正是 Codex CLI / ChatGPT Codex 使用的格式**，并不存在第三种「Codex 专用协议」。本系统把它选作中枢格式，正因为它是 OpenAI 当前主推、表达力最强的协议（原生承载 reasoning、加密推理续接、工具调用、多模态）。
>
> Codex 路径在 Responses 基础上额外依赖以下字段/约定（来自 `openai_codex_transform.go`、`openai_gateway_service.go`）：
> - **`instructions`** — Codex 模型强制要求非空，缺失会被本地 403 拦截（`ensureCodexOAuthInstructionsField`）。
> - **`previous_response_id`** — 多轮续接，服务端有状态接续上一响应。
> - **`include: ["reasoning.encrypted_content"]`** + input 中的 [`reasoning` 项](#responsesinputitem)（带 `encrypted_content`）— 跨轮携带加密推理上下文；精确重试时会被 `trimOpenAIEncryptedReasoningItems` 移除。
> - **`store`** — 是否服务端存储；OAuth 透传路径与本系统转换路径取值不同。
>
> ⚠️ apicompat 的 `ResponsesRequest`/`ResponsesInputItem` 结构体**未显式定义** `encrypted_content` 等 Codex 扩展字段，这些在透传路径中以**原始 JSON**形式保留（gateway 直接操作 `map[string]any` / gjson）。对接 Kiro 多轮续接时需注意保留这类透传字段。

### 1.1 请求：`POST /v1/responses`

`ResponsesRequest`：

| 字段 | JSON | 类型 | 说明 |
|------|------|------|------|
| Model | `model` | string | 模型名 |
| Instructions | `instructions` | string,opt | 系统级指令（独立于 input）|
| Input | `input` | string \| [`[]ResponsesInputItem`](#responsesinputitem) | 必填，对话输入 |
| MaxOutputTokens | `max_output_tokens` | *int,opt | 最大输出 token，下限 `minMaxOutputTokens=128` |
| Temperature | `temperature` | *float64,opt | reasoning 模型不接受 |
| TopP | `top_p` | *float64,opt | reasoning 模型不接受 |
| Stream | `stream` | bool | 是否流式 |
| Tools | `tools` | [`[]ResponsesTool`](#responsestool),opt | 工具定义 |
| Include | `include` | []string,opt | 附加返回项，如 `reasoning.encrypted_content` |
| Store | `store` | *bool,opt | 是否服务端存储（本系统恒置 false）|
| ParallelToolCalls | `parallel_tool_calls` | *bool,opt | 并行工具调用 |
| Reasoning | `reasoning` | [*ResponsesReasoning](#responsesreasoning),opt | 推理配置 |
| Text | `text` | [*ResponsesText](#responsestext),opt | 文本输出配置 |
| ToolChoice | `tool_choice` | rawJSON,opt | 工具选择策略 |
| ServiceTier | `service_tier` | string,opt | 服务等级 |
| PromptCacheKey | `prompt_cache_key` | string,opt | 提示缓存键 |
| PreviousResponseID | `previous_response_id` | string,opt | 续接上一响应 |

<a id="responsesreasoning"></a>
`ResponsesReasoning`：
| 字段 | JSON | 值域 |
|------|------|------|
| Effort | `effort` | `low` \| `medium` \| `high` \| `xhigh` |
| Summary | `summary` | `auto` \| `concise` \| `detailed` |

<a id="responsestext"></a>
`ResponsesText`：
| 字段 | JSON | 值域 |
|------|------|------|
| Verbosity | `verbosity` | `low` \| `medium` \| `high` |

<a id="responsesinputitem"></a>
### 1.2 输入项 `ResponsesInputItem`

`Type` 决定其余字段的含义：

| Type | 用途 | 关键字段 |
|------|------|----------|
| `""`（空，按 Role 区分）| 角色消息 | `role`(developer/system/user/assistant)、`content`(string \| [`[]ResponsesContentPart`](#responsescontentpart)) |
| `message` | 角色消息（显式）| 同上 |
| `function_call` | 助手发起工具调用 | `call_id`、`name`、`arguments`、`id` |
| `function_call_output` | 工具结果回填 | `call_id`、`output`(string) |
| `input_text` / `text` | 裸文本项（Codex 续接常见）| `text` |
| `input_image` | 裸图片项 | `image_url`(data URI) |
| `reasoning` | **Codex 加密推理项**（透传字段，结构体未显式定义）| `encrypted_content`、`summary[]` |

<a id="responsescontentpart"></a>
`ResponsesContentPart`：
| 字段 | JSON | 说明 |
|------|------|------|
| Type | `type` | `input_text` \| `output_text` \| `input_image` |
| Text | `text` | 文本内容 |
| ImageURL | `image_url` | `input_image` 的 data URI |

> 注意：`function_call_output.output` 只接受字符串。工具结果里的图片需单独拆成 user 消息的 `input_image` part（见 3.2）。

<a id="responsestool"></a>
### 1.3 工具 `ResponsesTool`

| 字段 | JSON | 说明 |
|------|------|------|
| Type | `type` | `function` \| `web_search` \| `local_shell` 等 |
| Name | `name` | 函数名 |
| Description | `description` | 描述 |
| Parameters | `parameters` | JSON Schema |
| Strict | `strict` | 严格模式 |

### 1.4 响应：`ResponsesResponse`

| 字段 | JSON | 说明 |
|------|------|------|
| ID | `id` | 响应 ID |
| Object | `object` | 恒为 `response` |
| Model | `model` | 模型名 |
| Status | `status` | `completed` \| `incomplete` \| `failed` |
| Output | `output` | [`[]ResponsesOutput`](#responsesoutput) |
| Usage | `usage` | [*ResponsesUsage](#responsesusage) |
| IncompleteDetails | `incomplete_details` | status=incomplete 时存在，`reason`=`max_output_tokens`\|`content_filter` |
| Error | `error` | status=failed 时存在，`{code,message}` |

<a id="responsesoutput"></a>
`ResponsesOutput`（按 `type` 区分）：

| Type | 字段 | 说明 |
|------|------|------|
| `message` | `id`、`role`、`content[]`、`status` | 文本消息，content 为 `output_text` parts |
| `reasoning` | `encrypted_content`、`summary[]` | 推理；`summary[].type=summary_text` |
| `function_call` | `call_id`、`name`、`arguments` | 工具调用 |
| `web_search_call` | `id`、`action{type,query}` | 联网搜索调用 |

<a id="responsesusage"></a>
### 1.5 用量 `ResponsesUsage`

| 字段 | JSON | 说明 |
|------|------|------|
| InputTokens | `input_tokens` | 输入 token（兼容反序列化 `prompt_tokens`）|
| OutputTokens | `output_tokens` | 输出 token（兼容 `completion_tokens`）|
| TotalTokens | `total_tokens` | 合计（缺省时自动 = input+output）|
| InputTokensDetails | `input_tokens_details` | `{cached_tokens, audio_tokens}` |
| OutputTokensDetails | `output_tokens_details` | `{reasoning_tokens, audio_tokens, accepted_prediction_tokens, rejected_prediction_tokens}` |

> `UnmarshalJSON` 做了兼容：上游若返回 Chat 风格的 `prompt_tokens`/`completion_tokens`/`prompt_tokens_details`/`completion_tokens_details`，会自动回填到对应字段。

### 1.6 流式事件 `ResponsesStreamEvent`

SSE 事件（`type` 字段决定语义）：

| event type | 携带字段 | 含义 |
|------------|----------|------|
| `response.created` | `response` | 流开始，给出 id/model |
| `response.output_item.added` | `item`、`output_index` | 新输出项（message/reasoning/function_call）开始 |
| `response.output_text.delta` | `delta`、`output_index`、`content_index` | 文本增量 |
| `response.output_text.done` | `text` | 文本块结束 |
| `response.function_call_arguments.delta` | `delta`、`output_index` | 工具参数增量 |
| `response.function_call_arguments.done` | `arguments` | 工具参数结束 |
| `response.reasoning_summary_text.delta` | `delta`、`summary_index` | 推理摘要增量 |
| `response.reasoning_summary_text.done` | — | 推理摘要结束 |
| `response.output_item.done` | `item` | 输出项结束（web_search_call 在此合成结果）|
| `response.completed` | `response` | 正常终止 |
| `response.done` | `response` | 终止别名（Realtime/WS/透传路径）|
| `response.incomplete` / `response.failed` | `response` | 异常终止 |

> 终止事件的 usage 可能出现在 `response.usage`，也可能在事件顶层 `usage`（部分兼容上游）。两处都要兜底读取。

---

## 2. OpenAI Chat Completions 协议（老格式）

### 2.1 请求：`POST /v1/chat/completions`

`ChatCompletionsRequest`：

| 字段 | JSON | 类型 | 说明 |
|------|------|------|------|
| Model | `model` | string | 模型名 |
| Messages | `messages` | [`[]ChatMessage`](#chatmessage) | 对话消息 |
| Instructions | `instructions` | string,opt | Responses 兼容字段 |
| MaxTokens | `max_tokens` | *int,opt | 旧字段 |
| MaxCompletionTokens | `max_completion_tokens` | *int,opt | 新字段，优先于 max_tokens |
| Temperature | `temperature` | *float64,opt | reasoning 模型不接受 |
| TopP | `top_p` | *float64,opt | reasoning 模型不接受 |
| Stream | `stream` | bool | — |
| StreamOptions | `stream_options` | `{include_usage}` | 流式是否带 usage |
| Tools | `tools` | [`[]ChatTool`](#chattool) | 工具（新式）|
| ToolChoice | `tool_choice` | rawJSON | 工具选择 |
| ReasoningEffort | `reasoning_effort` | string | `low`\|`medium`\|`high`\|`xhigh` |
| ServiceTier | `service_tier` | string,opt | — |
| Stop | `stop` | string \| []string | 停止序列 |
| Functions | `functions` | [`[]ChatFunction`](#chattool) | **遗留**函数调用（已弃用仍兼容）|
| FunctionCall | `function_call` | rawJSON | **遗留**函数选择 |

<a id="chatmessage"></a>
### 2.2 消息 `ChatMessage`

| 字段 | JSON | 说明 |
|------|------|------|
| Role | `role` | `system`\|`user`\|`assistant`\|`tool`\|`function` |
| Content | `content` | string \| [`[]ChatContentPart`](#chatcontentpart) |
| ReasoningContent | `reasoning_content` | 推理内容（非标准扩展）|
| Name | `name` | function 角色用作 call_id |
| ToolCalls | `tool_calls` | [`[]ChatToolCall`](#chattoolcall) |
| ToolCallID | `tool_call_id` | tool 角色的目标调用 |
| FunctionCall | `function_call` | 遗留单函数调用 |

<a id="chatcontentpart"></a>
`ChatContentPart`：`type`=`text`\|`image_url`；`image_url`={url, detail}。

<a id="chattoolcall"></a>
`ChatToolCall`：`index`(仅流式)、`id`、`type=function`、`function{name, arguments}`。

<a id="chattool"></a>
`ChatTool` / `ChatFunction`：`ChatTool{type:function, function}`；`ChatFunction{name, description, parameters(JSON Schema), strict}`。

### 2.3 响应 `ChatCompletionsResponse`

| 字段 | JSON | 说明 |
|------|------|------|
| ID | `id` | `chatcmpl-` 前缀 |
| Object | `object` | `chat.completion` |
| Created | `created` | Unix 秒 |
| Model | `model` | — |
| Choices | `choices` | [`[]ChatChoice`](#chatchoice) |
| Usage | `usage` | [*ChatUsage](#chatusage) |

<a id="chatchoice"></a>
`ChatChoice`：`index`、`message`、`finish_reason`(`stop`\|`length`\|`tool_calls`\|`content_filter`)。

<a id="chatusage"></a>
`ChatUsage`：`prompt_tokens`、`completion_tokens`、`total_tokens`、`prompt_tokens_details{cached_tokens,audio_tokens}`、`completion_tokens_details{reasoning_tokens,audio_tokens,accepted/rejected_prediction_tokens}`。

### 2.4 流式 `ChatCompletionsChunk`

`object=chat.completion.chunk`，`choices[].delta` 为 `ChatDelta{role, content(*string), reasoning_content(*string), tool_calls[]}`，`finish_reason` 为 `*string`（未结束时 null）。

> `content` 用指针：区分「不存在」(omit) 与 「空串/null」。最终 finish chunk 带空串 content + finish_reason。

---

## 3. Anthropic Messages 协议

### 3.1 请求：`POST /v1/messages`

`AnthropicRequest`：

| 字段 | JSON | 类型 | 说明 |
|------|------|------|------|
| Model | `model` | string | — |
| MaxTokens | `max_tokens` | int | **必填** |
| System | `system` | string \| [`[]AnthropicContentBlock`](#anthropiccontentblock) | 系统提示 |
| Messages | `messages` | [`[]AnthropicMessage`](#anthropicmessage) | — |
| Tools | `tools` | [`[]AnthropicTool`](#anthropictool) | — |
| Stream | `stream` | bool | — |
| Temperature | `temperature` | *float64,opt | — |
| TopP | `top_p` | *float64,opt | — |
| StopSeqs | `stop_sequences` | []string | — |
| Thinking | `thinking` | `{type, budget_tokens}` | type=`enabled`\|`adaptive`\|`disabled` |
| ToolChoice | `tool_choice` | rawJSON | — |
| Metadata | `metadata` | rawJSON | **原样透传**，含 `user_id`，用于上游判定是否官方 Claude Code |
| OutputConfig | `output_config` | `{effort}` | effort=`low`\|`medium`\|`high`\|`max` |

> ⚠️ Metadata 必须透传：OAuth/Claude Code 路径依赖 `metadata.user_id` 判定请求来源，丢失会被归类为第三方 app。

### 3.2 消息与内容块

<a id="anthropicmessage"></a>
`AnthropicMessage`：`role`(`user`\|`assistant`)、`content`(string \| [`[]AnthropicContentBlock`](#anthropiccontentblock))。

<a id="anthropiccontentblock"></a>
`AnthropicContentBlock`（`type` 区分）：

| type | 字段 | 说明 |
|------|------|------|
| `text` | `text` | 文本 |
| `thinking` | `thinking` | 思考内容 |
| `image` | `source{type=base64, media_type, data}` | 图片 |
| `tool_use` | `id`、`name`、`input`(rawJSON) | 工具调用 |
| `tool_result` | `tool_use_id`、`content`(string\|blocks)、`is_error` | 工具结果 |
| `server_tool_use` | `id`、`name`、`input` | 服务端工具（如 web_search）|
| `web_search_tool_result` | `tool_use_id`、`content` | 搜索结果 |

所有块可带 `cache_control{type=ephemeral, ttl}`。

<a id="anthropictool"></a>
`AnthropicTool`：`type`(如 `web_search_20250305`)、`name`、`description`、`input_schema`(JSON Schema)、`cache_control`。

### 3.3 响应 `AnthropicResponse`

| 字段 | JSON | 说明 |
|------|------|------|
| ID | `id` | — |
| Type | `type` | `message` |
| Role | `role` | `assistant` |
| Content | `content` | `[]AnthropicContentBlock` |
| Model | `model` | — |
| StopReason | `stop_reason` | `end_turn`\|`max_tokens`\|`tool_use`\|... |
| StopSequence | `stop_sequence` | *string |
| Usage | `usage` | [AnthropicUsage](#anthropicusage) |

<a id="anthropicusage"></a>
`AnthropicUsage`：`input_tokens`、`output_tokens`、`cache_creation_input_tokens`、`cache_read_input_tokens`。

### 3.4 流式事件

`AnthropicStreamEvent` 的 `type`：

| event type | 携带字段 | 含义 |
|------------|----------|------|
| `message_start` | `message` | 消息开始，含初始 usage |
| `content_block_start` | `index`、`content_block` | 内容块开始 |
| `content_block_delta` | `index`、`delta` | 内容块增量 |
| `content_block_stop` | `index` | 内容块结束 |
| `message_delta` | `delta{stop_reason}`、`usage` | 终止信息 |
| `message_stop` | — | 消息结束 |

`AnthropicDelta.type`：`text_delta`(text) \| `input_json_delta`(partial_json) \| `thinking_delta`(thinking) \| `signature_delta`(signature)。

---

## 4. 字段级映射规则

### 4.1 Anthropic 请求 → Responses（`AnthropicToResponses`）

| Anthropic | Responses | 规则 |
|-----------|-----------|------|
| `system`(string/blocks) | input 首项 `message{role:developer}` | 文本 → `input_text`；丢弃 `x-anthropic-billing-header:` 开头的块 |
| user 消息 string | `message{role:user, content:[input_text]}` | — |
| user `text` 块 | `input_text` part | — |
| user `image` 块 | `input_image` part | `source` → data URI `data:<mt>;base64,<data>`（mt 缺省 image/png）|
| user `tool_result` 块 | `function_call_output{call_id, output}` | output 仅取文本；内嵌 image 拆为独立 user 消息的 `input_image` |
| assistant 文本块 | `message{role:assistant, content:[output_text]}` | 多块用 `\n\n` 连接 |
| assistant `tool_use` 块 | `function_call{call_id, name, arguments}` | arguments 缺省 `{}` |
| assistant `thinking` 块 | **丢弃** | OpenAI 不接受 thinking 作为输入 |
| `max_tokens` | `max_output_tokens` | 下限 128 |
| `temperature`/`top_p` | 同名 | **gpt-5* 模型剔除**（reasoning 模型不接受，否则上游 400）|
| `output_config.effort` | `reasoning.effort` | 见 4.5；默认 `medium`；`summary:auto` |
| `tools[].web_search*` | `{type:web_search}` | 服务端工具映射 |
| 普通 `tools[]` | `{type:function, ...}` | `input_schema` 经 `normalizeToolParameters` 补 `properties:{}`；`strict:false` |
| `tool_choice` | 见 4.4 | — |
| 固定项 | `store:false`、`parallel_tool_calls:true`、`text.verbosity:medium`、`include:[reasoning.encrypted_content]` | — |

### 4.2 Responses 响应 → Anthropic（`ResponsesToAnthropic`）

| Responses output | Anthropic block | 规则 |
|------------------|-----------------|------|
| `reasoning.summary[].summary_text` | `thinking` 块 | 拼接所有 summary_text |
| `message.content[].output_text` | `text` 块 | — |
| `function_call` | `tool_use` 块 | `id=fromResponsesCallID(call_id)`；input 经 `sanitizeAnthropicToolUseInput` |
| `web_search_call` | `server_tool_use` + `web_search_tool_result` 一对 | tool_use_id=`srvtoolu_`+item.id；结果体为空数组 |
| 空输出 | 单个空 `text` 块 | 保证 content 非空 |

`stop_reason`：`incomplete`+`max_output_tokens`→`max_tokens`；`completed`+有 tool_use→`tool_use`；否则 `end_turn`。

### 4.3 用量映射

**Responses → Anthropic**（`anthropicUsageFromResponsesUsage`）：
- `cached = input_tokens_details.cached_tokens`
- `input_tokens(Anthropic) = input_tokens - cached`（不小于 0）
- `output_tokens` 直传
- `cache_read_input_tokens = cached`

**Responses → Chat**（`chatUsageFromResponsesUsage`）：
- `prompt_tokens = input_tokens`，`completion_tokens = output_tokens`，`total = 两者和`
- `prompt_tokens_details`/`completion_tokens_details` 仅在非零时输出

### 4.4 tool_choice 映射

| Anthropic | Responses | Chat（透传）|
|-----------|-----------|------|
| `{type:auto}` | `"auto"` | `"auto"` |
| `{type:any}` | `"required"` | `"required"` |
| `{type:none}` | `"none"` | `"none"` |
| `{type:tool,name:X}` | `{type:function,name:X}` | — |
| 未知 | 原样透传 | 原样透传 |

反向（`convertResponsesToAnthropicToolChoice`）对称；额外支持遗留 `{type:function,function:{name:X}}` → `{type:tool,name:X}`。

### 4.5 reasoning effort 映射

| Anthropic | Responses | 反向 |
|-----------|-----------|------|
| `low` | `low` | `low` |
| `medium` | `medium` | `medium` |
| `high` | `high` | `high` |
| `max` | `xhigh` | `xhigh`→`max` |

> 仅 `max↔xhigh` 不对称（Anthropic Opus 最高档 ↔ OpenAI GPT-5.2+ 最高档），其余 1:1。

反向 `ResponsesToAnthropicRequest` 还会按 effort 设默认 thinking budget：low=1024, medium=4096, high=10240, max=32768；非 low 时开启 `thinking{type:enabled}`。

### 4.6 call_id 处理

- `toResponsesCallID(id)`：**原样保留**。Claude Code 把 `tool_use.id` 原样回传为 `tool_result.tool_use_id`，Codex 续接要求 call_id 与原始 id 一致。
- `fromResponsesCallID(id)`：剥离遗留 `fc_` 前缀（仅当后缀是 `toolu_`/`call_` 时）。
- `fromResponsesCallIDToAnthropic(id)`：剥前缀；若无 `toolu_`/`call_` 前缀则补 `toolu_`。

### 4.7 Chat 请求 → Responses（`ChatCompletionsToResponses`）

| Chat | Responses |
|------|-----------|
| system 消息 | `message{role:system}` |
| user 消息 | `message{role:user}`（支持多模态 parts）|
| assistant 文本 | `message{role:assistant,output_text}`；`reasoning_content` 包成 `<thinking>...</thinking>` 前置 |
| assistant `tool_calls[]` | 每个 → `function_call` |
| tool 消息 | `function_call_output{call_id=tool_call_id}` |
| function 消息（遗留）| `function_call_output{call_id=name}` |
| `max_completion_tokens`>`max_tokens` | `max_output_tokens`（下限 128）|
| `reasoning_effort` | `reasoning.effort`+`summary:auto` |
| `tools[]`+`functions[]` | 合并为 `ResponsesTool[]` |
| `function_call`(遗留) | → tool_choice |
| 固定项 | `stream:true`、`store:false`、`include:[reasoning.encrypted_content]` |

### 4.8 Responses → Chat 请求（`ResponsesToChatCompletionsRequest`）

`max_output_tokens`→`max_completion_tokens`；`reasoning.effort`→`reasoning_effort`；tools/tool_choice 反向映射。用于只实现 `/v1/chat/completions` 的上游。

---

## 5. 流式事件序列与状态机

### 5.1 Responses → Anthropic 流（`ResponsesEventToAnthropicState`）

状态：`MessageStartSent`、`ContentBlockIndex`、`ContentBlockOpen`、`CurrentBlockType`(text/thinking/tool_use)、`CurrentToolName/Args/HadDelta`、`HasToolCall`、`OutputIndexToBlockIdx`(映射 output_index→块序号)、usage、ResponseID/Model。

事件转换：

| Responses 事件 | 处理 | 产出 Anthropic 事件 |
|----------------|------|---------------------|
| `response.created` | 记录 id/model | `message_start`（仅一次）|
| `output_item.added` (function_call) | 关闭当前块，开 tool_use 块 | `content_block_start{tool_use}` |
| `output_item.added` (reasoning) | 关闭当前块，开 thinking 块 | `content_block_start{thinking}` |
| `output_item.added` (message) | — | 无（等首个 text delta 再开块）|
| `output_text.delta` | 必要时先开 text 块 | `content_block_start{text}?` + `content_block_delta{text_delta}` |
| `output_text.done` | 关闭块 | `content_block_stop` |
| `function_call_arguments.delta` | 累积/透传 | `content_block_delta{input_json_delta}` |
| `function_call_arguments.done` | 收尾 | `content_block_delta{input_json_delta}?` + `content_block_stop` |
| `reasoning_summary_text.delta` | — | `content_block_delta{thinking_delta}` |
| `reasoning_summary_text.done` | 关闭块 | `content_block_stop` |
| `output_item.done` (web_search_call completed) | 合成 | server_tool_use 块(start+stop) + web_search_tool_result 块(start+stop) |
| `response.completed/done/incomplete/failed` | 收尾 | 关闭块 + `message_delta{stop_reason,usage}` + `message_stop` |

`FinalizeResponsesAnthropicStream`：上游异常断开时补发 `message_delta`+`message_stop`（幂等）。

> **特殊处理**：工具名为 `Read` 时参数增量被缓冲不立即下发（`CurrentToolHadDelta` 逻辑），在 done 时经 `sanitizeAnthropicToolUseInput` 清洗（去掉 `pages:""`）后一次性下发，避免 Claude Code 读文件工具的空 pages 报错。

### 5.2 Responses → Chat 流（`ResponsesEventToChatState`）

状态：`ID`、`Model`、`SentRole`、`SawToolCall`、`SawText`、`Finalized`、`NextToolCallIndex`、`OutputIndexToToolIndex`、`IncludeUsage`、`Usage`。

| Responses 事件 | 产出 Chat chunk |
|----------------|-----------------|
| `response.created` | role chunk(`delta{role:assistant}`，仅一次)|
| `output_text.delta` | `delta{content}` |
| `output_item.added`(function_call) | `delta{tool_calls:[{index,id,type,function.name}]}` |
| `function_call_arguments.delta` | `delta{tool_calls:[{index,function.arguments}]}` |
| `reasoning_summary_text.delta` | `delta{reasoning_content}` |
| `response.completed/...` | finish chunk(`finish_reason`) + 可选 usage chunk |

`finish_reason`：incomplete+max→`length`；有 tool_call→`tool_calls`；否则 `stop`。

### 5.3 BufferedResponseAccumulator（非流式兜底）

当上游以 SSE 推送但终止事件 `output` 为空时，累积 delta（text/reasoning/function_call），通过 `BuildOutput()` 重建 `[]ResponsesOutput`（顺序：reasoning→message→function_calls），再用 `SupplementResponseOutput` 回填到空 `resp.Output`。

---

## 6. 对接 Kiro 的建议

Kiro 底层是 Claude 模型，但走 AWS CodeWhisperer/Q streaming 私有协议。推荐做法：

1. **定义 Kiro 协议结构体**（请求/响应/流式事件），放入新文件如 `kiro_to_responses.go`。
2. **实现一对中枢转换**：
   - `KiroToResponses(req) (*ResponsesRequest, error)` — 入站请求转中枢；
   - `ResponsesToKiro...` / 流式 `ResponsesEventToKiro...` — 出站响应转 Kiro。
   或者若 Kiro 入站就是 Anthropic 格式，直接复用 `AnthropicToResponses`，只需实现 Kiro 上游协议的「Responses ⇄ Kiro 上游」转换。
3. **复用映射语义**：effort（注意 max↔xhigh）、tool_choice（any↔required）、call_id 原样保留、usage 的 cached_tokens 扣减、web_search 合成等规则与本文一致。
4. **流式状态机**参照 `ResponsesEventToAnthropicState` 实现：维护块索引、output_index→块号映射、tool 参数缓冲与收尾。
5. **接入点**：转换完成后挂到现有网关 forward 链路（参考 `OpenAIGatewayService.ForwardAsAnthropic`）。

> 合规提醒：对受订阅协议约束的服务做逆向中转可能违反其使用条款，需自行评估法律与封号风险。
