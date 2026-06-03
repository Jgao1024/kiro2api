# Anthropic Messages API 接口文档

---

## 基本信息

| 项目 | 内容 |
|------|------|
| 接口地址 | `POST https://api.anthropic.com/v1/messages` |
| 认证方式 | Header: `x-api-key: <ANTHROPIC_API_KEY>` |
| Content-Type | `application/json` |
| 响应方式 | SSE 流式（`text/event-stream`）|
| API 版本 | `anthropic-version: 2023-06-01` |

---

## 请求 Headers

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `x-api-key` | string | ✅ | Anthropic API Key，格式 `sk-ant-api03-...` |
| `anthropic-version` | string | ✅ | API 版本，固定 `2023-06-01` |
| `content-type` | string | ✅ | 固定 `application/json` |
| `anthropic-beta` | string | ❌ | 逗号分隔的 Beta 功能列表，见下表 |
| `x-session-affinity` | string | ❌ | 会话亲和 ID，用于路由到同一后端节点（部分代理支持）|

**`anthropic-beta` 可选值：**

| 值 | 说明 |
|----|------|
| `fine-grained-tool-streaming-2025-05-14` | 工具调用细粒度流式输出，有 tools 时建议开启 |
| `interleaved-thinking-2025-05-14` | 旧模型交错思考，Opus 4.6+ 不需要 |

---

## 请求体（Request Body）

### 顶层结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | ✅ | 模型 ID，如 `claude-opus-4-8`、`claude-sonnet-4-6` |
| `messages` | Message[] | ✅ | 对话消息列表，见 [Message 结构](#message-结构) |
| `max_tokens` | number | ✅ | 最大输出 token 数，如 `8192` |
| `stream` | boolean | ✅ | 固定 `true`（流式模式）|
| `system` | SystemBlock[] | ❌ | 系统提示，见 [SystemBlock 结构](#systemblock-结构) |
| `tools` | Tool[] | ❌ | 工具定义列表，见 [Tool 结构](#tool-结构) |
| `tool_choice` | ToolChoice | ❌ | 工具选择策略，见 [ToolChoice 结构](#toolchoice-结构) |
| `thinking` | ThinkingConfig | ❌ | 思考模式配置，见 [ThinkingConfig 结构](#thinkingconfig-结构) |
| `output_config` | OutputConfig | ❌ | 输出配置（adaptive thinking 时用）|
| `temperature` | number | ❌ | 温度，范围 `0~1`，**开启 thinking 时不可用** |
| `metadata` | Metadata | ❌ | 元数据，目前仅支持 `user_id` |

---

### Message 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | ✅ | `"user"` 或 `"assistant"` |
| `content` | string \| ContentBlock[] | ✅ | 消息内容，纯文本或内容块数组 |

**content 为数组时，ContentBlock 类型：**

#### TextBlock

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"text"` |
| `text` | string | ✅ | 文本内容 |
| `cache_control` | CacheControl | ❌ | 缓存控制，见 [CacheControl 结构](#cachecontrol-结构) |

#### ImageBlock

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"image"` |
| `source` | ImageSource | ✅ | 图片来源 |
| `source.type` | string | ✅ | 固定 `"base64"` |
| `source.media_type` | string | ✅ | `"image/jpeg"` \| `"image/png"` \| `"image/gif"` \| `"image/webp"` |
| `source.data` | string | ✅ | Base64 编码的图片数据 |

#### ToolUseBlock（assistant 消息中）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"tool_use"` |
| `id` | string | ✅ | 工具调用 ID，格式 `toolu_xxx`，仅含 `[a-zA-Z0-9_-]`，最长 64 字符 |
| `name` | string | ✅ | 工具名称 |
| `input` | object | ✅ | 工具调用参数 |

#### ToolResultBlock（user 消息中，紧跟 assistant 工具调用之后）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"tool_result"` |
| `tool_use_id` | string | ✅ | 对应的工具调用 ID |
| `content` | string \| ContentBlock[] | ✅ | 工具执行结果 |
| `is_error` | boolean | ❌ | 是否为错误结果，默认 `false` |
| `cache_control` | CacheControl | ❌ | 缓存控制 |

#### ThinkingBlock（assistant 消息中，多轮对话时回传）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"thinking"` |
| `thinking` | string | ✅ | 思考内容文本 |
| `signature` | string | ✅ | Anthropic 签名，多轮对话必须原样回传 |

#### RedactedThinkingBlock（被屏蔽的思考内容）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"redacted_thinking"` |
| `data` | string | ✅ | 加密的不透明数据，多轮对话必须原样回传 |

---

### SystemBlock 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"text"` |
| `text` | string | ✅ | 系统提示内容 |
| `cache_control` | CacheControl | ❌ | 缓存控制，建议对长系统提示开启 |

---

### Tool 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 工具名称，仅含 `[a-zA-Z0-9_-]` |
| `description` | string | ✅ | 工具描述，模型依据此决定是否调用 |
| `input_schema` | object | ✅ | JSON Schema 格式的参数定义 |
| `input_schema.type` | string | ✅ | 固定 `"object"` |
| `input_schema.properties` | object | ✅ | 参数属性定义 |
| `input_schema.required` | string[] | ❌ | 必填参数名列表 |
| `eager_input_streaming` | boolean | ❌ | 开启细粒度工具参数流式，需配合 beta header |
| `cache_control` | CacheControl | ❌ | 建议只在最后一个 tool 上加，缓存整个 tools 列表 |

---

### ToolChoice 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | `"auto"` 模型自行决定 \| `"any"` 强制调用某个工具 \| `"none"` 禁止调用 \| `"tool"` 指定工具 |
| `name` | string | 条件必填 | `type="tool"` 时必填，指定工具名 |

---

### ThinkingConfig 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | `"adaptive"` 自适应（Opus 4.6+）\| `"enabled"` 预算模式（旧模型）\| `"disabled"` 关闭 |
| `display` | string | ❌ | `"summarized"` 返回思考摘要（默认）\| `"omitted"` 不返回思考内容但保留签名 |
| `budget_tokens` | number | 条件必填 | `type="enabled"` 时必填，思考 token 预算，如 `1024` |

---

### OutputConfig 结构（adaptive thinking 时）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `effort` | string | ❌ | `"low"` \| `"medium"` \| `"high"` \| `"xhigh"` \| `"max"`，控制思考深度 |

---

### CacheControl 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 固定 `"ephemeral"` |
| `ttl` | string | ❌ | `"1h"` 长缓存（需模型支持），默认 5 分钟 |

---

### Metadata 结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | string | ❌ | 用户标识，用于滥用检测 |

---

## 完整请求示例

```json
POST https://api.anthropic.com/v1/messages
Content-Type: application/json
x-api-key: sk-ant-api03-xxxxxxxx
anthropic-version: 2023-06-01
anthropic-beta: fine-grained-tool-streaming-2025-05-14

{
  "model": "claude-opus-4-8",
  "max_tokens": 8192,
  "stream": true,
  "system": [
    {
      "type": "text",
      "text": "你是一个专业的代码助手。",
      "cache_control": { "type": "ephemeral" }
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": "帮我查一下当前目录有哪些文件"
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "text",
          "text": "我来帮你查看当前目录的文件。"
        },
        {
          "type": "tool_use",
          "id": "toolu_01ABC123",
          "name": "Bash",
          "input": { "command": "ls -la" }
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "tool_result",
          "tool_use_id": "toolu_01ABC123",
          "content": "total 48\ndrwxr-xr-x  8 user staff  256 May 30 21:00 .\n-rw-r--r--  1 user staff 1234 May 30 20:00 README.md",
          "is_error": false,
          "cache_control": { "type": "ephemeral" }
        }
      ]
    }
  ],
  "tools": [
    {
      "name": "Bash",
      "description": "在 shell 中执行命令",
      "input_schema": {
        "type": "object",
        "properties": {
          "command": {
            "type": "string",
            "description": "要执行的 shell 命令"
          }
        },
        "required": ["command"]
      },
      "eager_input_streaming": true,
      "cache_control": { "type": "ephemeral" }
    }
  ],
  "thinking": {
    "type": "adaptive",
    "display": "summarized"
  },
  "output_config": {
    "effort": "high"
  },
  "metadata": {
    "user_id": "user-abc123"
  }
}
```

---

## 响应（SSE 流式）

响应为 `text/event-stream`，每行格式为：

```
event: <event_type>
data: <json_payload>

```

### SSE 事件类型总览

| 事件名 | 触发时机 | 说明 |
|--------|----------|------|
| `message_start` | 流开始 | 包含消息 ID 和初始 token 用量 |
| `content_block_start` | 每个内容块开始 | 标识 text / thinking / tool_use 块开始 |
| `content_block_delta` | 内容块增量 | 流式传输文本、思考、工具参数 |
| `content_block_stop` | 每个内容块结束 | 标识内容块完成 |
| `message_delta` | 消息结束前 | 包含 stop_reason 和最终 token 用量 |
| `message_stop` | 流结束 | 整个响应完成 |
| `error` | 出错时 | 包含错误信息 |

---

### message_start

```json
event: message_start
data: {
  "type": "message_start",
  "message": {
    "id": "msg_01XFDUDYJgAACzvnptvVoYEL",
    "type": "message",
    "role": "assistant",
    "content": [],
    "model": "claude-opus-4-8",
    "stop_reason": null,
    "stop_sequence": null,
    "usage": {
      "input_tokens": 512,
      "output_tokens": 0,
      "cache_read_input_tokens": 256,
      "cache_creation_input_tokens": 0
    }
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `message.id` | string | 消息唯一 ID，格式 `msg_xxx` |
| `message.usage.input_tokens` | number | 输入 token 数（不含缓存命中部分）|
| `message.usage.cache_read_input_tokens` | number | 从缓存读取的 token 数（费用更低）|
| `message.usage.cache_creation_input_tokens` | number | 写入缓存的 token 数 |

---

### content_block_start

**text 块：**
```json
event: content_block_start
data: {
  "type": "content_block_start",
  "index": 0,
  "content_block": { "type": "text", "text": "" }
}
```

**thinking 块：**
```json
event: content_block_start
data: {
  "type": "content_block_start",
  "index": 0,
  "content_block": { "type": "thinking", "thinking": "" }
}
```

**tool_use 块：**
```json
event: content_block_start
data: {
  "type": "content_block_start",
  "index": 1,
  "content_block": {
    "type": "tool_use",
    "id": "toolu_01ABC123",
    "name": "Bash",
    "input": {}
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | number | 内容块在 content 数组中的位置，用于后续 delta/stop 事件定位 |
| `content_block.type` | string | `"text"` \| `"thinking"` \| `"tool_use"` \| `"redacted_thinking"` |
| `content_block.id` | string | tool_use 时的调用 ID |

---

### content_block_delta

**text_delta：**
```json
event: content_block_delta
data: {
  "type": "content_block_delta",
  "index": 0,
  "delta": { "type": "text_delta", "text": "当前目录" }
}
```

**thinking_delta：**
```json
event: content_block_delta
data: {
  "type": "content_block_delta",
  "index": 0,
  "delta": { "type": "thinking_delta", "thinking": "用户想查看文件列表..." }
}
```

**input_json_delta（工具参数流式）：**
```json
event: content_block_delta
data: {
  "type": "content_block_delta",
  "index": 1,
  "delta": { "type": "input_json_delta", "partial_json": "{\"command\": \"ls" }
}
```

**signature_delta（thinking 签名）：**
```json
event: content_block_delta
data: {
  "type": "content_block_delta",
  "index": 0,
  "delta": { "type": "signature_delta", "signature": "EqoBCkgIARAAGAIiQL..." }
}
```

| delta.type | 说明 |
|------------|------|
| `text_delta` | 文本增量，拼接 `delta.text` |
| `thinking_delta` | 思考内容增量，拼接 `delta.thinking` |
| `input_json_delta` | 工具参数 JSON 增量，拼接 `delta.partial_json` 后解析 |
| `signature_delta` | thinking 签名增量，多轮对话时必须原样回传 |

---

### content_block_stop

```json
event: content_block_stop
data: {
  "type": "content_block_stop",
  "index": 0
}
```

收到此事件后，对应 `index` 的内容块已完整，可进行最终处理（如解析完整 JSON 工具参数）。

---

### message_delta

```json
event: message_delta
data: {
  "type": "message_delta",
  "delta": {
    "stop_reason": "tool_use",
    "stop_sequence": null
  },
  "usage": {
    "input_tokens": null,
    "output_tokens": 128,
    "cache_read_input_tokens": null,
    "cache_creation_input_tokens": null
  }
}
```

| `stop_reason` 值 | 说明 |
|------------------|------|
| `end_turn` | 正常结束 |
| `max_tokens` | 达到 max_tokens 限制 |
| `tool_use` | 模型请求调用工具 |
| `stop_sequence` | 命中停止序列 |
| `pause_turn` | 暂停（可继续提交）|
| `refusal` | 模型拒绝回答 |

> **注意**：`message_delta` 中的 `usage` 字段某些代理可能返回 `null`，应以 `message_start` 中的值为准，仅在非 null 时覆盖。

---

### message_stop

```json
event: message_stop
data: { "type": "message_stop" }
```

---

## 完整响应流示例

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_01XFDUDYJgAACzvnptvVoYEL","type":"message","role":"assistant","content":[],"model":"claude-opus-4-8","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":512,"output_tokens":0,"cache_read_input_tokens":256,"cache_creation_input_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"用户想查看目录文件，我已经执行了 ls -la 命令并拿到了结果，现在整理输出。"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"EqoBCkgIARAAGAIiQL3x..."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"当前目录包含以下文件：\n\n"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"- `README.md` — 项目说明文档"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":null,"output_tokens":128,"cache_read_input_tokens":null,"cache_creation_input_tokens":null}}

event: message_stop
data: {"type":"message_stop"}
```

---

## 开发注意事项

1. **SSE 解析**：空行 `\n\n` 为事件分隔符，`\r\n` 和 `\r` 也需处理。
2. **index 定位**：delta/stop 事件用 `index` 字段定位内容块，不能假设顺序连续。
3. **工具参数**：`input_json_delta` 是增量 JSON 片段，需拼接后再解析，流式过程中可用容错 JSON 解析器实时预览。
4. **thinking 签名**：多轮对话时，assistant 消息中的 `thinking` 块必须连同 `signature` 原样回传，否则 API 报错。
5. **usage 合并策略**：`message_start` 提供 input tokens，`message_delta` 提供 output tokens，两者都可能为 null（代理场景），需分别判断后合并。
6. **cache_control 位置**：只在 system 最后一块、最后一条 user message 的最后一个 block、tools 列表最后一个 tool 上加，避免浪费缓存写入配额。
7. **temperature 与 thinking 互斥**：开启 `thinking` 时不能设置 `temperature`，否则 API 报错。
