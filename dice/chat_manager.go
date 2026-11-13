package dice

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	faceTagWithSpaceRegex = regexp.MustCompile(`(?i)<faceType[\s=][^>]*>\s*`)
	faceTagExactRegex     = regexp.MustCompile(`(?i)<faceType[\s=][^>]*>`)
	faceExtRegex          = regexp.MustCompile(`(?i)ext=\\?["']([^"'>\\]+)\\?["']`)
)

// CleanFaceTypeTags cleans or converts QQ faceType tags like <faceType=6,faceId="0",ext="eyJ0ZXh0IjoiW+ivtl0ifQ==">.
// If the message is accompanied by an image, the tag is completely stripped so only the sticker image remains.
// If the message is a standalone emoji/face, it decodes the ext text (e.g. "[咬]") or falls back to "[表情]".
func CleanFaceTypeTags(content string) string {
	if !strings.Contains(content, "<faceType") {
		return content
	}
	hasImage := strings.Contains(content, "[CQ:image") ||
		strings.Contains(content, "[图:") ||
		strings.Contains(content, "[img:") ||
		strings.Contains(content, "![")

	if hasImage {
		return faceTagWithSpaceRegex.ReplaceAllString(content, "")
	}

	return faceTagExactRegex.ReplaceAllStringFunc(content, func(tag string) string {
		if m := faceExtRegex.FindStringSubmatch(tag); len(m) > 1 {
			rawB64 := strings.ReplaceAll(m[1], `\`, "")
			if decoded, err := base64.StdEncoding.DecodeString(rawB64); err == nil {
				var extObj struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(decoded, &extObj); err == nil && strings.TrimSpace(extObj.Text) != "" {
					return strings.TrimSpace(extObj.Text)
				}
			}
		}
		return "[表情]"
	})
}

// ChatMessage represents a single chat message in a group or private session.
type ChatMessage struct {
	ID            string `json:"id"`
	SessionType   string `json:"sessionType"` // "group" or "private"
	SessionID     string `json:"sessionId"`
	SessionName   string `json:"sessionName"`
	GroupID       string `json:"groupId"`
	GroupName     string `json:"groupName"`
	SenderID      string `json:"senderId"`
	SenderName    string `json:"senderName"`
	SenderDisplay string `json:"senderDisplay"`
	SenderSuffix  string `json:"senderSuffix"`
	SenderAvatar  string `json:"senderAvatar,omitempty"`
	IsOfficial    bool   `json:"isOfficial"`
	IsBot         bool   `json:"isBot"`
	Content       string `json:"content"`
	Timestamp     int64  `json:"timestamp"`
	RawID         any    `json:"rawId,omitempty"`
}

// ChatGroupSummary represents session information for the QQ chat sidebar (group or private).
type ChatGroupSummary struct {
	GroupID      string       `json:"groupId"`
	GroupName    string       `json:"groupName"`
	Platform     string       `json:"platform"`
	IsOfficial   bool         `json:"isOfficial"`
	Active       bool         `json:"active"`
	MemberNum    int64        `json:"memberNum"`
	LastMsg      *ChatMessage `json:"lastMsg,omitempty"`
	MsgCount     int          `json:"msgCount"`
	RecentTime   int64        `json:"recentTime"`
	SessionType  string       `json:"sessionType,omitempty"`
	SenderSuffix string       `json:"senderSuffix,omitempty"`
	AvatarURL    string       `json:"avatarUrl,omitempty"`
}

// ChatManager manages group and private message history and formatting.
type ChatManager struct {
	mu          sync.RWMutex
	messages    map[string][]*ChatMessage
	maxPerGroup int
	d           *Dice
	db          *ChatDB
}

func NewChatManager(d *Dice) *ChatManager {
	cm := &ChatManager{
		messages:    make(map[string][]*ChatMessage),
		maxPerGroup: 500,
		d:           d,
	}
	if d != nil && d.BaseConfig.DataDir != "" {
		dbPath := filepath.Join(d.BaseConfig.DataDir, "data-chat.db")
		if db, err := OpenChatDB(dbPath); err == nil {
			cm.db = db
		} else if d.Logger != nil {
			d.Logger.Errorf("打开聊天数据库失败: %v", err)
		}
	}
	return cm
}

func (cm *ChatManager) InitDB(dbPath string) error {
	if cm == nil {
		return nil
	}
	db, err := OpenChatDB(dbPath)
	if err != nil {
		return err
	}
	cm.mu.Lock()
	if cm.db != nil {
		_ = cm.db.Close()
	}
	cm.db = db
	cm.mu.Unlock()
	return nil
}

// Close closes the underlying persistent chat database.
func (cm *ChatManager) Close() error {
	if cm == nil {
		return nil
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.db != nil {
		return cm.db.Close()
	}
	return nil
}

func generateChatMsgID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// FormatSenderDisplay formats the sender display name and extracts the official 4-digit ID suffix when applicable.
func FormatSenderDisplay(senderID, nickname, groupID string, isBot bool) (display string, suffix string, isOfficial bool) {
	if isBot {
		if nickname != "" {
			return nickname, "", false
		}
		return "海豹核心", "", false
	}

	senderID = strings.TrimSpace(senderID)
	isOfficial = strings.HasPrefix(senderID, "OpenQQ:") ||
		strings.HasPrefix(senderID, "OpenQQCH:") ||
		strings.HasPrefix(senderID, "OpenQQ-Member-T:") ||
		strings.HasPrefix(groupID, "OpenQQ-Group:")

	if isOfficial {
		coreID := senderID
		if idx := strings.LastIndex(coreID, "-"); idx != -1 && idx < len(coreID)-1 {
			coreID = coreID[idx+1:]
		} else {
			coreID = strings.TrimPrefix(coreID, "OpenQQ:")
			coreID = strings.TrimPrefix(coreID, "OpenQQCH:")
			coreID = strings.TrimPrefix(coreID, "OpenQQ-Member-T:")
		}
		coreID = strings.TrimSpace(coreID)
		if len(coreID) >= 4 {
			suffix = coreID[len(coreID)-4:]
		} else if len(coreID) > 0 {
			suffix = coreID
		}

		name := strings.TrimSpace(nickname)
		if name == "" {
			name = "用户"
		}
		if suffix != "" {
			if !strings.HasSuffix(name, suffix) && !strings.Contains(name, "("+suffix+")") {
				display = fmt.Sprintf("%s (%s)", name, suffix)
			} else {
				display = name
			}
		} else {
			display = name
		}
		return display, suffix, isOfficial
	}

	name := strings.TrimSpace(nickname)
	if name == "" {
		name = senderID
	}
	return name, "", false
}

// AddIncomingMessage records an incoming message from a group member or private user.
func (cm *ChatManager) AddIncomingMessage(ctx *MsgContext, msg *Message) *ChatMessage {
	if cm == nil || msg == nil {
		return nil
	}

	sessionType := "group"
	sessionID := msg.GroupID
	sessionName := msg.GroupName

	if msg.MessageType == "private" || (sessionID == "" && msg.Sender.UserID != "") {
		sessionType = "private"
		sessionID = msg.Sender.UserID
		sessionName = msg.Sender.Nickname
	}

	if sessionID == "" {
		return nil
	}

	if sessionType == "group" {
		if sessionName == "" && ctx != nil && ctx.Group != nil {
			sessionName = ctx.Group.GroupName
		}
		if sessionName == "" && cm.d != nil && cm.d.Parent != nil {
			sessionName = cm.d.Parent.TryGetGroupName(sessionID)
		}
	}

	display, suffix, isOfficial := FormatSenderDisplay(msg.Sender.UserID, msg.Sender.Nickname, sessionID, false)

	t := msg.Time
	if t <= 0 {
		t = time.Now().Unix()
	}

	chatMsg := &ChatMessage{
		ID:            generateChatMsgID(),
		SessionType:   sessionType,
		SessionID:     sessionID,
		SessionName:   sessionName,
		GroupID:       sessionID,
		GroupName:     sessionName,
		SenderID:      msg.Sender.UserID,
		SenderName:    msg.Sender.Nickname,
		SenderDisplay: display,
		SenderSuffix:  suffix,
		SenderAvatar:  ChatUserAvatar(msg.Sender.UserID, msg.Sender.AvatarURL),
		IsOfficial:    isOfficial,
		IsBot:         false,
		Content:       CleanFaceTypeTags(msg.Message),
		Timestamp:     t,
		RawID:         msg.RawID,
	}

	cm.mu.Lock()
	list := cm.messages[sessionID]
	if len(list) >= cm.maxPerGroup {
		list = list[len(list)-cm.maxPerGroup+1:]
	}
	cm.messages[sessionID] = append(list, chatMsg)
	cm.mu.Unlock()

	if cm.db != nil {
		_ = cm.db.SaveMessage(chatMsg)
	}

	return chatMsg
}

// AddOutgoingMessage records an outgoing message sent by the bot into a group or private chat.
func (cm *ChatManager) AddOutgoingMessage(ctx *MsgContext, msg *Message) *ChatMessage {
	if cm == nil || msg == nil {
		return nil
	}

	sessionType := "group"
	sessionID := msg.GroupID
	sessionName := msg.GroupName

	if msg.MessageType == "private" || (sessionID == "" && ctx != nil && ctx.Player != nil && ctx.Player.UserID != "") {
		sessionType = "private"
		if ctx != nil && ctx.Player != nil {
			sessionID = ctx.Player.UserID
			sessionName = ctx.Player.Name
		}
		if sessionID == "" && msg.TmpUID != "" {
			sessionID = msg.TmpUID
		}
	}

	if sessionID == "" {
		return nil
	}

	if sessionType == "group" {
		if sessionName == "" && ctx != nil && ctx.Group != nil {
			sessionName = ctx.Group.GroupName
		}
		if sessionName == "" && cm.d != nil && cm.d.Parent != nil {
			sessionName = cm.d.Parent.TryGetGroupName(sessionID)
		}
	}
	if sessionType == "private" && sessionName == "" {
		if last := cm.GetLastMessage(sessionID); last != nil && last.SessionName != "" {
			sessionName = last.SessionName
		} else {
			sessionName = sessionID
		}
	}

	botName := msg.Sender.Nickname
	if botName == "" && ctx != nil && ctx.EndPoint != nil {
		botName = ctx.EndPoint.Nickname
	}
	if botName == "" {
		botName = "海豹核心"
	}

	botID := msg.Sender.UserID
	if botID == "" && ctx != nil && ctx.EndPoint != nil {
		botID = ctx.EndPoint.UserID
	}

	t := msg.Time
	if t <= 0 {
		t = time.Now().Unix()
	}

	isOfficial := strings.HasPrefix(sessionID, "OpenQQ-Group:") ||
		strings.HasPrefix(sessionID, "OpenQQ:") ||
		strings.HasPrefix(sessionID, "OpenQQCH:")

	chatMsg := &ChatMessage{
		ID:            generateChatMsgID(),
		SessionType:   sessionType,
		SessionID:     sessionID,
		SessionName:   sessionName,
		GroupID:       sessionID,
		GroupName:     sessionName,
		SenderID:      botID,
		SenderName:    botName,
		SenderDisplay: botName,
		SenderSuffix:  "",
		SenderAvatar:  ChatUserAvatar(botID, msg.Sender.AvatarURL),
		IsOfficial:    isOfficial,
		IsBot:         true,
		Content:       CleanFaceTypeTags(msg.Message),
		Timestamp:     t,
		RawID:         msg.RawID,
	}

	cm.mu.Lock()
	list := cm.messages[sessionID]
	if len(list) >= cm.maxPerGroup {
		list = list[len(list)-cm.maxPerGroup+1:]
	}
	cm.messages[sessionID] = append(list, chatMsg)
	cm.mu.Unlock()

	if cm.db != nil {
		_ = cm.db.SaveMessage(chatMsg)
	}

	return chatMsg
}

// GetMessagesPaged retrieves messages for a session with since and before support.
func (cm *ChatManager) GetMessagesPaged(sessionID string, limit int, since int64, before int64) ([]*ChatMessage, bool, int) {
	if cm == nil || sessionID == "" {
		return []*ChatMessage{}, false, 0
	}

	if cm.db != nil {
		msgs, hasMore, total := cm.db.GetMessagesPaged(sessionID, limit, since, before)
		for _, m := range msgs {
			m.Content = CleanFaceTypeTags(m.Content)
		}
		return msgs, hasMore, total
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	list := cm.messages[sessionID]
	total := len(list)
	if total == 0 {
		return []*ChatMessage{}, false, 0
	}

	var filtered []*ChatMessage
	for _, m := range list {
		if since > 0 && m.Timestamp <= since {
			continue
		}
		if before > 0 && m.Timestamp >= before {
			continue
		}
		filtered = append(filtered, m)
	}

	hasMore := false
	if limit > 0 && len(filtered) > limit {
		hasMore = true
		filtered = filtered[len(filtered)-limit:]
	}

	res := make([]*ChatMessage, len(filtered))
	for i, m := range filtered {
		copyMsg := *m
		copyMsg.Content = CleanFaceTypeTags(copyMsg.Content)
		res[i] = &copyMsg
	}
	return res, hasMore, total
}

// GetMessages retrieves recent messages for a session.
func (cm *ChatManager) GetMessages(sessionID string, limit int, since int64) []*ChatMessage {
	msgs, _, _ := cm.GetMessagesPaged(sessionID, limit, since, 0)
	return msgs
}

// GetLastMessage gets the latest recorded message for a session.
func (cm *ChatManager) GetLastMessage(sessionID string) *ChatMessage {
	if cm == nil || sessionID == "" {
		return nil
	}

	if cm.db != nil {
		if msg := cm.db.GetLastMessage(sessionID); msg != nil {
			msg.Content = CleanFaceTypeTags(msg.Content)
			return msg
		}
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	list := cm.messages[sessionID]
	if len(list) == 0 {
		return nil
	}
	last := list[len(list)-1]
	copyLast := *last
	copyLast.Content = CleanFaceTypeTags(copyLast.Content)
	return &copyLast
}

// GetMessageCount gets the count of messages for a session.
func (cm *ChatManager) GetMessageCount(sessionID string) int {
	if cm == nil || sessionID == "" {
		return 0
	}

	if cm.db != nil {
		if cnt := cm.db.GetMessageCount(sessionID); cnt > 0 {
			return cnt
		}
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return len(cm.messages[sessionID])
}

// ClearMessages removes message history for a session.
func (cm *ChatManager) ClearMessages(sessionID string) {
	if cm == nil || sessionID == "" {
		return
	}

	cm.mu.Lock()
	delete(cm.messages, sessionID)
	cm.mu.Unlock()

	if cm.db != nil {
		_ = cm.db.ClearSession(sessionID)
	}
}

// GetSessions returns recent session summaries from DB for the specified sessionType.
func (cm *ChatManager) GetSessions(sessionType string, limit, offset int, search string) ([]*ChatGroupSummary, int, error) {
	if cm != nil && cm.db != nil {
		return cm.db.GetRecentSessions(sessionType, limit, offset, search)
	}
	return []*ChatGroupSummary{}, 0, nil
}
