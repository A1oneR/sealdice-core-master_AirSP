package dice

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sealdice/botgo/dto"
	"sealdice-core/message"
)

func TestChatAvatarIdentityBoundaries(t *testing.T) {
	if ChatGroupAvatar("QQ-Group:123456") != "https://p.qlogo.cn/gh/123456/123456/100" {
		t.Fatal("numeric group avatar missing")
	}
	if ChatGroupAvatar("OpenQQ-Group:fixture-123456") != "" || ChatUserAvatar("OpenQQ:fixture-123456", "") != "" {
		t.Fatal("opaque identity used as numeric QQ ID")
	}
	if ChatUserAvatar("QQ:123456", "javascript:alert(1)") != "https://q1.qlogo.cn/g?b=qq&nk=123456&s=100" {
		t.Fatal("unsafe supplied avatar not rejected")
	}
	if ChatUserAvatar("OpenQQ:fixture-member", "https://example.test/avatar.png") != "https://example.test/avatar.png" {
		t.Fatal("platform avatar lost")
	}
}

func TestChatOfficialGroupProfileWire(t *testing.T) {
	api := officialQQTransportFunc(func(_ context.Context, method, endpoint string, body interface{}) ([]byte, error) {
		if method != http.MethodGet || !strings.HasSuffix(endpoint, "/v2/groups/fixture-group/info") || body != nil {
			t.Fatal("incorrect group info request")
		}
		return []byte(`{"group_name":"真实群名","group_member_num":42}`), nil
	})
	profile, err := fetchOfficialQQChatGroupProfile(context.Background(), api, "fixture-group")
	if err != nil || profile.GroupName != "真实群名" || profile.MemberNum != 42 || profile.AvatarURL != "" {
		t.Fatalf("unexpected profile: %+v, %v", profile, err)
	}
}

func TestChatOfficialIncomingAvatar(t *testing.T) {
	pa := &PlatformAdapterOfficialQQ{UIN: "fixture"}
	msg := pa.groupNormalMsgToStdMsg(nil, &dto.WSGroupMessageData{Author: &dto.User{MemberOpenID: "fixture-member", Avatar: "https://example.test/avatar.png"}, GroupOpenID: "fixture-group"})
	if msg.Sender.AvatarURL != "https://example.test/avatar.png" {
		t.Fatal("avatar lost for sender without username")
	}
}

func TestChatDBAvatarMigrationPreservesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE chat_messages (id TEXT PRIMARY KEY, session_type TEXT NOT NULL DEFAULT 'group', session_id TEXT NOT NULL, session_name TEXT, sender_id TEXT NOT NULL, sender_name TEXT, sender_display TEXT, sender_suffix TEXT, is_official INTEGER NOT NULL DEFAULT 0, is_bot INTEGER NOT NULL DEFAULT 0, content TEXT NOT NULL, timestamp INTEGER NOT NULL, raw_id TEXT);
	INSERT INTO chat_messages VALUES ('old','group','QQ-Group:123456','旧群','QQ:123456','旧用户','旧用户','',0,0,'旧消息',1,'old-id');`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	cdb, err := OpenChatDB(path)
	if err != nil {
		t.Fatal(err)
	}
	old := cdb.GetLastMessage("QQ-Group:123456")
	if old == nil || old.Content != "旧消息" || old.SenderAvatar == "" {
		t.Fatal("legacy history lost")
	}
	msg := &ChatMessage{ID: "new", SessionID: "QQ-Group:123456", SenderID: "OpenQQ:fixture-member", SenderAvatar: "https://example.test/avatar.png", Content: "新消息", Timestamp: 2}
	if err := cdb.SaveMessage(msg); err != nil {
		t.Fatal(err)
	}
	cdb.Close()
	cdb, err = OpenChatDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer cdb.Close()
	if got := cdb.GetLastMessage(msg.SessionID); got == nil || got.SenderAvatar != msg.SenderAvatar || cdb.GetMessageCount(msg.SessionID) != 2 {
		t.Fatal("avatar persistence or idempotent migration failed")
	}
}

func TestChatRichShortcutsMarkdownWire(t *testing.T) {
	d := &Dice{}
	s := &IMSession{Parent: d}
	pa := &PlatformAdapterOfficialQQ{EndPoint: &EndPointInfo{EndPointInfoBase: EndPointInfoBase{Session: s}}}
	text := `$\infty$ $\dfrac{a}{b}$ <qqbot-cmd-input text="%2Er%201d100" show="%E6%8E%B7%E9%AA%B0" reference="false" />`
	elems := message.ConvertStringMessage(text)
	if len(elems) != 1 {
		t.Fatal("rich text was split into segments")
	}
	content, ok := elems[0].(*message.TextElement)
	if !ok || content.Content != text {
		t.Fatal("rich text was rewritten")
	}
	ctx := &MsgContext{}
	translated := ctx.TranslateSplit(text)
	if len(ctx.SplitText(translated)) != 1 || translated != text {
		t.Fatal("shortcut triggered form-feed splitting")
	}
	payload := &dto.MessageToCreate{}
	pa.finalizeMessageToCreate(payload, textLinkStrip(content.Content), nil, true, true)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		MsgType  int `json:"msg_type"`
		Markdown struct {
			Content string `json:"content"`
		} `json:"markdown"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.MsgType != 2 || wire.Markdown.Content != text {
		t.Fatal("KaTeX or command-input tag missing from Markdown wire payload")
	}
}
