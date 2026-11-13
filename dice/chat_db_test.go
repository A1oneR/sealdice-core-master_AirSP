package dice

import (
	"testing"
	"time"
)

func TestChatDB_BasicAndPagination(t *testing.T) {
	cdb, err := OpenChatDB(":memory:")
	if err != nil {
		t.Fatalf("OpenChatDB(:memory:) failed: %v", err)
	}
	defer cdb.Close()

	// 1. Initially empty
	msgs, hasMore, total := cdb.GetMessagesPaged("group-1", 10, 0, 0)
	if len(msgs) != 0 || hasMore || total != 0 {
		t.Fatalf("expected empty messages, got %d (hasMore=%v, total=%d)", len(msgs), hasMore, total)
	}

	// 2. Save 5 messages for group-1 with timestamps 1001..1005
	for i := 1; i <= 5; i++ {
		msg := &ChatMessage{
			ID:            generateChatMsgID(),
			SessionType:   "group",
			SessionID:     "group-1",
			SessionName:   "测试群1",
			GroupID:       "group-1",
			GroupName:     "测试群1",
			SenderID:      "user-100",
			SenderName:    "张三",
			SenderDisplay: "张三",
			Content:       CleanFaceTypeTags("hello " + string(rune('0'+i))),
			Timestamp:     int64(1000 + i),
		}
		if err := cdb.SaveMessage(msg); err != nil {
			t.Fatalf("SaveMessage failed: %v", err)
		}
	}

	// 3. Verify total count and last message
	if cnt := cdb.GetMessageCount("group-1"); cnt != 5 {
		t.Errorf("expected count 5, got %d", cnt)
	}
	last := cdb.GetLastMessage("group-1")
	if last == nil || last.Content != "hello 5" {
		t.Fatalf("expected last message 'hello 5', got %+v", last)
	}

	// 4. Test normal pagination: limit 3 (should return [3, 4, 5], hasMore=true, total=5)
	msgs, hasMore, total = cdb.GetMessagesPaged("group-1", 3, 0, 0)
	if total != 5 || !hasMore || len(msgs) != 3 {
		t.Fatalf("expected 3 msgs, hasMore=true, total=5, got %d, %v, %d", len(msgs), hasMore, total)
	}
	if msgs[0].Content != "hello 3" || msgs[1].Content != "hello 4" || msgs[2].Content != "hello 5" {
		t.Fatalf("unexpected msgs content: %v, %v, %v", msgs[0].Content, msgs[1].Content, msgs[2].Content)
	}

	// 5. Test history pagination: before = 1003, limit = 2 (should return [1, 2], hasMore=false, total=5)
	msgs, hasMore, total = cdb.GetMessagesPaged("group-1", 2, 0, 1003)
	if total != 5 || len(msgs) != 2 {
		t.Fatalf("expected 2 msgs, total=5, got len=%d", len(msgs))
	}
	if msgs[0].Content != "hello 1" || msgs[1].Content != "hello 2" {
		t.Fatalf("unexpected before content: %v, %v", msgs[0].Content, msgs[1].Content)
	}

	// 6. Test incremental update: since = 1003 (should return [4, 5])
	msgs, _, _ = cdb.GetMessagesPaged("group-1", 10, 1003, 0)
	if len(msgs) != 2 || msgs[0].Content != "hello 4" || msgs[1].Content != "hello 5" {
		t.Fatalf("unexpected since msgs: %v, %v", msgs[0].Content, msgs[1].Content)
	}

	// 7. Clear session
	if err := cdb.ClearSession("group-1"); err != nil {
		t.Fatalf("ClearSession failed: %v", err)
	}
	if cnt := cdb.GetMessageCount("group-1"); cnt != 0 {
		t.Fatalf("expected 0 after clear, got %d", cnt)
	}
}

func TestChatDB_PrivateSessions(t *testing.T) {
	cdb, err := OpenChatDB(":memory:")
	if err != nil {
		t.Fatalf("OpenChatDB(:memory:) failed: %v", err)
	}
	defer cdb.Close()

	now := time.Now().Unix()

	// Insert private messages for user A (Official QQ) and user B (Regular QQ)
	_ = cdb.SaveMessage(&ChatMessage{
		ID:            generateChatMsgID(),
		SessionType:   "private",
		SessionID:     "OpenQQ:12345678-ABCD",
		SessionName:   "李四",
		SenderID:      "OpenQQ:12345678-ABCD",
		SenderName:    "李四",
		SenderDisplay: "李四 (ABCD)",
		SenderSuffix:  "ABCD",
		IsOfficial:    true,
		Content:       "私聊消息 1",
		Timestamp:     now - 10,
	})

	_ = cdb.SaveMessage(&ChatMessage{
		ID:            generateChatMsgID(),
		SessionType:   "private",
		SessionID:     "OpenQQ:12345678-ABCD",
		SessionName:   "李四",
		SenderID:      "bot-id",
		SenderName:    "骰娘",
		SenderDisplay: "骰娘",
		IsBot:         true,
		Content:       "回复私聊 1",
		Timestamp:     now - 5,
	})

	_ = cdb.SaveMessage(&ChatMessage{
		ID:            generateChatMsgID(),
		SessionType:   "private",
		SessionID:     "QQ:987654321",
		SessionName:   "王五",
		SenderID:      "QQ:987654321",
		SenderName:    "王五",
		SenderDisplay: "王五",
		Content:       "你好呀",
		Timestamp:     now - 2,
	})

	// Query private sessions
	summaries, total, err := cdb.GetRecentSessions("private", 10, 0, "")
	if err != nil {
		t.Fatalf("GetRecentSessions failed: %v", err)
	}
	if total != 2 || len(summaries) != 2 {
		t.Fatalf("expected 2 private sessions, got total=%d len=%d", total, len(summaries))
	}

	// Check order (QQ:987654321 has newest timestamp now-2, so it should be first)
	if summaries[0].GroupID != "QQ:987654321" || summaries[0].GroupName != "王五" {
		t.Errorf("expected QQ:987654321 first, got %+v", summaries[0])
	}
	if summaries[1].GroupID != "OpenQQ:12345678-ABCD" || !summaries[1].IsOfficial || summaries[1].SenderSuffix != "ABCD" {
		t.Errorf("expected OpenQQ:12345678-ABCD second with suffix ABCD, got %+v", summaries[1])
	}
	if summaries[1].MsgCount != 2 {
		t.Errorf("expected 2 messages for OpenQQ user, got %d", summaries[1].MsgCount)
	}

	// Test search filter
	summariesSearch, totalSearch, err := cdb.GetRecentSessions("private", 10, 0, "李四")
	if err != nil || totalSearch != 1 || len(summariesSearch) != 1 {
		t.Fatalf("expected 1 search result, got %d (%v)", totalSearch, err)
	}
	if summariesSearch[0].GroupName != "李四" {
		t.Errorf("expected 李四, got %s", summariesSearch[0].GroupName)
	}
}
