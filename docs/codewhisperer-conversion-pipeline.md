# CodeWhisperer 请求转换流程

本文档描述 `converter.BuildCodeWhispererRequest` 将 `AnthropicRequest` 转换为 `CodeWhispererRequest` 的完整处理过程。

## 入口

`converter.BuildCodeWhispererRequest()` — `/converter/codewhisperer.go`

由 `server/common.go` 的 `buildCodeWhispererRequest()` 调用，后者负责组装 HTTP 请求、设置 headers、记录日志和 record。

---

## 转换阶段

### 1. Session 初始化

| 字段 | 值 |
|------|-----|
| `ConversationId` | 基于请求上下文生成的稳定 ID |
| `AgentContinuationId` | 同上，用于 agent 连续性 |
| `AgentTaskType` | 固定为 `"vibe"` |
| `ChatTriggerType` | 有工具且 `tool_choice` 为 `any`/`tool` → `"AUTO"`，否则 `"MANUAL"` |

### 2. 当前消息处理（最后一条 message）

`processMessageContent()` 解析内容块：

- `text` → 文本字符串
- `image` → `CodeWhispererImage`（含格式校验）
- `tool_result` → 提取工具结果
- `image_url` → 转换为 image 格式

`extractToolResultsFromMessage()` 提取 `tool_result` 块：

- 归一化为 `ToolResult` 结构体
- 根据 `is_error` 字段设置状态为 `"success"` 或 `"error"`

### 3. 工具处理

- 过滤掉 `web_search` / `websearch`（静默跳过）
- 描述超过 `MaxToolDescriptionLength`（10000 字符）时截断
- 清理不支持的 schema 字段：`additionalProperties`、`strict`、`$schema` 等
- 参数名超 64 字符时截断
- 最终放入 `UserInputMessageContext.Tools`

### 4. 历史消息构建

CodeWhisperer 要求严格的 user/assistant 交替格式，而 Anthropic 允许连续 user 消息，因此需要合并和补全。

**System messages：**

```
所有 system 消息 → 合并为单条 HistoryUserMessage
                 → 配对一条内容为 "OK" 的 HistoryAssistantMessage
```

**普通消息配对逻辑：**

```
连续 user messages → 合并到 buffer
遇到 assistant message → buffer 中的 user 消息合并，与 assistant 配对

边界处理：
  最后一条是 assistant → 纳入 history
  最后一条是 user     → 作为 currentMessage（不进 history）
  孤立 assistant（无前置 user）→ 忽略
  末尾孤立 user       → 自动配对内容为 "OK" 的 assistant
  user 消息含 tool_result → content 置为空字符串
```

### 5. 最终校验

`validateCodeWhispererRequest()` 检查：

- `ModelId`、`ConversationId` 不能为空
- content + images + tools + tool_results 不能全为空
- content 为空但有 tools 时，注入占位符 `"执行工具任务"`

---

## 数据流向

```
AnthropicRequest
  ├── Model          → config.ModelMap 映射        → ModelId
  ├── Messages[-1]   → processMessageContent()     → Content + Images
  ├── Messages[-1]   → extractToolResults()        → ToolResults[]
  ├── Tools          → validateAndProcessTools()   → CW Tools[]
  ├── System         → GetMessageContent()         → History[0] (user/"OK" pair)
  └── Messages[0..]  → 消息配对逻辑                → History[]

↓

CodeWhispererRequest.ConversationState
  ├── AgentContinuationId
  ├── AgentTaskType: "vibe"
  ├── ChatTriggerType: "AUTO" | "MANUAL"
  ├── ConversationId
  ├── CurrentMessage.UserInputMessage
  │     ├── Content
  │     ├── Images[]
  │     ├── ModelId
  │     ├── Origin: "AI_EDITOR"
  │     └── UserInputMessageContext
  │           ├── Tools[]
  │           └── ToolResults[]
  └── History[]
        ├── HistoryUserMessage
        └── HistoryAssistantMessage  (严格交替)
```

---

## 关键文件

| 文件 | 职责 |
|------|------|
| `converter/codewhisperer.go` | 主转换逻辑、消息配对、工具结果提取 |
| `converter/content.go` | 内容块解析（text/image/tool_result） |
| `converter/tools.go` | 工具校验、schema 清理、tool_choice 转换 |
| `server/common.go` | HTTP 请求组装、headers、record 记录 |
| `utils/message.go` | 消息内容提取工具函数 |
| `utils/image.go` | 图片格式校验与转换 |
| `types/codewhisperer.go` | CodeWhisperer 数据结构定义 |
| `types/anthropic.go` | Anthropic 数据结构定义 |
