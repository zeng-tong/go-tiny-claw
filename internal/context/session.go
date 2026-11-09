package context

import (
	"go-tiny-claw/internal/schema"
	"sync"
	"time"
)

type Session struct {
	ID       string
	WorkDir  string
	CreateAt time.Time
	UpdateAt time.Time
	// 存放该 session 中所有用户输入、大模型回复和工具调用结果
	history []schema.Message
	mu      sync.RWMutex // 读写锁
}

func NewSession(id, workDir string) *Session {
	return &Session{
		ID:       id,
		WorkDir:  workDir,
		CreateAt: time.Now(),
		UpdateAt: time.Now(),
		history:  make([]schema.Message, 0),
	}
}

func (s *Session) Append(msg ...schema.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, msg...)
	s.UpdateAt = time.Now()
	// 【持久化预留点】：在真实的工业级实现中（如 Claude Code），
	//  我们会在这里将 s.history 以 JSONL 的格式 Append 到 workDir/.claw/sessions/xxx.jsonl 中。
	// s.SaveToDisk()
}

func (s *Session) GetWorkingMemory(limit int) []schema.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := len(s.history)
	if total <= limit || limit < 0 { //总量小于限制或者不设限，全量返回。
		res := make([]schema.Message, total)
		copy(res, s.history)
		return res
	}
	// 截取最近的 limit 条
	res := make([]schema.Message, limit)
	copy(res, s.history[total-limit:])

	//【驾驭防线】 LLM API 强制要求历史消息的连续性
	// 若截断的第一条消息恰好是 toolResult，但发出这个请求的 toolCall 被截断抛弃了。LLM API 会报 400 Bad Request
	for len(res) > 0 {
		if res[0].Role == schema.RoleUser && res[0].ToolCallID != "" {
			res = res[1:]
		} else {
			break
		}
	}
	return res
}

type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

var GlobalSessionManager = &SessionManager{
	sessions: make(map[string]*Session),
}

// GetOrCreate 获取或创建会话
func (sm *SessionManager) GetOrCreate(id string, workDir string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sess, exist := sm.sessions[id]; exist {
		return sess
	}
	sess := NewSession(id, workDir)
	sm.sessions[id] = sess
	return sess
}
