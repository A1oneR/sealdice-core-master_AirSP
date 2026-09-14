package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"sealdice-core/dice"
)

func setupTestChatAPI(t *testing.T) (token string, testDice *dice.Dice) {
	testDice = &dice.Dice{
		ImSession: &dice.IMSession{
			ServiceAtNew: new(dice.SyncMap[string, *dice.GroupInfo]),
		},
	}
	testDice.ImSession.Parent = testDice
	testDice.ChatManager = dice.NewChatManager(testDice)

	group1 := &dice.GroupInfo{
		GroupID:         "OpenQQ-Group:1234-group1",
		GroupName:       "官方测试群",
		Active:          true,
		DiceIDActiveMap: new(dice.SyncMap[string, bool]),
		DiceIDExistsMap: new(dice.SyncMap[string, bool]),
	}
	group1.DiceIDActiveMap.Store("OpenQQ:1234", true)
	group1.DiceIDExistsMap.Store("OpenQQ:1234", true)

	group2 := &dice.GroupInfo{
		GroupID:         "QQ-Group:5678",
		GroupName:       "普通QQ群",
		Active:          false,
		DiceIDActiveMap: new(dice.SyncMap[string, bool]),
		DiceIDExistsMap: new(dice.SyncMap[string, bool]),
	}
	group2.DiceIDExistsMap.Store("QQ:9999", true)

	testDice.ImSession.ServiceAtNew.Store(group1.GroupID, group1)
	testDice.ImSession.ServiceAtNew.Store(group2.GroupID, group2)

	// Add sample messages to group1
	testDice.ChatManager.AddIncomingMessage(nil, &dice.Message{
		GroupID:     group1.GroupID,
		GroupName:   group1.GroupName,
		MessageType: "group",
		Message:     "大家好，我是调查员",
		Time:        1000,
		Sender: dice.SenderBase{
			UserID:   "OpenQQ:1234-F8A2",
			Nickname: "张三",
		},
	})
	testDice.ChatManager.AddOutgoingMessage(nil, &dice.Message{
		GroupID:     group1.GroupID,
		GroupName:   group1.GroupName,
		MessageType: "group",
		Message:     "欢迎加入跑团！",
		Time:        1005,
		Sender: dice.SenderBase{
			UserID:   "OpenQQ:1234",
			Nickname: "海豹测试机",
		},
	})

	token = "chat-test-token"
	manager := &dice.DiceManager{
		Dice:        []*dice.Dice{testDice},
		JustForTest: false,
	}
	manager.AccessTokens.Store(token, true)
	testDice.Parent = manager
	myDice = testDice
	dm = manager

	t.Cleanup(func() {
		myDice = nil
		dm = nil
	})

	return token, testDice
}

func TestChatAPIGroupList(t *testing.T) {
	token, _ := setupTestChatAPI(t)

	// 1. Unauthorized
	rec403 := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/groups", "", "wrong-token", chatGroupList)
	if rec403.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec403.Code)
	}

	// 2. Authorized - default onlyWithMsgs=true (filters out empty group2)
	rec200 := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/groups", "", token, chatGroupList)
	if rec200.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec200.Code, rec200.Body.String())
	}

	var res struct {
		Items           []*dice.ChatGroupSummary `json:"items"`
		Total           int                      `json:"total"`
		Page            int                      `json:"page"`
		PageSize        int                      `json:"pageSize"`
		TotalPages      int                      `json:"totalPages"`
		HasMessagesOnly bool                     `json:"hasMessagesOnly"`
	}
	if err := json.Unmarshal(rec200.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal group list response: %v", err)
	}

	if len(res.Items) != 1 || res.Total != 1 {
		t.Fatalf("expected 1 group with messages by default, got len=%d total=%d", len(res.Items), res.Total)
	}

	// The active group with recent message should be first
	first := res.Items[0]
	if first.GroupID != "OpenQQ-Group:1234-group1" {
		t.Fatalf("expected first group to be OpenQQ-Group:1234-group1, got %s", first.GroupID)
	}
	if !first.IsOfficial {
		t.Fatal("expected OpenQQ group to have IsOfficial=true")
	}
	if first.MsgCount != 2 {
		t.Fatalf("expected MsgCount=2, got %d", first.MsgCount)
	}
	if first.LastMsg == nil || first.LastMsg.Content != "欢迎加入跑团！" {
		t.Fatalf("unexpected LastMsg: %#v", first.LastMsg)
	}

	// 3. Query all groups with showAll=true
	recAll := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/groups?showAll=true", "", token, chatGroupList)
	var resAll struct {
		Items []*dice.ChatGroupSummary `json:"items"`
		Total int                      `json:"total"`
	}
	if err := json.Unmarshal(recAll.Body.Bytes(), &resAll); err != nil {
		t.Fatalf("failed to unmarshal showAll response: %v", err)
	}
	if len(resAll.Items) != 2 || resAll.Total != 2 {
		t.Fatalf("expected 2 groups with showAll=true, got len=%d total=%d", len(resAll.Items), resAll.Total)
	}

	// 4. Test pagination page=1&pageSize=1
	recPaged := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/groups?showAll=true&page=1&pageSize=1", "", token, chatGroupList)
	var resPaged struct {
		Items      []*dice.ChatGroupSummary `json:"items"`
		Total      int                      `json:"total"`
		Page       int                      `json:"page"`
		PageSize   int                      `json:"pageSize"`
		TotalPages int                      `json:"totalPages"`
	}
	_ = json.Unmarshal(recPaged.Body.Bytes(), &resPaged)
	if len(resPaged.Items) != 1 || resPaged.Total != 2 || resPaged.TotalPages != 2 {
		t.Fatalf("unexpected pagination result: len=%d total=%d pages=%d", len(resPaged.Items), resPaged.Total, resPaged.TotalPages)
	}
}

func TestChatAPIMessageList(t *testing.T) {
	token, _ := setupTestChatAPI(t)

	// 1. Missing groupId
	recBad := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/messages", "", token, chatMessageList)
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when missing groupId, got %d", recBad.Code)
	}

	// 2. Query messages of group1
	rec200 := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/messages?groupId=OpenQQ-Group:1234-group1", "", token, chatMessageList)
	if rec200.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec200.Code, rec200.Body.String())
	}

	var res struct {
		Messages []*dice.ChatMessage `json:"messages"`
		HasMore  bool                `json:"hasMore"`
		Total    int                 `json:"total"`
	}
	if err := json.Unmarshal(rec200.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal messages response: %v", err)
	}

	if len(res.Messages) != 2 || res.Total != 2 {
		t.Fatalf("expected 2 messages, got len=%d total=%d", len(res.Messages), res.Total)
	}
	// Check official bot ID suffix in sender display
	m1 := res.Messages[0]
	if m1.SenderSuffix != "F8A2" {
		t.Fatalf("expected SenderSuffix=F8A2, got %s", m1.SenderSuffix)
	}
	if m1.SenderDisplay != "张三 (F8A2)" {
		t.Fatalf("expected SenderDisplay='张三 (F8A2)', got %s", m1.SenderDisplay)
	}

	// 3. Incremental query with since
	recInc := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/messages?groupId=OpenQQ-Group:1234-group1&since=1000", "", token, chatMessageList)
	var resInc struct {
		Messages []*dice.ChatMessage `json:"messages"`
	}
	_ = json.Unmarshal(recInc.Body.Bytes(), &resInc)
	if len(resInc.Messages) != 1 || resInc.Messages[0].Content != "欢迎加入跑团！" {
		t.Fatalf("expected 1 incremental message, got %#v", resInc.Messages)
	}

	// 4. Query with before
	recBefore := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/messages?groupId=OpenQQ-Group:1234-group1&before=1005", "", token, chatMessageList)
	var resBefore struct {
		Messages []*dice.ChatMessage `json:"messages"`
	}
	_ = json.Unmarshal(recBefore.Body.Bytes(), &resBefore)
	if len(resBefore.Messages) != 1 || resBefore.Messages[0].Content != "大家好，我是调查员" {
		t.Fatalf("expected 1 earlier message, got %#v", resBefore.Messages)
	}
}

func TestChatAPIMessageSend(t *testing.T) {
	token, _ := setupTestChatAPI(t)

	// Test mode returns ok without sending
	dm.JustForTest = true
	recTestMode := performReplyAPIRequest(t, http.MethodPost, "/sd-api/chat/send", `{"groupId":"OpenQQ-Group:1234-group1","message":"测试发送"}`, token, chatMessageSend)
	if recTestMode.Code != http.StatusOK {
		t.Fatalf("expected 200 in test mode, got %d", recTestMode.Code)
	}

	// Bad request on empty message
	dm.JustForTest = false
	recEmpty := performReplyAPIRequest(t, http.MethodPost, "/sd-api/chat/send", `{"groupId":"OpenQQ-Group:1234-group1","message":""}`, token, chatMessageSend)
	if recEmpty.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty message, got %d", recEmpty.Code)
	}

	// No connected endpoint error
	recNoEP := performReplyAPIRequest(t, http.MethodPost, "/sd-api/chat/send", `{"groupId":"OpenQQ-Group:1234-group1","message":"hello"}`, token, chatMessageSend)
	if recNoEP.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when no endpoint is connected, got %d: %s", recNoEP.Code, recNoEP.Body.String())
	}
}

func TestChatAPIMessageClear(t *testing.T) {
	token, testDice := setupTestChatAPI(t)

	recClear := performReplyAPIRequest(t, http.MethodPost, "/sd-api/chat/clear", `{"groupId":"OpenQQ-Group:1234-group1"}`, token, chatMessageClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 on clear, got %d", recClear.Code)
	}

	if testDice.ChatManager.GetMessageCount("OpenQQ-Group:1234-group1") != 0 {
		t.Fatal("expected message count to be 0 after clear API")
	}
}

func TestChatAPIImageGet(t *testing.T) {
	token, _ := setupTestChatAPI(t)

	// Test 403 Forbidden without token
	recAuth := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/image?path=data/test.png", "", "bad-token", chatImageGet)
	if recAuth.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on bad token, got %d", recAuth.Code)
	}

	// Test 400 Bad Request on empty path
	recEmpty := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/image?path=", "", token, chatImageGet)
	if recEmpty.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty path, got %d", recEmpty.Code)
	}

	// Test 403 Forbidden on directory traversal
	recEscape := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/image?path=../../etc/passwd", "", token, chatImageGet)
	if recEscape.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on traversal attempt, got %d", recEscape.Code)
	}

	// Test 404 on non-existent file inside data/
	rec404 := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/image?path=data/images/nonexistent_xyz.png", "", token, chatImageGet)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on non-existent file, got %d", rec404.Code)
	}
}

func TestChatAPIPrivateChat(t *testing.T) {
	token, testDice := setupTestChatAPI(t)
	_ = testDice.ChatManager.InitDB(":memory:")
	defer testDice.ChatManager.Close()

	userID := "OpenQQ:friend1-8888"
	// 1. Add private message
	testDice.ChatManager.AddIncomingMessage(nil, &dice.Message{
		MessageType: "private",
		Message:     "私聊测试",
		Time:        time.Now().Unix(),
		Sender: dice.SenderBase{
			UserID:   userID,
			Nickname: "测试好友",
		},
	})

	// 2. Query private sessions via API
	recPrivate := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/groups?sessionType=private", "", token, chatGroupList)
	if recPrivate.Code != http.StatusOK {
		t.Fatalf("expected 200 on private groups query, got %d", recPrivate.Code)
	}
	var resp struct {
		Items []struct {
			GroupID      string `json:"groupId"`
			GroupName    string `json:"groupName"`
			SenderSuffix string `json:"senderSuffix"`
		} `json:"items"`
		Total       int    `json:"total"`
		SessionType string `json:"sessionType"`
	}
	if err := json.Unmarshal(recPrivate.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("expected 1 private session, got total=%d len=%d", resp.Total, len(resp.Items))
	}
	if resp.Items[0].GroupID != userID || resp.Items[0].SenderSuffix != "8888" {
		t.Errorf("unexpected private item: %+v", resp.Items[0])
	}

	// 3. Query private messages via API
	recMsgs := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/messages?targetId="+userID, "", token, chatMessageList)
	if recMsgs.Code != http.StatusOK {
		t.Fatalf("expected 200 on private messages query, got %d", recMsgs.Code)
	}

	// 4. Send private message in test mode
	dm.JustForTest = true
	recSend := performReplyAPIRequest(t, http.MethodPost, "/sd-api/chat/send", fmt.Sprintf(`{"targetId":"%s","sessionType":"private","message":"回复好友"}`, userID), token, chatMessageSend)
	if recSend.Code != http.StatusOK {
		t.Fatalf("expected 200 on private send, got %d", recSend.Code)
	}

	// 5. Clear private messages via API
	recClear := performReplyAPIRequest(t, http.MethodPost, "/sd-api/chat/clear", fmt.Sprintf(`{"targetId":"%s"}`, userID), token, chatMessageClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 on private clear, got %d", recClear.Code)
	}
	if testDice.ChatManager.GetMessageCount(userID) != 0 {
		t.Fatal("expected 0 messages after clear")
	}
}

