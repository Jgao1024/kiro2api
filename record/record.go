package record

import (
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// Phase 记录阶段
type Phase string

const (
	PhaseInboundRaw     Phase = "inbound_raw"     // 调用方原始请求/响应
	PhaseInboundMapped  Phase = "inbound_mapped"  // 转换为 CW 格式后的请求
	PhaseOutboundRaw    Phase = "outbound_raw"    // CW 原始响应
	PhaseOutboundMapped Phase = "outbound_mapped" // 转换回调用方格式后的响应
)

// Direction 方向
type Direction string

const (
	DirectionRequest  Direction = "request"
	DirectionResponse Direction = "response"
)

// Entry 一条记录
type Entry struct {
	RequestID       string
	Phase           Phase
	Direction       Direction
	URL             string
	Model           string
	Headers         map[string]string
	Body            string
	EstimatedTokens int

	// 仅 response
	StatusCode      int
	CreditsUsed     float64
	ContextUsagePct float64 // CW 返回的上下文使用百分比

	// 对话 & 工具
	ConversationID string
	ToolCallCount  int
	ToolNames      []string
}

var (
	db            *sql.DB
	once          sync.Once
	mu            sync.Mutex
	enabledPhases map[Phase]bool
)

const dbPath = "requests.db"

const schema = `
CREATE TABLE IF NOT EXISTS request_logs (
	id               INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
	request_id       TEXT,
	phase            TEXT,
	direction        TEXT,
	url              TEXT,
	model            TEXT,
	headers          TEXT,
	body             TEXT,
	estimated_tokens INTEGER,
	status_code      INTEGER,
	credits_used     REAL,
	context_usage_pct REAL,
	conversation_id  TEXT,
	tool_call_count  INTEGER,
	tool_names       TEXT
);
CREATE INDEX IF NOT EXISTS idx_request_id ON request_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_created_at ON request_logs(created_at);
`

// Init 初始化数据库，path 为空时使用默认路径
func Init(path string) error {
	// 解析 RECORD_PHASES 环境变量
	enabledPhases = make(map[Phase]bool)
	if raw := os.Getenv("RECORD_PHASES"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				enabledPhases[Phase(p)] = true
			}
		}
	}
	// 没有配置任何 phase，跳过数据库初始化
	if len(enabledPhases) == 0 {
		return nil
	}

	var initErr error
	once.Do(func() {
		if path == "" {
			path = dbPath
		}
		// 支持通过环境变量覆盖
		if envPath := os.Getenv("RECORD_DB_PATH"); envPath != "" {
			path = envPath
		}
		var d *sql.DB
		d, initErr = sql.Open("sqlite", path)
		if initErr != nil {
			return
		}
		d.SetMaxOpenConns(1) // SQLite 单写
		_, initErr = d.Exec(schema)
		if initErr != nil {
			return
		}
		db = d
	})
	return initErr
}

// Save 异步写入一条记录（不阻塞请求链路）
func Save(e Entry) {
	if db == nil || !enabledPhases[e.Phase] {
		return
	}
	go func() { saveOne(e) }()
}

// SaveBatch 异步顺序写入多条记录，保证写入顺序
func SaveBatch(entries ...Entry) {
	filtered := entries[:0]
	for _, e := range entries {
		if db != nil && enabledPhases[e.Phase] {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		return
	}
	go func() {
		for _, e := range filtered {
			saveOne(e)
		}
	}()
}

func saveOne(e Entry) {
	headersJSON, _ := json.Marshal(e.Headers)
	toolNamesJSON, _ := json.Marshal(e.ToolNames)
	mu.Lock()
	defer mu.Unlock()
	_, _ = db.Exec(`
		INSERT INTO request_logs
			(request_id, phase, direction, url, model, headers, body,
			 estimated_tokens, status_code, credits_used, context_usage_pct,
			 conversation_id, tool_call_count, tool_names)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.RequestID, string(e.Phase), string(e.Direction),
		e.URL, e.Model, string(headersJSON), e.Body,
		e.EstimatedTokens, e.StatusCode, e.CreditsUsed, e.ContextUsagePct,
		e.ConversationID, e.ToolCallCount, string(toolNamesJSON),
	)
}

// LogRow 列表行（不含 body）
type LogRow struct {
	ID              int64   `json:"id"`
	CreatedAt       string  `json:"created_at"`
	RequestID       string  `json:"request_id"`
	Phase           string  `json:"phase"`
	Direction       string  `json:"direction"`
	URL             string  `json:"url"`
	Model           string  `json:"model"`
	EstimatedTokens int     `json:"estimated_tokens"`
	StatusCode      int     `json:"status_code"`
	CreditsUsed     float64 `json:"credits_used"`
	ContextUsagePct float64 `json:"context_usage_pct"`
	ConversationID  string  `json:"conversation_id"`
	ToolCallCount   int     `json:"tool_call_count"`
	ToolNames       string  `json:"tool_names"`
}

// QueryLogs 分页查询（不含 body/headers），phase/requestID 为空时查全部
func QueryLogs(page, size int, phase, requestID string) ([]LogRow, int64, error) {
	if db == nil {
		return nil, 0, nil
	}
	if size <= 0 {
		size = 20
	}
	offset := (page - 1) * size

	// 构建 WHERE 子句
	var conds []string
	var args []any
	if phase != "" {
		conds = append(conds, "phase=?")
		args = append(args, phase)
	}
	if requestID != "" {
		conds = append(conds, "request_id=?")
		args = append(args, requestID)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM request_logs`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`SELECT id, created_at, request_id, phase, direction, url, model,
		       estimated_tokens, status_code, credits_used, context_usage_pct,
		       conversation_id, tool_call_count, tool_names
		FROM request_logs`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, size, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []LogRow
	for rows.Next() {
		var r LogRow
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.RequestID, &r.Phase, &r.Direction,
			&r.URL, &r.Model, &r.EstimatedTokens, &r.StatusCode, &r.CreditsUsed,
			&r.ContextUsagePct, &r.ConversationID, &r.ToolCallCount, &r.ToolNames); err != nil {
			return nil, 0, err
		}
		list = append(list, r)
	}
	return list, total, nil
}

// GetBody 获取单条记录的 body 和 headers
func GetBody(id int64) (headers, body string, err error) {
	if db == nil {
		return "", "", nil
	}
	err = db.QueryRow(`SELECT COALESCE(headers,''), COALESCE(body,'') FROM request_logs WHERE id=?`, id).
		Scan(&headers, &body)
	return
}
