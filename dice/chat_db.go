package dice

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/glebarez/go-sqlite"
)

// ChatDB provides persistent SQLite storage for chat messages.
type ChatDB struct {
	db *sql.DB
	mu sync.RWMutex
}

// OpenChatDB opens or creates the chat database at the given path.
func OpenChatDB(dbPath string) (*ChatDB, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create chat db dir failed: %w", err)
		}
	}

	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = fmt.Sprintf("file:%s?_txlock=immediate&_busy_timeout=15000", filepath.ToSlash(dbPath))
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open chat db failed: %w", err)
	}

	db.SetMaxOpenConns(1)

	cdb := &ChatDB{db: db}
	if err := cdb.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init chat db schema failed: %w", err)
	}

	return cdb, nil
}

func (c *ChatDB) initSchema() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, p := range pragmas {
		_, _ = c.db.Exec(p)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS chat_messages (
		id TEXT PRIMARY KEY,
		session_type TEXT NOT NULL DEFAULT 'group',
		session_id TEXT NOT NULL,
		session_name TEXT,
		sender_id TEXT NOT NULL,
		sender_name TEXT,
		sender_display TEXT,
		sender_suffix TEXT,
		is_official INTEGER NOT NULL DEFAULT 0,
		is_bot INTEGER NOT NULL DEFAULT 0,
		content TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		raw_id TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_chat_msg_session ON chat_messages(session_id, timestamp);
	CREATE INDEX IF NOT EXISTS idx_chat_msg_type ON chat_messages(session_type, timestamp);
	CREATE INDEX IF NOT EXISTS idx_chat_msg_recent ON chat_messages(session_type, session_id, timestamp);
	`
	_, err := c.db.Exec(schema)
	if err != nil {
		return err
	}
	// Migrate existing histories without discarding messages.
	rows, err := c.db.Query("PRAGMA table_info(chat_messages)")
	if err != nil {
		return err
	}
	hasAvatar := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, columnType string
		var defaultValue any
		if err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "sender_avatar" {
			hasAvatar = true
		}
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return readErr
	}
	if !hasAvatar {
		_, err = c.db.Exec("ALTER TABLE chat_messages ADD COLUMN sender_avatar TEXT NOT NULL DEFAULT ''")
	}
	return err
}

// Close closes the database connection.
func (c *ChatDB) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.db.Close()
}

// SaveMessage persists a chat message into the database.
func (c *ChatDB) SaveMessage(msg *ChatMessage) error {
	if c == nil || c.db == nil || msg == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	sessionType := msg.SessionType
	if sessionType == "" {
		sessionType = "group"
	}
	sessionID := msg.SessionID
	if sessionID == "" {
		sessionID = msg.GroupID
	}
	sessionName := msg.SessionName
	if sessionName == "" {
		sessionName = msg.GroupName
	}

	rawIDStr := ""
	if msg.RawID != nil {
		rawIDStr = fmt.Sprintf("%v", msg.RawID)
	}

	isOfficialInt := 0
	if msg.IsOfficial {
		isOfficialInt = 1
	}
	isBotInt := 0
	if msg.IsBot {
		isBotInt = 1
	}

	query := `
	INSERT OR REPLACE INTO chat_messages (
		id, session_type, session_id, session_name,
		sender_id, sender_name, sender_display, sender_suffix,
		is_official, is_bot, content, timestamp, raw_id, sender_avatar
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`
	_, err := c.db.Exec(query,
		msg.ID, sessionType, sessionID, sessionName,
		msg.SenderID, msg.SenderName, msg.SenderDisplay, msg.SenderSuffix,
		isOfficialInt, isBotInt, msg.Content, msg.Timestamp, rawIDStr, msg.SenderAvatar,
	)
	return err
}

// GetMessagesPaged retrieves paginated messages for a session.
func (c *ChatDB) GetMessagesPaged(sessionID string, limit int, since int64, before int64) ([]*ChatMessage, bool, int) {
	if c == nil || c.db == nil || sessionID == "" {
		return []*ChatMessage{}, false, 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	var total int
	_ = c.db.QueryRow("SELECT COUNT(*) FROM chat_messages WHERE session_id = ?", sessionID).Scan(&total)
	if total == 0 {
		return []*ChatMessage{}, false, 0
	}

	if limit <= 0 {
		limit = 40
	}

	var msgs []*ChatMessage
	var hasMore bool

	if since > 0 {
		// Incremental update: chronological order
		query := `
		SELECT id, session_type, session_id, session_name,
		       sender_id, sender_name, sender_display, sender_suffix,
		       is_official, is_bot, content, timestamp, raw_id, sender_avatar
		FROM chat_messages
		WHERE session_id = ? AND timestamp > ?
		ORDER BY timestamp ASC, id ASC
		LIMIT ?
		`
		rows, err := c.db.Query(query, sessionID, since, limit+1)
		if err != nil {
			return []*ChatMessage{}, false, total
		}
		defer rows.Close()

		for rows.Next() {
			m := c.scanMessage(rows)
			if m != nil {
				msgs = append(msgs, m)
			}
		}
		if len(msgs) > limit {
			hasMore = true
			msgs = msgs[:limit]
		}
		return msgs, hasMore, total
	}

	// Normal or history backward pagination: fetch newest first up to limit+1, then reverse
	var query string
	var args []any
	if before > 0 {
		query = `
		SELECT id, session_type, session_id, session_name,
		       sender_id, sender_name, sender_display, sender_suffix,
		       is_official, is_bot, content, timestamp, raw_id, sender_avatar
		FROM chat_messages
		WHERE session_id = ? AND timestamp < ?
		ORDER BY timestamp DESC, id DESC
		LIMIT ?
		`
		args = []any{sessionID, before, limit + 1}
	} else {
		query = `
		SELECT id, session_type, session_id, session_name,
		       sender_id, sender_name, sender_display, sender_suffix,
		       is_official, is_bot, content, timestamp, raw_id, sender_avatar
		FROM chat_messages
		WHERE session_id = ?
		ORDER BY timestamp DESC, id DESC
		LIMIT ?
		`
		args = []any{sessionID, limit + 1}
	}

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return []*ChatMessage{}, false, total
	}
	defer rows.Close()

	for rows.Next() {
		m := c.scanMessage(rows)
		if m != nil {
			msgs = append(msgs, m)
		}
	}

	if len(msgs) > limit {
		hasMore = true
		msgs = msgs[:limit]
	}

	// Reverse to restore chronological order (oldest to newest)
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, hasMore, total
}

func (c *ChatDB) scanMessage(scanner interface{ Scan(dest ...any) error }) *ChatMessage {
	var (
		id, sessionType, sessionID, sessionName           string
		senderID, senderName, senderDisplay, senderSuffix string
		isOfficialInt, isBotInt                           int
		content                                           string
		timestamp                                         int64
		rawID                                             sql.NullString
		senderAvatar                                      string
	)

	err := scanner.Scan(
		&id, &sessionType, &sessionID, &sessionName,
		&senderID, &senderName, &senderDisplay, &senderSuffix,
		&isOfficialInt, &isBotInt, &content, &timestamp, &rawID, &senderAvatar,
	)
	if err != nil {
		return nil
	}

	var rawVal any
	if rawID.Valid && rawID.String != "" {
		rawVal = rawID.String
	}

	return &ChatMessage{
		ID:            id,
		SessionType:   sessionType,
		SessionID:     sessionID,
		SessionName:   sessionName,
		GroupID:       sessionID,
		GroupName:     sessionName,
		SenderID:      senderID,
		SenderName:    senderName,
		SenderDisplay: senderDisplay,
		SenderSuffix:  senderSuffix,
		SenderAvatar:  ChatUserAvatar(senderID, senderAvatar),
		IsOfficial:    isOfficialInt == 1,
		IsBot:         isBotInt == 1,
		Content:       content,
		Timestamp:     timestamp,
		RawID:         rawVal,
	}
}

func (c *ChatDB) getLastMessageLocked(sessionID string) *ChatMessage {
	if c == nil || c.db == nil || sessionID == "" {
		return nil
	}

	query := `
	SELECT id, session_type, session_id, session_name,
	       sender_id, sender_name, sender_display, sender_suffix,
	       is_official, is_bot, content, timestamp, raw_id, sender_avatar
	FROM chat_messages
	WHERE session_id = ?
	ORDER BY timestamp DESC, id DESC
	LIMIT 1
	`
	row := c.db.QueryRow(query, sessionID)
	return c.scanMessage(row)
}

// GetLastMessage retrieves the most recent message in a session.
func (c *ChatDB) GetLastMessage(sessionID string) *ChatMessage {
	if c == nil || c.db == nil || sessionID == "" {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getLastMessageLocked(sessionID)
}

// GetMessageCount returns the total message count for a session.
func (c *ChatDB) GetMessageCount(sessionID string) int {
	if c == nil || c.db == nil || sessionID == "" {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	var cnt int
	_ = c.db.QueryRow("SELECT COUNT(*) FROM chat_messages WHERE session_id = ?", sessionID).Scan(&cnt)
	return cnt
}

// ClearSession deletes all messages belonging to the given session.
func (c *ChatDB) ClearSession(sessionID string) error {
	if c == nil || c.db == nil || sessionID == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.db.Exec("DELETE FROM chat_messages WHERE session_id = ?", sessionID)
	return err
}

// GetRecentSessions returns grouped session summaries for the given sessionType ("group" or "private").
func (c *ChatDB) GetRecentSessions(sessionType string, limit, offset int, search string) ([]*ChatGroupSummary, int, error) {
	if c == nil || c.db == nil {
		return nil, 0, nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	whereClause := "session_type = ?"
	args := []any{sessionType}
	search = strings.TrimSpace(search)
	if search != "" {
		whereClause += " AND (session_id LIKE ? OR session_name LIKE ?)"
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT session_id) FROM chat_messages WHERE %s", whereClause)
	var total int
	if err := c.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*ChatGroupSummary{}, 0, nil
	}

	query := fmt.Sprintf(`
	SELECT session_id, MAX(session_name) as name, MAX(timestamp) as recent_time, COUNT(*) as msg_cnt
	FROM chat_messages
	WHERE %s
	GROUP BY session_id
	ORDER BY recent_time DESC
	LIMIT ? OFFSET ?
	`, whereClause)

	pageArgs := make([]any, 0, len(args)+2)
	pageArgs = append(pageArgs, args...)
	pageArgs = append(pageArgs, limit, offset)
	rows, err := c.db.Query(query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}

	type rawSession struct {
		sessionID  string
		name       string
		recentTime int64
		msgCount   int
	}
	var rawSessions []rawSession
	for rows.Next() {
		var s rawSession
		if err := rows.Scan(&s.sessionID, &s.name, &s.recentTime, &s.msgCount); err == nil {
			rawSessions = append(rawSessions, s)
		}
	}
	_ = rows.Close() // Release connection to pool before performing point queries

	var summaries []*ChatGroupSummary
	for _, raw := range rawSessions {
		sessionID := raw.sessionID
		name := raw.name
		recentTime := raw.recentTime
		msgCount := raw.msgCount

		lastMsg := c.getLastMessageLocked(sessionID)
		if lastMsg != nil && lastMsg.SessionName != "" {
			name = lastMsg.SessionName
		}
		if name == "" {
			name = sessionID
		}

		isOfficial := strings.HasPrefix(sessionID, "OpenQQ:") ||
			strings.HasPrefix(sessionID, "OpenQQCH:") ||
			strings.HasPrefix(sessionID, "OpenQQ-Group:")
		platform := "QQ"
		if isOfficial {
			platform = "OpenQQ"
		} else if dashIdx := strings.Index(sessionID, "-"); dashIdx != -1 {
			platform = sessionID[:dashIdx]
		} else if colonIdx := strings.Index(sessionID, ":"); colonIdx != -1 {
			platform = sessionID[:colonIdx]
		}

		var suffix string
		if lastMsg != nil && lastMsg.SenderSuffix != "" {
			suffix = lastMsg.SenderSuffix
		} else if isOfficial {
			_, suffix, _ = FormatSenderDisplay(sessionID, name, "", false)
		}

		memberNum := int64(1)
		if sessionType == "group" {
			memberNum = 0
		}

		summaries = append(summaries, &ChatGroupSummary{
			GroupID:      sessionID,
			GroupName:    name,
			Platform:     platform,
			IsOfficial:   isOfficial,
			Active:       true,
			MemberNum:    memberNum,
			LastMsg:      lastMsg,
			MsgCount:     msgCount,
			RecentTime:   recentTime,
			SessionType:  sessionType,
			SenderSuffix: suffix,
		})
	}

	return summaries, total, nil
}
