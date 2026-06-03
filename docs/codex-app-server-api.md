# Codex App Server 接口文档

**协议**：JSON-RPC 2.0（wire 上省略 `"jsonrpc":"2.0"`）  
**传输**：stdio（JSONL）/ WebSocket / Unix Socket  
**版本**：Codex CLI v0.36+  
**源码**：https://github.com/openai/codex/tree/main/codex-rs/app-server

---

## 目录

1. [连接与初始化](#1-连接与初始化)
2. [账号认证](#2-账号认证)
3. [Thread 会话管理](#3-thread-会话管理)
4. [Turn 轮次管理](#4-turn-轮次管理)
5. [命令执行](#5-命令执行)
6. [模型管理](#6-模型管理)
7. [MCP 服务器](#7-mcp-服务器)
8. [文件系统](#8-文件系统)
9. [服务端推送通知](#9-服务端推送通知)
10. [错误码](#10-错误码)
11. [公共数据结构](#11-公共数据结构)

---

## 传输层说明

| 传输方式 | 启动参数 | 格式 | 状态 |
|---|---|---|---|
| stdio | `--listen stdio://`（默认） | 换行分隔 JSONL | 稳定 |
| WebSocket (TCP) | `--listen ws://IP:PORT` | 每帧一条 JSON-RPC 消息 | 实验性 |
| Unix Socket | `--listen unix://` 或 `--listen unix://PATH` | WebSocket over Unix socket | 稳定 |
| 禁用 | `--listen off` | — | — |

WebSocket 模式额外提供 HTTP 健康探针：
- `GET /readyz` → `200 OK`（监听器就绪后）
- `GET /healthz` → `200 OK`（无 `Origin` 头时）
- 带 `Origin` 头的请求 → `403 Forbidden`

### WebSocket 认证

客户端在握手时通过 `Authorization: Bearer <token>` 传递凭证。

| 模式 | 参数 |
|---|---|
| Capability Token | `--ws-auth capability-token --ws-token-file /path` |
| Signed Bearer Token | `--ws-auth signed-bearer-token --ws-shared-secret-file /path` |

---

## 消息格式

```json
// 请求
{ "method": "thread/start", "id": 10, "params": { "model": "gpt-5.4" } }

// 成功响应
{ "id": 10, "result": { "thread": { "id": "thr_123" } } }

// 错误响应
{ "id": 10, "error": { "code": 123, "message": "Something went wrong" } }

// 通知（服务端主动推送，无 id）
{ "method": "turn/started", "params": { "turn": { "id": "turn_456" } } }
```

---

## 连接流程

```
Client                          Server
  |                               |
  |-- initialize (id:0) -------->|
  |<- result: serverInfo --------|
  |-- initialized (notify) ----->|
  |                               |
  |-- thread/start (id:1) ------>|
  |<- result: {thread} ----------|
  |<- thread/started (notify) ---|
  |                               |
  |-- turn/start (id:2) -------->|
  |<- result: {turn} ------------|
  |<- turn/started (notify) -----|
  |<- item/started (notify) -----|
  |<- item/agentMessage/delta ---|  (流式)
  |<- item/completed (notify) ---|
  |<- turn/completed (notify) ---|
```

---

## 1. 连接与初始化

### 1.1 initialize

**说明**：每个连接建立后必须首先调用，否则后续所有请求返回 `Not initialized` 错误。重复调用返回 `Already initialized`。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.clientInfo.name | string | ✅ | 客户端标识符，用于合规日志 |
| params.clientInfo.title | string | ✅ | 客户端展示名 |
| params.clientInfo.version | string | ✅ | 客户端版本 |
| params.capabilities.experimentalApi | boolean | ❌ | 是否启用实验性 API，默认 false |
| params.capabilities.optOutNotificationMethods | string[] | ❌ | 要屏蔽的通知方法名列表（精确匹配） |

**请求示例**

```json
{
  "method": "initialize",
  "id": 0,
  "params": {
    "clientInfo": {
      "name": "my_client",
      "title": "My Client",
      "version": "1.0.0"
    },
    "capabilities": {
      "experimentalApi": false,
      "optOutNotificationMethods": []
    }
  }
}
```

**响应参数**

| 字段 | 类型 | 说明 |
|---|---|---|
| result.userAgent | string | 服务端 User-Agent |
| result.platformFamily | string | 平台系列（如 `macos`） |
| result.platformOs | string | 操作系统 |

**响应示例**

```json
{
  "id": 0,
  "result": {
    "userAgent": "codex/0.36.0",
    "platformFamily": "macos",
    "platformOs": "macos"
  }
}
```

---

### 1.2 initialized

**说明**：`initialize` 响应后必须立即发送此通知，无需等待响应。

**方向**：Client → Server（通知，无 id）

```json
{ "method": "initialized", "params": {} }
```

---

## 2. 账号认证

### 2.1 account/read

**说明**：读取当前认证状态。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.refreshToken | boolean | ❌ | 是否强制刷新 token（仅 chatgpt 模式有效） |

**请求示例**

```json
{ "method": "account/read", "id": 1, "params": { "refreshToken": false } }
```

**响应参数**

| 字段 | 类型 | 说明 |
|---|---|---|
| result.account | object \| null | 当前账号信息，未登录时为 null |
| result.account.type | string | `"apiKey"` \| `"chatgpt"` \| `"chatgptAuthTokens"` |
| result.account.email | string | 邮箱（chatgpt 模式） |
| result.account.planType | string | `"free"` \| `"plus"` \| `"pro"` \| `"business"` |
| result.requiresOpenaiAuth | boolean | 当前 provider 是否需要 OpenAI 凭证 |

**响应示例**

```json
{
  "id": 1,
  "result": {
    "account": {
      "type": "chatgpt",
      "email": "user@example.com",
      "planType": "pro"
    },
    "requiresOpenaiAuth": true
  }
}
```

---

### 2.2 account/login/start

**说明**：发起登录流程。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.type | string | ✅ | `"apiKey"` \| `"chatgpt"` \| `"chatgptDeviceCode"` |
| params.apiKey | string | 条件必填 | type=apiKey 时必填 |

**请求示例（API Key）**

```json
{ "method": "account/login/start", "id": 2, "params": { "type": "apiKey", "apiKey": "sk-..." } }
```

**响应示例（API Key）**

```json
{ "id": 2, "result": { "type": "apiKey" } }
```

**响应示例（chatgpt 浏览器流）**

| 字段 | 类型 | 说明 |
|---|---|---|
| result.loginId | string | UUID，用于取消登录 |
| result.authUrl | string | 需在浏览器打开的授权 URL |

```json
{
  "id": 2,
  "result": {
    "type": "chatgpt",
    "loginId": "uuid-xxx",
    "authUrl": "https://chatgpt.com/...&redirect_uri=..."
  }
}
```

**响应示例（chatgptDeviceCode）**

| 字段 | 类型 | 说明 |
|---|---|---|
| result.loginId | string | UUID |
| result.verificationUrl | string | 展示给用户的验证页面 URL |
| result.userCode | string | 用户需输入的验证码，如 `"ABCD-1234"` |

```json
{
  "id": 2,
  "result": {
    "type": "chatgptDeviceCode",
    "loginId": "uuid-xxx",
    "verificationUrl": "https://auth.openai.com/codex/device",
    "userCode": "ABCD-1234"
  }
}
```

---

### 2.3 account/login/cancel

**请求示例**

```json
{ "method": "account/login/cancel", "id": 3, "params": { "loginId": "uuid-xxx" } }
```

**响应**：`{ "id": 3, "result": {} }`

---

### 2.4 account/logout

```json
{ "method": "account/logout", "id": 4 }
```

**响应**：`{ "id": 4, "result": {} }`

登出后服务端推送 `account/updated`，`authMode` 为 null。

---

### 2.5 account/rateLimits/read

**说明**：读取 ChatGPT 速率限制（仅 chatgpt 模式）。

```json
{ "method": "account/rateLimits/read", "id": 5 }
```

**响应参数**

| 字段 | 类型 | 说明 |
|---|---|---|
| result.rateLimits.limitId | string | 限制桶 ID |
| result.rateLimits.primary.usedPercent | number | 当前窗口使用百分比（0-100） |
| result.rateLimits.primary.windowDurationMins | number | 窗口时长（分钟） |
| result.rateLimits.primary.resetsAt | number | 下次重置时间（Unix 秒） |
| result.rateLimitsByLimitId | object | 多桶视图，key 为 limitId |

**响应示例**

```json
{
  "id": 5,
  "result": {
    "rateLimits": {
      "limitId": "codex",
      "primary": { "usedPercent": 25, "windowDurationMins": 15, "resetsAt": 1730947200 }
    },
    "rateLimitsByLimitId": {
      "codex": {
        "limitId": "codex",
        "primary": { "usedPercent": 25, "windowDurationMins": 15, "resetsAt": 1730947200 }
      }
    }
  }
}
```

---

## 3. Thread 会话管理

### 3.1 thread/start

**说明**：创建新会话。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.model | string | ❌ | 模型 ID，如 `"gpt-5.4"` |
| params.cwd | string | ❌ | 工作目录绝对路径 |
| params.approvalPolicy | string | ❌ | `"never"` \| `"on-request"` \| `"unlessTrusted"` |
| params.sandbox | string | ❌ | `"workspaceWrite"` \| `"readOnly"` \| `"dangerFullAccess"` |
| params.personality | string | ❌ | 人格预设名 |
| params.serviceName | string | ❌ | 集成服务名，用于指标标记 |

**请求示例**

```json
{
  "method": "thread/start",
  "id": 10,
  "params": {
    "model": "gpt-5.4",
    "cwd": "/Users/me/project",
    "approvalPolicy": "never",
    "sandbox": "workspaceWrite"
  }
}
```

**响应参数**

| 字段 | 类型 | 说明 |
|---|---|---|
| result.thread.id | string | Thread ID |
| result.thread.sessionId | string | Session 根 ID（fork 时指向根） |
| result.thread.ephemeral | boolean | 是否临时会话（不持久化） |
| result.thread.modelProvider | string | 模型提供商 |
| result.thread.createdAt | number | 创建时间（Unix 秒） |

**响应示例**

```json
{
  "id": 10,
  "result": {
    "thread": {
      "id": "thr_123",
      "sessionId": "thr_123",
      "ephemeral": false,
      "modelProvider": "openai",
      "createdAt": 1730910000
    }
  }
}
```

---

### 3.2 thread/resume

**说明**：恢复已有会话，后续 turn/start 将追加到该会话。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.threadId | string | ✅ | 要恢复的 Thread ID |
| params.personality | string | ❌ | 覆盖人格预设 |
| params.model | string | ❌ | 覆盖模型（会触发一次性切换提示） |

**请求示例**

```json
{ "method": "thread/resume", "id": 11, "params": { "threadId": "thr_123" } }
```

**响应**：同 `thread/start` 响应结构。

---

### 3.3 thread/fork

**说明**：从已有会话分叉出新会话，保留历史记录，新会话有独立 ID。

**请求示例**

```json
{ "method": "thread/fork", "id": 12, "params": { "threadId": "thr_123" } }
```

**响应示例**

```json
{
  "id": 12,
  "result": {
    "thread": {
      "id": "thr_456",
      "sessionId": "thr_123",
      "forkedFromId": "thr_123"
    }
  }
}
```

---

### 3.4 thread/list

**说明**：分页列出会话列表，默认按 `createdAt` 降序。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.cursor | string \| null | ❌ | 分页游标，首页传 null |
| params.limit | number | ❌ | 每页数量 |
| params.sortKey | string | ❌ | `"created_at"`（默认）\| `"updated_at"` |
| params.archived | boolean | ❌ | true=只返回已归档，false=未归档（默认） |
| params.cwd | string | ❌ | 按工作目录精确过滤 |
| params.searchTerm | string | ❌ | 搜索关键词 |
| params.sourceKinds | string[] | ❌ | 来源过滤，默认 `["cli","vscode"]`，可选值见下 |

`sourceKinds` 可选值：`cli` \| `vscode` \| `exec` \| `appServer` \| `subAgent` \| `subAgentReview` \| `unknown`

**请求示例**

```json
{
  "method": "thread/list",
  "id": 20,
  "params": { "cursor": null, "limit": 25, "sortKey": "created_at" }
}
```

**响应参数**

| 字段 | 类型 | 说明 |
|---|---|---|
| result.data | Thread[] | 会话列表 |
| result.nextCursor | string \| null | 下一页游标，null 表示最后一页 |

**响应示例**

```json
{
  "id": 20,
  "result": {
    "data": [
      {
        "id": "thr_a",
        "preview": "Fix failing tests",
        "name": "Test fix session",
        "ephemeral": false,
        "modelProvider": "openai",
        "createdAt": 1730831111,
        "updatedAt": 1730831111,
        "status": { "type": "notLoaded" }
      }
    ],
    "nextCursor": null
  }
}
```

---

### 3.5 thread/read

**说明**：读取会话详情，不订阅事件、不加载到内存。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.threadId | string | ✅ | Thread ID |
| params.includeTurns | boolean | ❌ | 是否包含轮次历史，默认 false |

**请求示例**

```json
{ "method": "thread/read", "id": 19, "params": { "threadId": "thr_123", "includeTurns": true } }
```

---

### 3.6 thread/archive

**说明**：将会话日志移入归档目录，归档后不出现在默认列表中。

```json
{ "method": "thread/archive", "id": 22, "params": { "threadId": "thr_123" } }
```

**响应**：`{ "id": 22, "result": {} }`，同时推送 `thread/archived` 通知。

---

### 3.7 thread/unarchive

```json
{ "method": "thread/unarchive", "id": 23, "params": { "threadId": "thr_123" } }
```

**响应**：返回恢复的 thread 对象，同时推送 `thread/unarchived` 通知。

---

### 3.8 thread/rollback

**说明**：从内存上下文中移除最近 N 轮，并在持久化日志中写入回滚标记。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.threadId | string | ✅ | Thread ID |
| params.numTurns | number | ✅ | 要回滚的轮次数 |

```json
{ "method": "thread/rollback", "id": 28, "params": { "threadId": "thr_123", "numTurns": 1 } }
```

---

### 3.9 thread/unsubscribe

**说明**：取消当前连接对该会话的事件订阅。若为最后一个订阅者，30 分钟无活动后会话自动卸载。

```json
{ "method": "thread/unsubscribe", "id": 29, "params": { "threadId": "thr_123" } }
```

**响应**

| result.status | 说明 |
|---|---|
| `"unsubscribed"` | 成功取消订阅 |
| `"notSubscribed"` | 本连接未订阅该会话 |
| `"notLoaded"` | 会话未加载 |

---

### 3.10 thread/turns/list

**说明**：分页读取会话的轮次历史，不恢复会话。默认最新优先。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.threadId | string | ✅ | Thread ID |
| params.limit | number | ❌ | 每页数量 |
| params.cursor | string | ❌ | 分页游标 |
| params.sortDirection | string | ❌ | `"desc"`（默认）\| `"asc"` |
| params.itemsView | string | ❌ | `"notLoaded"` \| `"summary"`（默认）\| `"full"` |

---

## 4. Turn 轮次管理

### 4.1 turn/start

**说明**：向会话发送用户输入，开始一轮 Agent 执行。执行过程通过通知流式推送。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.threadId | string | ✅ | 目标 Thread ID |
| params.input | InputItem[] | ✅ | 用户输入列表，见下方 InputItem 结构 |
| params.cwd | string | ❌ | 覆盖工作目录（后续轮次生效） |
| params.model | string | ❌ | 覆盖模型（后续轮次生效） |
| params.effort | string | ❌ | `"low"` \| `"medium"` \| `"high"` |
| params.approvalPolicy | string | ❌ | 覆盖审批策略 |
| params.sandboxPolicy | SandboxPolicy | ❌ | 覆盖沙箱策略，见 SandboxPolicy 结构 |
| params.summary | string | ❌ | `"concise"` \| `"detailed"` |
| params.outputSchema | object | ❌ | JSON Schema，约束最终输出结构（仅本轮有效） |

**InputItem 结构**

| type | 额外字段 | 说明 |
|---|---|---|
| `"text"` | `text: string` | 文本输入 |
| `"image"` | `url: string` | 远程图片 URL |
| `"localImage"` | `path: string` | 本地图片绝对路径 |
| `"skill"` | `name: string`, `path: string` | 调用技能，path 为 SKILL.md 绝对路径 |
| `"mention"` | `name: string`, `path: string` | 调用 App，path 格式 `app://<id>` |

**SandboxPolicy 结构**

| type | 额外字段 | 说明 |
|---|---|---|
| `"readOnly"` | `access?` | 只读沙箱 |
| `"workspaceWrite"` | `writableRoots: string[]`, `networkAccess: boolean` | 工作区写入沙箱 |
| `"dangerFullAccess"` | — | 无限制（危险） |
| `"externalSandbox"` | `networkAccess: "restricted" \| "enabled"` | 外部沙箱，跳过内置沙箱 |

**请求示例**

```json
{
  "method": "turn/start",
  "id": 30,
  "params": {
    "threadId": "thr_123",
    "input": [{ "type": "text", "text": "Run all tests and fix failures" }],
    "cwd": "/Users/me/project",
    "model": "gpt-5.4",
    "effort": "medium",
    "sandboxPolicy": {
      "type": "workspaceWrite",
      "writableRoots": ["/Users/me/project"],
      "networkAccess": false
    }
  }
}
```

**响应参数**

| 字段 | 类型 | 说明 |
|---|---|---|
| result.turn.id | string | Turn ID |
| result.turn.status | string | 初始为 `"inProgress"` |
| result.turn.items | array | 初始为空，后续通过通知推送 |
| result.turn.error | null | 初始无错误 |

**响应示例**

```json
{
  "id": 30,
  "result": {
    "turn": { "id": "turn_456", "status": "inProgress", "items": [], "error": null }
  }
}
```

---

### 4.2 turn/steer

**说明**：向进行中的轮次追加用户输入，不创建新轮次，不接受模型/沙箱等覆盖参数。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.threadId | string | ✅ | Thread ID |
| params.input | InputItem[] | ✅ | 追加的输入 |
| params.expectedTurnId | string | ✅ | 必须与当前活跃 Turn ID 一致，防止竞态 |

**请求示例**

```json
{
  "method": "turn/steer",
  "id": 32,
  "params": {
    "threadId": "thr_123",
    "input": [{ "type": "text", "text": "Focus on unit tests only." }],
    "expectedTurnId": "turn_456"
  }
}
```

**响应示例**

```json
{ "id": 32, "result": { "turnId": "turn_456" } }
```

---

### 4.3 turn/interrupt

**说明**：中断当前进行中的轮次。成功后服务端推送 `turn/completed`，`status` 为 `"interrupted"`。

**请求示例**

```json
{ "method": "turn/interrupt", "id": 31, "params": { "threadId": "thr_123", "turnId": "turn_456" } }
```

**响应**：`{ "id": 31, "result": {} }`

---

### 4.4 审批响应（客户端响应服务端请求）

当命令执行或文件变更需要用户审批时，服务端发送带 `id` 的 RPC 请求，客户端**必须**回复。

**服务端发送的命令审批请求**

```json
{
  "method": "item/commandExecution/requestApproval",
  "id": 99,
  "params": {
    "itemId": "item_xxx",
    "threadId": "thr_123",
    "turnId": "turn_456",
    "command": ["rm", "-rf", "dist"],
    "cwd": "/Users/me/project",
    "reason": "Clean build artifacts"
  }
}
```

**客户端响应（命令审批决策）**

| 决策值 | 说明 |
|---|---|
| `"accept"` | 本次允许 |
| `"acceptForSession"` | 本次会话内始终允许 |
| `"decline"` | 拒绝执行 |
| `"cancel"` | 取消整个轮次 |
| `{ "acceptWithExecpolicyAmendment": { "execpolicy_amendment": [...] } }` | 允许并添加策略规则 |

```json
{ "id": 99, "result": "accept" }
```

**服务端发送的文件变更审批请求**

```json
{
  "method": "item/fileChange/requestApproval",
  "id": 100,
  "params": {
    "itemId": "item_yyy",
    "threadId": "thr_123",
    "turnId": "turn_456",
    "reason": "Apply patch to src/index.ts"
  }
}
```

**客户端响应（文件变更审批决策）**：同命令审批，可选 `"accept"` \| `"acceptForSession"` \| `"decline"` \| `"cancel"`

审批完成后服务端推送 `serverRequest/resolved`，随后推送 `item/completed`。

---

## 5. 命令执行

### 5.1 command/exec

**说明**：在沙箱中执行单条命令，不创建 Thread/Turn，适合独立的工具调用场景。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.command | string[] | ✅ | 命令及参数数组，不能为空 |
| params.cwd | string | ❌ | 工作目录 |
| params.sandboxPolicy | SandboxPolicy | ❌ | 沙箱策略，同 turn/start |
| params.timeoutMs | number | ❌ | 超时毫秒数 |
| params.tty | boolean | ❌ | 是否使用 PTY |
| params.streamStdoutStderr | boolean | ❌ | 是否流式推送输出（触发 `command/exec/outputDelta` 通知） |

**请求示例**

```json
{
  "method": "command/exec",
  "id": 50,
  "params": {
    "command": ["git", "status", "--short"],
    "cwd": "/Users/me/project",
    "sandboxPolicy": { "type": "readOnly" },
    "timeoutMs": 5000
  }
}
```

**响应参数**

| 字段 | 类型 | 说明 |
|---|---|---|
| result.exitCode | number | 退出码，0 表示成功 |
| result.stdout | string | 标准输出内容 |
| result.stderr | string | 标准错误内容 |

**响应示例**

```json
{
  "id": 50,
  "result": { "exitCode": 0, "stdout": "M  src/index.ts\n", "stderr": "" }
}
```

---

## 6. 模型管理

### 6.1 model/list

**说明**：列出可用模型及其能力，用于渲染模型选择器。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.limit | number | ❌ | 每页数量 |
| params.cursor | string | ❌ | 分页游标 |
| params.includeHidden | boolean | ❌ | 是否包含隐藏模型，默认 false |

**请求示例**

```json
{ "method": "model/list", "id": 6, "params": { "limit": 20 } }
```

**响应参数（单条模型）**

| 字段 | 类型 | 说明 |
|---|---|---|
| id | string | 模型 ID |
| displayName | string | 展示名 |
| isDefault | boolean | 是否默认模型 |
| hidden | boolean | 是否隐藏 |
| defaultReasoningEffort | string | 默认推理强度 |
| supportedReasoningEfforts | object[] | 支持的推理强度列表，含 `reasoningEffort` 和 `description` |
| inputModalities | string[] | 支持的输入类型，如 `["text","image"]` |
| supportsPersonality | boolean | 是否支持人格预设 |
| upgrade | string | 推荐升级的模型 ID（可选） |

**响应示例**

```json
{
  "id": 6,
  "result": {
    "data": [
      {
        "id": "gpt-5.4",
        "displayName": "GPT-5.4",
        "isDefault": true,
        "hidden": false,
        "defaultReasoningEffort": "medium",
        "supportedReasoningEfforts": [
          { "reasoningEffort": "low", "description": "Lower latency" },
          { "reasoningEffort": "medium", "description": "Balanced" },
          { "reasoningEffort": "high", "description": "Best quality" }
        ],
        "inputModalities": ["text", "image"],
        "supportsPersonality": true
      }
    ],
    "nextCursor": null
  }
}
```

---

## 7. MCP 服务器

### 7.1 mcpServerStatus/list

**说明**：列出所有 MCP 服务器及其工具、认证状态。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.limit | number | ❌ | 每页数量 |
| params.cursor | string | ❌ | 分页游标 |
| params.detail | string | ❌ | `"full"` \| `"toolsAndAuthOnly"`（默认） |

**请求示例**

```json
{ "method": "mcpServerStatus/list", "id": 70, "params": { "detail": "toolsAndAuthOnly" } }
```

---

### 7.2 mcpServer/tool/call

**说明**：通过已初始化的 MCP 服务器调用工具。

**请求参数**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params.serverName | string | ✅ | MCP 服务器名称 |
| params.toolName | string | ✅ | 工具名称 |
| params.arguments | object | ✅ | 工具参数 |

**请求示例**

```json
{
  "method": "mcpServer/tool/call",
  "id": 71,
  "params": {
    "serverName": "github",
    "toolName": "search_repositories",
    "arguments": { "query": "openai codex" }
  }
}
```

---

### 7.3 mcpServer/oauth/login

**说明**：为 HTTP 类型 MCP 服务器发起 OAuth 登录，返回授权 URL。

```json
{ "method": "mcpServer/oauth/login", "id": 72, "params": { "serverName": "github" } }
```

**响应**：`{ "result": { "authorizationUrl": "https://..." } }`

登录完成后服务端推送 `mcpServer/oauthLogin/completed`：`{ name, success, error? }`

---

### 7.4 config/mcpServer/reload

**说明**：从磁盘重新加载 MCP 服务器配置，并为已加载的会话排队刷新。

```json
{ "method": "config/mcpServer/reload", "id": 73, "params": {} }
```

---

## 8. 文件系统

所有路径均为绝对路径。

### 8.1 fs/readFile

```json
{ "method": "fs/readFile", "id": 80, "params": { "path": "/Users/me/project/README.md" } }
```

### 8.2 fs/writeFile

```json
{ "method": "fs/writeFile", "id": 81, "params": { "path": "/tmp/out.txt", "content": "hello" } }
```

### 8.3 fs/readDirectory

```json
{ "method": "fs/readDirectory", "id": 82, "params": { "path": "/Users/me/project" } }
```

### 8.4 fs/getMetadata

```json
{ "method": "fs/getMetadata", "id": 83, "params": { "path": "/Users/me/project/src" } }
```

### 8.5 fs/createDirectory

```json
{ "method": "fs/createDirectory", "id": 84, "params": { "path": "/Users/me/project/new-dir" } }
```

### 8.6 fs/remove

```json
{ "method": "fs/remove", "id": 85, "params": { "path": "/Users/me/project/old-file.txt" } }
```

### 8.7 fs/copy

```json
{ "method": "fs/copy", "id": 86, "params": { "src": "/Users/me/a.txt", "dest": "/Users/me/b.txt" } }
```

### 8.8 fs/watch / fs/unwatch

**说明**：监听文件或目录变更，变更时推送 `fs/changed` 通知。

```json
{ "method": "fs/watch", "id": 87, "params": { "watchId": "watch-001", "path": "/Users/me/project/.git/HEAD" } }
{ "method": "fs/unwatch", "id": 88, "params": { "watchId": "watch-001" } }
```

**变更通知**

```json
{ "method": "fs/changed", "params": { "watchId": "watch-001", "changedPaths": ["/Users/me/project/.git/HEAD"] } }
```

---

## 9. 服务端推送通知

所有通知均无 `id` 字段，由服务端主动推送，客户端只需监听不需回复（审批请求除外）。

### 9.1 认证通知

| 方法 | 参数 | 说明 |
|---|---|---|
| `account/login/completed` | `{ loginId, success, error }` | 登录流程完成 |
| `account/updated` | `{ authMode, planType }` | 认证状态变更，`authMode` 为 `"apikey"` \| `"chatgpt"` \| null |
| `account/rateLimits/updated` | `{ rateLimits }` | 速率限制更新 |

### 9.2 Thread 通知

| 方法 | 参数 | 说明 |
|---|---|---|
| `thread/started` | `{ thread }` | 会话创建或恢复 |
| `thread/archived` | `{ threadId }` | 会话已归档 |
| `thread/unarchived` | `{ threadId }` | 会话已取消归档 |
| `thread/closed` | `{ threadId }` | 会话已从内存卸载 |
| `thread/status/changed` | `{ threadId, status }` | 会话运行状态变更 |
| `thread/tokenUsage/updated` | `{ threadId, usage }` | Token 用量更新 |

`thread.status.type` 可选值：

| 值 | 说明 |
|---|---|
| `"notLoaded"` | 未加载到内存 |
| `"idle"` | 空闲 |
| `"active"` | 活跃中，含 `activeFlags`（如 `["waitingOnApproval"]`） |
| `"systemError"` | 系统错误 |

### 9.3 Turn 通知

| 方法 | 参数 | 说明 |
|---|---|---|
| `turn/started` | `{ turn }` | 轮次开始，`status: "inProgress"` |
| `turn/completed` | `{ turn }` | 轮次结束 |
| `turn/diff/updated` | `{ threadId, turnId, diff }` | 本轮所有文件变更的聚合 unified diff |
| `turn/plan/updated` | `{ turnId, explanation?, plan[] }` | Agent 计划更新 |

`turn.status` 可选值：`"completed"` \| `"interrupted"` \| `"failed"`

失败时 `turn.error` 结构：

```json
{
  "message": "Context window exceeded",
  "codexErrorInfo": "ContextWindowExceeded",
  "additionalDetails": "..."
}
```

### 9.4 Item 通知

| 方法 | 参数 | 说明 |
|---|---|---|
| `item/started` | `{ item }` | 工作单元开始，含完整 item 对象 |
| `item/completed` | `{ item }` | 工作单元完成，**以此为最终状态** |
| `item/agentMessage/delta` | `{ itemId, delta }` | Agent 消息流式文本片段 |
| `item/commandExecution/outputDelta` | `{ itemId, delta }` | 命令 stdout/stderr 流，按序追加 |
| `item/reasoning/summaryTextDelta` | `{ itemId, delta, summaryIndex }` | 推理摘要流 |
| `item/plan/delta` | `{ itemId, delta }` | 计划文本流 |

**Item 类型一览**

| type | 关键字段 | 说明 |
|---|---|---|
| `userMessage` | `id`, `content[]` | 用户消息 |
| `agentMessage` | `id`, `text`, `phase?` | Agent 回复，`phase` 为 `"commentary"` \| `"final_answer"` |
| `commandExecution` | `id`, `command`, `cwd`, `status`, `exitCode?`, `aggregatedOutput?` | 命令执行 |
| `fileChange` | `id`, `changes[]`, `status` | 文件变更，`changes[].kind` 为 `create` \| `modify` \| `delete` |
| `mcpToolCall` | `id`, `server`, `tool`, `arguments`, `result?`, `error?` | MCP 工具调用 |
| `webSearch` | `id`, `query`, `action?` | 网络搜索 |
| `reasoning` | `id`, `summary`, `content` | 推理过程 |
| `contextCompaction` | `id` | 上下文压缩 |
| `enteredReviewMode` | `id`, `review` | 进入 Review 模式 |
| `exitedReviewMode` | `id`, `review` | 退出 Review 模式，含最终 review 文本 |

### 9.5 其他通知

| 方法 | 参数 | 说明 |
|---|---|---|
| `serverRequest/resolved` | `{ threadId, requestId }` | 审批请求已处理或被清除 |
| `mcpServer/oauthLogin/completed` | `{ name, success, error? }` | MCP OAuth 登录完成 |
| `mcpServer/startupStatus/updated` | `{ name, status, error }` | MCP 服务器启动状态变更 |
| `skills/changed` | — | 本地技能文件变更，需重新调用 `skills/list` |
| `fs/changed` | `{ watchId, changedPaths[] }` | 被监听的文件/目录发生变更 |
| `command/exec/outputDelta` | `{ processId, stream, deltaBase64 }` | 流式命令输出（base64 编码） |

---

## 10. 错误码

### JSON-RPC 错误

| code | 说明 | 处理建议 |
|---|---|---|
| `-32001` | 服务器过载 `"Server overloaded; retry later."` | 指数退避重试 |

### codexErrorInfo 枚举

| 值 | 说明 |
|---|---|
| `ContextWindowExceeded` | 上下文超出模型窗口限制 |
| `UsageLimitExceeded` | 用量超限 |
| `HttpConnectionFailed` | 上游 HTTP 4xx/5xx，含 `httpStatusCode` |
| `ResponseStreamConnectionFailed` | 响应流连接失败 |
| `ResponseStreamDisconnected` | 响应流中断 |
| `ResponseTooManyFailedAttempts` | 重试次数过多 |
| `BadRequest` | 请求参数错误 |
| `Unauthorized` | 未授权（401） |
| `SandboxError` | 沙箱执行错误 |
| `InternalServerError` | 内部服务器错误 |
| `Other` | 其他错误 |

---

## 11. 公共数据结构

### Thread 对象

| 字段 | 类型 | 说明 |
|---|---|---|
| id | string | Thread ID |
| sessionId | string | Session 根 ID |
| name | string \| null | 用户设置的会话名称 |
| preview | string | 会话预览文本 |
| ephemeral | boolean | 是否临时会话 |
| modelProvider | string | 模型提供商 |
| createdAt | number | 创建时间（Unix 秒） |
| updatedAt | number | 更新时间（Unix 秒） |
| status | ThreadStatus | 运行状态 |
| forkedFromId | string \| null | 分叉来源 Thread ID |

### SandboxPolicy 对象

```json
// 只读
{ "type": "readOnly" }

// 工作区写入
{
  "type": "workspaceWrite",
  "writableRoots": ["/Users/me/project"],
  "networkAccess": false
}

// 完全访问（危险）
{ "type": "dangerFullAccess" }

// 外部沙箱
{ "type": "externalSandbox", "networkAccess": "restricted" }
```

### InputItem 对象

```json
{ "type": "text", "text": "Fix the bug in src/index.ts" }
{ "type": "image", "url": "https://example.com/screenshot.png" }
{ "type": "localImage", "path": "/tmp/screenshot.png" }
{ "type": "skill", "name": "skill-creator", "path": "/Users/me/.codex/skills/skill-creator/SKILL.md" }
{ "type": "mention", "name": "GitHub", "path": "app://github" }
```

---

## 附录：Node.js 快速接入示例

```typescript
import { spawn } from "node:child_process";
import readline from "node:readline";

const proc = spawn("codex", ["app-server"], {
  stdio: ["pipe", "pipe", "inherit"],
});
const rl = readline.createInterface({ input: proc.stdout });

const send = (msg: unknown) => proc.stdin.write(JSON.stringify(msg) + "\n");

let threadId: string | null = null;

rl.on("line", (line) => {
  const msg = JSON.parse(line) as any;

  // initialize 完成后启动会话
  if (msg.id === 0 && msg.result) {
    send({ method: "initialized", params: {} });
    send({ method: "thread/start", id: 1, params: { model: "gpt-5.4" } });
  }

  // 会话创建后发起对话
  if (msg.id === 1 && msg.result?.thread?.id) {
    threadId = msg.result.thread.id;
    send({
      method: "turn/start",
      id: 2,
      params: {
        threadId,
        input: [{ type: "text", text: "Hello, summarize this repo." }],
      },
    });
  }

  // 处理流式 Agent 消息
  if (msg.method === "item/agentMessage/delta") {
    process.stdout.write(msg.params.delta);
  }

  // 轮次完成
  if (msg.method === "turn/completed") {
    console.log("\n[Done]", msg.params.turn.status);
    proc.kill();
  }
});

// 发起初始化
send({
  method: "initialize",
  id: 0,
  params: {
    clientInfo: { name: "my_client", title: "My Client", version: "1.0.0" },
  },
});
```

---

*文档生成时间：2026-05-30 | 数据来源：https://developers.openai.com/codex/app-server*
