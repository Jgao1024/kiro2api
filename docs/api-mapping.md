# OpenAI / Anthropic → AWS CodeWhisperer 接口映射文档

## 概述

kiro2api 作为代理层，支持两条请求路径：

1. **Anthropic 路径**：`POST /v1/messages` → 直接转换为 CodeWhisperer 请求
2. **OpenAI 路径**：`POST /v1/chat/completions` → 先转换为 Anthropic 格式 → 再转换为 CodeWhisperer 请求

响应路径反向：CodeWhisperer EventStream → Anthropic SSE 格式（Anthropic 路径）或 OpenAI SSE/JSON 格式（OpenAI 路径）。

---

## 一、OpenAI → Anthropic 请求映射

### 1.1 顶层字段映射

| OpenAI 字段 | Anthropic 字段 | 转换规则 |
|---|---|---|
| `model` | `model` | 直接透传 |
| `messages[]` | `messages[]` | 逐条转换（见 1.2） |
| `max_tokens` | `max_tokens` | 直接透传；未设置时默认 `16384` |
| `temperature` | `temperature` | 直接透传（可选） |
| `stream` | `stream` | 直接透传；未设置时默认 `false` |
| `tools[]` | `tools[]` | 转换工具格式（见 1.3） |
| `tool_choice` | `tool_choice` | 转换选择策略（见 1.4） |

**丢弃字段**：OpenAI 请求中的 `top_p`、`frequency_penalty`、`presence_penalty`、`n`、`stop`、`logprobs`、`user` 等字段不传递给 Anthropic。

### 1.2 消息角色与内容映射

| OpenAI `role` | Anthropic `role` | 说明 |
|---|---|---|
| `user` | `user` | 直接透传 |
| `assistant` | `assistant` | 直接透传 |
| `system` | _(不直接映射)_ | OpenAI 的 system 消息保留在 messages 数组中，由 CodeWhisperer 层处理为 history |
| `tool` | `user` (content 含 `tool_result` 块) | OpenAI 工具结果消息转换为含 tool_result 内容块的 user 消息 |

**消息内容块转换**：

| OpenAI 内容类型 | Anthropic 内容类型 | 转换规则 |
|---|---|---|
| `string` | `string` | 直接透传 |
| `{"type":"text","text":"..."}` | `{"type":"text","text":"..."}` | 直接透传 |
| `{"type":"image_url","image_url":{"url":"data:..."}}` | `{"type":"image","source":{"type":"base64","media_type":"...","data":"..."}}` | base64 data URI 解析后转换 |
| `{"type":"image_url","image_url":{"url":"https://..."}}` | 不支持 | URL 类型图片无法转换，跳过 |
| `{"type":"tool_use",...}` | `{"type":"tool_use",...}` | 直接透传（过滤 web_search） |
| `{"type":"tool_result",...}` | `{"type":"tool_result",...}` | 直接透传 |

### 1.3 工具定义映射（OpenAI → Anthropic）

```
OpenAITool                              AnthropicTool
────────────────────────────────────────────────────────
type: "function"                    →   (丢弃，仅支持 function 类型)
function.name                       →   name
function.description                →   description
function.parameters (JSON Schema)   →   input_schema
function.strict                     →   (丢弃)
```

**Schema 清理规则**（`cleanAndValidateToolParameters`）：
- 删除不兼容字段：`additionalProperties`、`strict`、`$schema`、`$id`、`$ref`、`definitions`、`$defs`
- 若顶层无 `type` 字段，自动补充 `"type": "object"`
- 参数名超过 64 字符时截断（>80 字符取前20+后20，否则取前30+"_param"）
- `required` 字段确保为字符串数组，`properties` 字段确保为对象

**过滤规则**：`web_search` / `websearch` 工具静默过滤，不发送给上游。

### 1.4 tool_choice 映射（OpenAI → Anthropic）

| OpenAI `tool_choice` | Anthropic `tool_choice` |
|---|---|
| `"auto"` | `{"type":"auto"}` |
| `"required"` | `{"type":"any"}` |
| `"none"` | `null`（不传） |
| `{"type":"function","function":{"name":"xxx"}}` | `{"type":"tool","name":"xxx"}` |
| 其他未知值 | `{"type":"auto"}` |

---

## 二、Anthropic → CodeWhisperer 请求映射

### 2.1 顶层结构映射

| Anthropic 字段 | CodeWhisperer 字段 | 说明 |
|---|---|---|
| `model` | `conversationState.currentMessage.userInputMessage.modelId` | 经过 ModelMap 转换 |
| `messages[-1].content` | `conversationState.currentMessage.userInputMessage.content` | 仅取最后一条消息的文本部分 |
| `messages[-1].content[image]` | `conversationState.currentMessage.userInputMessage.images[]` | 图片单独提取 |
| `tools[]` | `conversationState.currentMessage.userInputMessage.userInputMessageContext.tools[]` | 工具定义转换 |
| `messages[-1].content[tool_result]` | `conversationState.currentMessage.userInputMessage.userInputMessageContext.toolResults[]` | 工具结果提取，同时将 content 置为空字符串 |
| `messages[0..-2]` + `system[]` | `conversationState.history[]` | 历史消息构建（见 2.4） |
| _(自动生成)_ | `conversationState.conversationId` | 基于客户端 IP/UA 稳定生成（UUID 格式） |
| _(自动生成)_ | `conversationState.agentContinuationId` | 基于客户端信息稳定生成 |
| _(固定值)_ | `conversationState.agentTaskType` | 固定为 `"vibe"` |
| _(固定值)_ | `conversationState.currentMessage.userInputMessage.origin` | 固定为 `"AI_EDITOR"` |
| `tools` 存在 + `tool_choice` 类型 | `conversationState.chatTriggerType` | `"AUTO"` 或 `"MANUAL"`（见 2.2） |

**丢弃字段**：`stream`、`metadata`、`stop_sequences`、`top_k`、`top_p` 不传递给 CodeWhisperer。

### 2.2 模型名称映射

| Anthropic 模型名（输入） | CodeWhisperer modelId（输出） |
|---|---|
| `claude-haiku-4-5` | `claude-haiku-4.5` |
| `claude-haiku-4.5` | `claude-haiku-4.5` |
| `claude-sonnet-4-6` | `claude-sonnet-4.6` |
| `claude-sonnet-4.6` | `claude-sonnet-4.6` |
| `claude-opus-4-7` | `claude-opus-4.7` |
| `claude-opus-4.7` | `claude-opus-4.7` |
| `claude-opus-4-8` | `claude-opus-4.8` |
| `claude-opus-4.8` | `claude-opus-4.8` |
| 其他未知模型 | 返回 `model_not_found` 错误 |

### 2.3 chatTriggerType 决策

| 条件 | chatTriggerType |
|---|---|
| 无工具，或工具存在但 tool_choice 为 auto/未设置 | `"MANUAL"` |
| tool_choice 为 `"any"` 或 `"tool"` | `"AUTO"` |

### 2.4 消息内容处理（currentMessage）

最后一条消息按内容块类型处理：

| 内容块类型 | 处理方式 |
|---|---|
| `string` | 直接作为 `content` |
| `text` 块 | 拼接到 `content` |
| `image` 块（base64） | 转换为 `CodeWhispererImage`，放入 `images[]` |
| `image_url` 块（OpenAI 格式） | 解析 base64 data URI 后放入 `images[]` |
| `tool_result` 块 | 提取到 `userInputMessageContext.toolResults[]`，`content` 置为空字符串 |
| `tool_use` 块 | 忽略（不放入 currentMessage） |

### 2.5 工具定义映射（Anthropic → CodeWhisperer）

```
AnthropicTool                              CodeWhispererTool
──────────────────────────────────────────────────────────────
name                                   →   toolSpecification.name
description (截断至 10000 字符)        →   toolSpecification.description
input_schema (map[string]any)          →   toolSpecification.inputSchema.json
```

### 2.6 工具结果映射（Anthropic → CodeWhisperer）

```
ContentBlock (tool_result)              ToolResult
──────────────────────────────────────────────────────────────
tool_use_id                         →   toolUseId
content (string)                    →   content: [{"text": "<string>"}]
content ([]any)                     →   content: <原数组>
content (map[string]any)            →   content: [<原对象>]
is_error: false / 未设置             →   status: "success", isError: false
is_error: true                      →   status: "error", isError: true
```

### 2.7 历史消息构建

历史消息（`conversationState.history[]`）构建规则：

1. **System 消息**：所有 `system[]` 内容拼接后，作为一对 `HistoryUserMessage("system内容") + HistoryAssistantMessage("OK")` 放在历史最前面。

2. **对话历史**（`messages[0..-2]`，最后一条为 currentMessage）：
   - 连续的 `user` 消息合并为一条（文本用 `\n` 拼接，图片和工具结果合并）
   - 遇到 `assistant` 消息时，与前面累积的 `user` 消息配对
   - `assistant` 消息中的 `tool_use` 块提取为 `assistantResponseMessage.toolUses[]`
   - 孤立的 `user` 消息（末尾无对应 `assistant`）自动配对一条 `HistoryAssistantMessage("OK")`
   - 孤立的 `assistant` 消息（无前置 user）直接忽略

3. **历史消息结构**：

```
HistoryUserMessage:
  userInputMessage.content          ← 文本内容（含工具结果时为空字符串）
  userInputMessage.modelId          ← 同 currentMessage 的 modelId
  userInputMessage.origin           ← 固定 "AI_EDITOR"
  userInputMessage.images[]         ← 图片列表（可选）
  userInputMessage.userInputMessageContext.toolResults[]  ← 工具结果（可选）

HistoryAssistantMessage:
  assistantResponseMessage.content  ← 文本内容
  assistantResponseMessage.toolUses[] ← [{toolUseId, name, input}]（可选）
```

---

## 三、CodeWhisperer → Anthropic 响应映射

### 3.1 传输协议

CodeWhisperer 返回 AWS EventStream 二进制格式（BigEndian 帧结构），每帧包含：

```
[4B 总长度][4B 头部长度][4B CRC][头部键值对][Payload JSON][4B 尾部 CRC]
```

头部关键字段：
- `:message-type`：`"event"` / `"error"` / `"exception"`
- `:event-type`：`"assistantResponseEvent"` / `"toolUseEvent"` / `"meteringEvent"` / `"contextUsageEvent"`
- `:content-type`：`"application/json"`

### 3.2 CodeWhisperer 事件类型映射

| `:message-type` | `:event-type` | Payload 关键字段 | 生成的 Anthropic SSE 事件 |
|---|---|---|---|
| `event` | `assistantResponseEvent` | `content`（文本） | `content_block_delta` (text_delta) |
| `event` | `assistantResponseEvent` | `toolUseId` + `name` + `input`（工具调用） | `content_block_start` (tool_use) + `content_block_delta` (input_json_delta) + `content_block_stop` |
| `event` | `toolUseEvent` | `toolUseId`, `name`, `input`, `stop` | `content_block_start` + `content_block_delta` + `content_block_stop` |
| `event` | `meteringEvent` | `usage`（float64，credits 消耗） | 不转发客户端，内部记录 `creditsUsed` |
| `event` | `contextUsageEvent` | `contextUsagePercentage`（float64） | 不转发客户端，内部记录 `contextUsagePct` |
| `error` | — | `__type`, `message` | `error` 事件（`error_code`, `error_message`） |
| `exception` | — | `__type`, `message` | `exception` 事件（`exception_type`, `exception_message`） |
| `exception` | — | `__type: "ContentLengthExceededException"` | 转换为 `message_delta` (stop_reason: max_tokens) + `message_stop` |

### 3.3 assistantResponseEvent 完整字段

| CodeWhisperer 字段 | 类型 | 用途 |
|---|---|---|
| `conversationId` | string | 会话 ID（内部记录） |
| `messageId` | string | 消息 ID（内部记录） |
| `content` | string | 文本内容，映射为 text_delta |
| `contentType` | enum | `"text/markdown"` / `"text/plain"` / `"application/json"`，默认 markdown |
| `messageStatus` | enum | `"COMPLETED"` / `"IN_PROGRESS"` / `"ERROR"` |
| `supplementaryWebLinks[]` | array | 补充网页链接（不转发给客户端） |
| `references[]` | array | 代码引用（不转发给客户端） |
| `codeReference[]` | array | 代码引用（不转发给客户端） |
| `followupPrompt` | object | 后续提示建议（不转发给客户端） |
| `programmingLanguage` | object | 编程语言信息（不转发给客户端） |
| `customizations[]` | array | 自定义模型信息（不转发给客户端） |
| `userIntent` | enum | 用户意图分类（不转发给客户端） |
| `codeQuery` | object | 代码查询信息（不转发给客户端） |

### 3.4 流式响应完整事件序列（Anthropic 格式）

**纯文本响应**：
```
message_start        type:"message_start", message:{id, type:"message", role:"assistant",
                       content:[], model, stop_reason:null, stop_sequence:null,
                       usage:{input_tokens, output_tokens:0}}
ping                 type:"ping"
content_block_start  type:"content_block_start", index:0, content_block:{type:"text", text:""}
content_block_delta  type:"content_block_delta", index:0, delta:{type:"text_delta", text:"..."}
...（每个 assistantResponseEvent 一个 delta）
content_block_stop   type:"content_block_stop", index:0
message_delta        type:"message_delta", delta:{stop_reason:"end_turn", stop_sequence:null},
                       usage:{output_tokens, input_tokens}
message_stop         type:"message_stop"
```

**含工具调用的响应**：
```
message_start        （同上）
ping
content_block_start  index:0, type:"text"（首次有文本时动态生成）
content_block_delta  index:0, text_delta（工具调用前的介绍文本，当前为空字符串）
content_block_start  index:1, type:"tool_use", id:"toolu_xxx", name:"tool_name", input:{}
content_block_delta  index:1, input_json_delta:{partial_json:"..."}
...（流式工具参数片段）
content_block_stop   index:1
...（多个工具调用重复，index 递增：2, 3, ...）
content_block_stop   index:0（关闭文本块，由 sendFinalEvents 补发）
message_delta        delta:{stop_reason:"tool_use", stop_sequence:null}, usage:{...}
message_stop
```

### 3.5 非流式响应结构（Anthropic 格式）

```json
{
  "type": "message",
  "role": "assistant",
  "model": "<原始请求的 model>",
  "content": [
    {"type": "text", "text": "..."},
    {"type": "tool_use", "id": "toolu_xxx", "name": "tool_name", "input": {...}}
  ],
  "stop_reason": "end_turn" | "tool_use" | "max_tokens",
  "stop_sequence": null,
  "usage": {
    "input_tokens": <估算值>,
    "output_tokens": <估算值>
  }
}
```

### 3.6 stop_reason 决策

| 条件 | stop_reason |
|---|---|
| 响应中包含工具调用（活跃或已完成） | `"tool_use"` |
| 纯文本响应，自然结束 | `"end_turn"` |
| 上游返回 `ContentLengthExceededException` | `"max_tokens"` |

---

## 四、CodeWhisperer → OpenAI 响应映射

### 4.1 非流式响应（OpenAI 格式）

CodeWhisperer → Anthropic 中间格式 → OpenAI 格式：

```
Anthropic 中间格式                      OpenAI 响应
──────────────────────────────────────────────────────────────
(生成)                              →   id: "chatcmpl-<timestamp>"
(固定)                              →   object: "chat.completion"
(生成)                              →   created: <unix timestamp>
model                               →   model
content[].text (拼接)               →   choices[0].message.content
content[].tool_use                  →   choices[0].message.tool_calls[]
role: "assistant"                   →   choices[0].message.role: "assistant"
stop_reason: "end_turn"             →   choices[0].finish_reason: "stop"
stop_reason: "tool_use"             →   choices[0].finish_reason: "tool_calls"
usage.input_tokens                  →   usage.prompt_tokens
usage.output_tokens                 →   usage.completion_tokens
(计算)                              →   usage.total_tokens
```

**工具调用映射**（Anthropic → OpenAI）：

```
Anthropic content[tool_use]             OpenAI tool_calls[]
──────────────────────────────────────────────────────────────
id                                  →   id
(固定)                              →   type: "function"
name                                →   function.name
input (map[string]any)              →   function.arguments (JSON 字符串)
```

### 4.2 流式响应（OpenAI SSE 格式）

**初始事件**：
```json
{"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,
 "model":"...","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}
```

**文本增量**（来自 `content_block_delta` text_delta）：
```json
{"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,
 "model":"...","choices":[{"index":0,"delta":{"content":"<text>"},"finish_reason":null}]}
```

**工具调用开始**（来自 `content_block_start` tool_use）：
```json
{"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,
 "model":"...","choices":[{"index":0,"delta":{"tool_calls":[{
   "index":<tool_idx>,"id":"toolu_xxx","type":"function",
   "function":{"name":"tool_name","arguments":""}}]},"finish_reason":null}]}
```

**工具参数增量**（来自 `content_block_delta` input_json_delta）：
```json
{"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,
 "model":"...","choices":[{"index":0,"delta":{"tool_calls":[{
   "index":<tool_idx>,"type":"function",
   "function":{"arguments":"<partial_json>"}}]},"finish_reason":null}]}
```

**结束事件**（来自 `message_delta`）：
```json
{"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,
 "model":"...","choices":[{"index":0,"delta":{},"finish_reason":"stop"|"tool_calls"}]}
```

**流结束标记**：
```
data: [DONE]
```

**stop_reason 映射**（Anthropic → OpenAI）：

| Anthropic `stop_reason` | OpenAI `finish_reason` |
|---|---|
| `"end_turn"` | `"stop"` |
| `"tool_use"` | `"tool_calls"` |
| `"max_tokens"` | `"length"` |
| `"stop_sequence"` | `"stop"` |

---

## 五、图片格式映射

### 5.1 OpenAI image_url → Anthropic image

| OpenAI | Anthropic |
|---|---|
| `type: "image_url"` | `type: "image"` |
| `image_url.url: "data:<media_type>;base64,<data>"` | `source.type: "base64"`, `source.media_type: "<media_type>"`, `source.data: "<data>"` |

### 5.2 Anthropic image → CodeWhisperer image

| Anthropic | CodeWhisperer |
|---|---|
| `source.type: "base64"` | `source.bytes: <base64数据>` |
| `source.media_type: "image/jpeg"` | `format: "jpeg"` |
| `source.media_type: "image/png"` | `format: "png"` |
| `source.media_type: "image/gif"` | `format: "gif"` |
| `source.media_type: "image/webp"` | `format: "webp"` |

---

## 六、错误映射

### 6.1 CodeWhisperer 错误 → Anthropic 错误

| CodeWhisperer 错误 | HTTP 状态码 | Anthropic 响应 |
|---|---|---|
| `reason: "CONTENT_LENGTH_EXCEEDS_THRESHOLD"` | 400 | `message_delta` (stop_reason: max_tokens) + `message_stop` |
| `exception: "ContentLengthExceededException"` | — | `message_delta` (stop_reason: max_tokens) + `message_stop` |
| 其他上游错误 | 任意非 200 | `{"type":"error","error":{"type":"overloaded_error","message":"Upstream error: ..."}}` |

### 6.2 代理层错误（HTTP JSON 格式）

| 错误场景 | HTTP 状态码 | 响应格式 |
|---|---|---|
| 模型未找到 | 404 | `{"error":{"code":"model_not_found","message":"...","type":"new_api_error"}}` |
| 请求体解析失败 | 400 | `{"error":{"message":"...","code":"bad_request"}}` |
| 认证失败 | 401 | `{"error":{"message":"...","code":"unauthorized"}}` |
| 内部错误 | 500 | `{"error":{"message":"...","code":"internal_error"}}` |

---

## 七、Token 计算规则

### 7.1 输入 Token 估算

基于 `TokenEstimator.EstimateTokens()`，在发送给上游前计算：
- 文本内容：`len(text) / 4`（字节数除以 4）
- 工具定义：名称 + 描述 + schema 的字节数之和 / 4
- 过滤掉不支持的工具（`web_search`）后再计算

### 7.2 输出 Token 计算（流式）

- `text_delta`：`len(text) / 4`，每个 delta 累加
- `input_json_delta`：累加 JSON 字节数，在 `content_block_stop` 时一次性计算 `ceil(bytes / 4)`
- `tool_use` 块结构开销：固定 12 tokens + 工具名称 tokens

### 7.3 输出 Token 计算（非流式）

- 文本块：`len(text) / 4`
- 工具调用块：`EstimateToolUseTokens(name, input)`

---

## 八、关键约束与特殊行为

| 约束 | 说明 |
|---|---|
| 工具描述长度 | 最大 10000 字符（可通过 `MAX_TOOL_DESCRIPTION_LENGTH` 环境变量配置） |
| 参数名长度 | 最大 64 字符，超出自动截断 |
| 历史消息格式 | 必须严格 user/assistant 交替，不允许孤立消息 |
| 工具结果请求 | 包含 `tool_result` 的消息，`content` 字段必须为空字符串 |
| content_block 索引 | index:0 预留给文本块，工具调用从 index:1 开始递增 |
| message_delta 唯一性 | 一次消息流中只能出现一次 `message_delta`，由 `sendFinalEvents` 统一发送 |
| 工具介绍文本 | 工具调用前自动生成空字符串介绍文本（`generateIntroText` 当前返回空字符串） |
| 孤立 assistant 消息 | 历史中无前置 user 的 assistant 消息直接忽略 |
| 空内容工具请求 | 仅含工具定义但无文本内容时，自动注入占位内容 `"执行工具任务"` |
