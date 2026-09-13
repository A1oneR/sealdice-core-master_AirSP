package dice

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// ChatMessage represents a single chat message in a group.
type ChatMessage struct {
	ID            string `json:"id"`
	GroupID       string `json:"groupId"`
	GroupName     string `json:"groupName"`
	SenderID      string `json:"senderId"`
	SenderName    string `json:"senderName"`
	SenderDisplay string `json:"senderDisplay"`
	SenderSuffix  string `json:"senderSuffix"`
	IsOfficial    bool   `json:"isOfficial"`
	IsBot         bool   `json:"isBot"`
	Content       string `json:"content"`
	Timestamp     int64  `json:"timestamp"`
	RawID         any    `json:"rawId,omitempty"`
}

// ChatGroupSummary represents group information for the QQ chat sidebar.
type ChatGroupSummary struct {
	GroupID    string       `json:"groupId"`
	GroupName  string       `json:"groupName"`
	Platform   string       `json:"platform"`
	IsOfficial bool         `json:"isOfficial"`
	Active     bool         `json:"active"`
	MemberNum  int64        `json:"memberNum"`
	LastMsg    *ChatMessage `json:"lastMsg,omitempty"`
	MsgCount   int          `json:"msgCount"`
	RecentTime int64        `json:"recentTime"`
}

// ChatManager manages group message history and formatting.
type ChatManager struct {
	mu          sync.RWMutex
	messages    map[string][]*ChatMessage
	maxPerGroup int
	d           *Dice
}

func NewChatManager(d *Dice) *ChatManager {
	return &ChatManager{
		messages:    make(map[string][]*ChatMessage),
		maxPerGroup: 500,
		d:           d,
	}
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

// AddIncomingMessage records an incoming group message from a group member.
func (cm *ChatManager) AddIncomingMessage(ctx *MsgContext, msg *Message) *ChatMessage {
	if cm == nil || msg == nil || msg.GroupID == "" {
		return nil
	}

	groupName := msg.GroupName
	if groupName == "" && ctx != nil && ctx.Group != nil {
		groupName = ctx.Group.GroupName
	}
	if groupName == "" && cm.d != nil && cm.d.Parent != nil {
		groupName = cm.d.Parent.TryGetGroupName(msg.GroupID)
	}

	display, suffix, isOfficial := FormatSenderDisplay(msg.Sender.UserID, msg.Sender.Nickname, msg.GroupID, false)

	t := msg.Time
	if t <= 0 {
		t = time.Now().Unix()
	}

	chatMsg := &ChatMessage{
		ID:            generateChatMsgID(),
		GroupID:       msg.GroupID,
		GroupName:     groupName,
		SenderID:      msg.Sender.UserID,
		SenderName:    msg.Sender.Nickname,
		SenderDisplay: display,
		SenderSuffix:  suffix,
		IsOfficial:    isOfficial,
		IsBot:         false,
		Content:       CleanFaceTypeTags(msg.Message),
		Timestamp:     t,
		RawID:         msg.RawID,
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	list := cm.messages[msg.GroupID]
	if len(list) >= cm.maxPerGroup {
		list = list[len(list)-cm.maxPerGroup+1:]
	}
	cm.messages[msg.GroupID] = append(list, chatMsg)

	return chatMsg
}

// AddOutgoingMessage records an outgoing message sent by the bot into a group.
func (cm *ChatManager) AddOutgoingMessage(ctx *MsgContext, msg *Message) *ChatMessage {
	if cm == nil || msg == nil || msg.GroupID == "" {
		return nil
	}

	groupName := msg.GroupName
	if groupName == "" && ctx != nil && ctx.Group != nil {
		groupName = ctx.Group.GroupName
	}
	if groupName == "" && cm.d != nil && cm.d.Parent != nil {
		groupName = cm.d.Parent.TryGetGroupName(msg.GroupID)
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

	chatMsg := &ChatMessage{
		ID:            generateChatMsgID(),
		GroupID:       msg.GroupID,
		GroupName:     groupName,
		SenderID:      botID,
		SenderName:    botName,
		SenderDisplay: botName,
		SenderSuffix:  "",
		IsOfficial:    strings.HasPrefix(msg.GroupID, "OpenQQ-Group:"),
		IsBot:         true,
		Content:       CleanFaceTypeTags(msg.Message),
		Timestamp:     t,
		RawID:         msg.RawID,
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	list := cm.messages[msg.GroupID]
	if len(list) >= cm.maxPerGroup {
		list = list[len(list)-cm.maxPerGroup+1:]
	}
	cm.messages[msg.GroupID] = append(list, chatMsg)

	return chatMsg
}

// GetMessagesPaged retrieves messages for a group with since and before support.
func (cm *ChatManager) GetMessagesPaged(groupID string, limit int, since int64, before int64) ([]*ChatMessage, bool, int) {
	if cm == nil {
		return []*ChatMessage{}, false, 0
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	list := cm.messages[groupID]
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

// GetMessages retrieves recent messages for a group.
func (cm *ChatManager) GetMessages(groupID string, limit int, since int64) []*ChatMessage {
	msgs, _, _ := cm.GetMessagesPaged(groupID, limit, since, 0)
	return msgs
}

// GetLastMessage gets the latest recorded message for a group.
func (cm *ChatManager) GetLastMessage(groupID string) *ChatMessage {
	if cm == nil {
		return nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	list := cm.messages[groupID]
	if len(list) == 0 {
		return nil
	}
	last := list[len(list)-1]
	copyLast := *last
	copyLast.Content = CleanFaceTypeTags(copyLast.Content)
	return &copyLast
}

// GetMessageCount gets the count of cached messages for a group.
func (cm *ChatManager) GetMessageCount(groupID string) int {
	if cm == nil {
		return 0
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return len(cm.messages[groupID])
}

// ClearMessages removes message history for a group.
func (cm *ChatManager) ClearMessages(groupID string) {
	if cm == nil {
		return
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.messages, groupID)
}
