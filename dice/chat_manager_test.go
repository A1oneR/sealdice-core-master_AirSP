package dice

import (
	"fmt"
	"testing"
)

func TestFormatSenderDisplay(t *testing.T) {
	cases := []struct {
		name         string
		senderID     string
		nickname     string
		groupID      string
		isBot        bool
		wantDisplay  string
		wantSuffix   string
		wantOfficial bool
	}{
		{
			name:         "official qq with full member openid",
			senderID:     "OpenQQ:3889204361-A1B2C3D4",
			nickname:     "测试玩家",
			groupID:      "OpenQQ-Group:3889204361-G100",
			isBot:        false,
			wantDisplay:  "测试玩家 (C3D4)",
			wantSuffix:   "C3D4",
			wantOfficial: true,
		},
		{
			name:         "official qq default nickname with suffix already",
			senderID:     "OpenQQ:3889204361-9876",
			nickname:     "用户9876",
			groupID:      "OpenQQ-Group:3889204361-G100",
			isBot:        false,
			wantDisplay:  "用户9876",
			wantSuffix:   "9876",
			wantOfficial: true,
		},
		{
			name:         "official qq channel member",
			senderID:     "OpenQQCH:CH10-ABCD",
			nickname:     "频道成员",
			groupID:      "OpenQQCH-Channel:CH10",
			isBot:        false,
			wantDisplay:  "频道成员 (ABCD)",
			wantSuffix:   "ABCD",
			wantOfficial: true,
		},
		{
			name:         "official group but short sender id",
			senderID:     "OpenQQ:12",
			nickname:     "短号",
			groupID:      "OpenQQ-Group:G1",
			isBot:        false,
			wantDisplay:  "短号 (12)",
			wantSuffix:   "12",
			wantOfficial: true,
		},
		{
			name:         "normal qq member",
			senderID:     "QQ:123456789",
			nickname:     "跑团KP",
			groupID:      "QQ-Group:987654321",
			isBot:        false,
			wantDisplay:  "跑团KP",
			wantSuffix:   "",
			wantOfficial: false,
		},
		{
			name:         "bot message",
			senderID:     "OpenQQ:3889204361",
			nickname:     "海豹测试机",
			groupID:      "OpenQQ-Group:3889204361-G100",
			isBot:        true,
			wantDisplay:  "海豹测试机",
			wantSuffix:   "",
			wantOfficial: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			display, suffix, isOfficial := FormatSenderDisplay(tc.senderID, tc.nickname, tc.groupID, tc.isBot)
			if display != tc.wantDisplay {
				t.Errorf("FormatSenderDisplay() display = %q, want %q", display, tc.wantDisplay)
			}
			if suffix != tc.wantSuffix {
				t.Errorf("FormatSenderDisplay() suffix = %q, want %q", suffix, tc.wantSuffix)
			}
			if isOfficial != tc.wantOfficial {
				t.Errorf("FormatSenderDisplay() isOfficial = %v, want %v", isOfficial, tc.wantOfficial)
			}
		})
	}
}

func TestChatManagerMessageFlow(t *testing.T) {
	d := &Dice{}
	cm := NewChatManager(d)

	groupID := "OpenQQ-Group:1234-test"

	// 1. Add incoming member message
	msg1 := &Message{
		GroupID:     groupID,
		GroupName:   "测试冒险团",
		MessageType: "group",
		Message:     ".r 1d100 检定",
		Time:        1000,
		Sender: SenderBase{
			UserID:   "OpenQQ:1234-AABB",
			Nickname: "调查员A",
		},
	}
	recorded1 := cm.AddIncomingMessage(nil, msg1)
	if recorded1 == nil {
		t.Fatal("AddIncomingMessage returned nil")
	}
	if recorded1.SenderSuffix != "AABB" || recorded1.SenderDisplay != "调查员A (AABB)" {
		t.Fatalf("unexpected formatted display: %#v", recorded1)
	}
	if recorded1.IsBot {
		t.Fatal("incoming message must not be marked as bot")
	}

	// 2. Add outgoing bot reply
	msg2 := &Message{
		GroupID:     groupID,
		GroupName:   "测试冒险团",
		MessageType: "group",
		Message:     "调查员A 掷出了 1D100=42 / 成功",
		Time:        1001,
		Sender: SenderBase{
			UserID:   "OpenQQ:1234",
			Nickname: "海豹核心",
		},
	}
	recorded2 := cm.AddOutgoingMessage(nil, msg2)
	if recorded2 == nil || !recorded2.IsBot {
		t.Fatalf("outgoing message must be marked as bot: %#v", recorded2)
	}

	// 3. Retrieve all messages
	list := cm.GetMessages(groupID, 10, 0)
	if len(list) != 2 {
		t.Fatalf("GetMessages count = %d, want 2", len(list))
	}
	if list[0].Content != msg1.Message || list[1].Content != msg2.Message {
		t.Fatalf("unexpected message list order or content: %#v", list)
	}

	// 4. Incremental query with since
	incList := cm.GetMessages(groupID, 10, 1000)
	if len(incList) != 1 || incList[0].Content != msg2.Message {
		t.Fatalf("GetMessages with since=1000 returned %#v, want only msg2", incList)
	}

	// 5. Last message check
	last := cm.GetLastMessage(groupID)
	if last == nil || last.ID != recorded2.ID {
		t.Fatalf("GetLastMessage mismatch: %#v", last)
	}

	// 6. Test capacity trimming
	cm.maxPerGroup = 5
	for i := 0; i < 10; i++ {
		cm.AddIncomingMessage(nil, &Message{
			GroupID:     groupID,
			MessageType: "group",
			Message:     fmt.Sprintf("msg-%d", i),
			Time:        int64(2000 + i),
			Sender: SenderBase{
				UserID:   "OpenQQ:1234-AABB",
				Nickname: "调查员A",
			},
		})
	}
	trimmedList := cm.GetMessages(groupID, 100, 0)
	if len(trimmedList) != 5 {
		t.Fatalf("trimmed message count = %d, want 5", len(trimmedList))
	}
	if trimmedList[len(trimmedList)-1].Content != "msg-9" {
		t.Fatalf("latest trimmed message = %s, want msg-9", trimmedList[len(trimmedList)-1].Content)
	}

	// 7. Clear messages
	cm.ClearMessages(groupID)
	if cm.GetMessageCount(groupID) != 0 {
		t.Fatal("expected message count to be 0 after ClearMessages")
	}
}

func TestChatManagerGetMessagesPaged(t *testing.T) {
	d := &Dice{}
	cm := NewChatManager(d)
	groupID := "QQ-Group:8888"

	for i := 1; i <= 10; i++ {
		cm.AddIncomingMessage(nil, &Message{
			GroupID:     groupID,
			MessageType: "group",
			Message:     fmt.Sprintf("msg-%d", i),
			Time:        int64(1000 + i*10),
			Sender: SenderBase{
				UserID:   "QQ:10001",
				Nickname: "玩家",
			},
		})
	}

	// 1. Initial page: limit 4 latest messages
	msgs, hasMore, total := cm.GetMessagesPaged(groupID, 4, 0, 0)
	if total != 10 {
		t.Fatalf("total = %d, want 10", total)
	}
	if !hasMore {
		t.Fatal("expected hasMore to be true")
	}
	if len(msgs) != 4 {
		t.Fatalf("len(msgs) = %d, want 4", len(msgs))
	}
	if msgs[0].Content != "msg-7" || msgs[3].Content != "msg-10" {
		t.Fatalf("unexpected msgs: %v .. %v", msgs[0].Content, msgs[3].Content)
	}

	// 2. Load earlier page before msg-7's timestamp (1070)
	earlierMsgs, earlierHasMore, earlierTotal := cm.GetMessagesPaged(groupID, 4, 0, 1070)
	if earlierTotal != 10 {
		t.Fatalf("earlierTotal = %d, want 10", earlierTotal)
	}
	if !earlierHasMore {
		t.Fatal("expected earlierHasMore to be true (since 6 msgs before 1070)")
	}
	if len(earlierMsgs) != 4 {
		t.Fatalf("len(earlierMsgs) = %d, want 4", len(earlierMsgs))
	}
	if earlierMsgs[0].Content != "msg-3" || earlierMsgs[3].Content != "msg-6" {
		t.Fatalf("unexpected earlierMsgs: %v .. %v", earlierMsgs[0].Content, earlierMsgs[3].Content)
	}

	// 3. Load next earlier page before msg-3's timestamp (1030)
	oldestMsgs, oldestHasMore, _ := cm.GetMessagesPaged(groupID, 4, 0, 1030)
	if oldestHasMore {
		t.Fatal("expected oldestHasMore to be false (only 2 msgs left)")
	}
	if len(oldestMsgs) != 2 {
		t.Fatalf("len(oldestMsgs) = %d, want 2", len(oldestMsgs))
	}
	if oldestMsgs[0].Content != "msg-1" || oldestMsgs[1].Content != "msg-2" {
		t.Fatalf("unexpected oldestMsgs: %v .. %v", oldestMsgs[0].Content, oldestMsgs[1].Content)
	}
}

func TestCleanFaceTypeTags(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "sticker with image attachment CQ code",
			input: `<faceType=6,faceId="0",ext="eyJ0ZXh0IjoiW+ivtl0ifQ==">[CQ:image,file=https://example.com/sticker.png]`,
			want:  `[CQ:image,file=https://example.com/sticker.png]`,
		},
		{
			name:  "sticker with empty text and image",
			input: `<faceType=6,faceId="0",ext="eyJ0ZXh0IjoiIn0=">[CQ:image,file=https://example.com/sticker.png]`,
			want:  `[CQ:image,file=https://example.com/sticker.png]`,
		},
		{
			name:  "sticker with seal native img format",
			input: `<faceType=6,faceId="0",ext="eyJ0ZXh0IjoiW+ivtl0ifQ==">[图:data/images/1.png]`,
			want:  `[图:data/images/1.png]`,
		},
		{
			name:  "standalone face with text yao",
			input: `<faceType=4,faceId="",ext="eyJ0ZXh0IjoiW+WSrF0ifQ==">`,
			want:  `[咬]`,
		},
		{
			name:  "standalone face with text si le",
			input: `<faceType=4,faceId="",ext="eyJ0ZXh0IjoiW+S8vOS6hl0ifQ==">`,
			want:  `[似了]`,
		},
		{
			name:  "standalone face with escaped quotes",
			input: `<faceType=6,faceId=\"0\",ext=\"eyJ0ZXh0IjoiW+ivtl0ifQ==\">`,
			want:  `[诶]`,
		},
		{
			name:  "standalone face with empty text fallback",
			input: `<faceType=6,faceId="0",ext="eyJ0ZXh0IjoiIn0=">`,
			want:  `[表情]`,
		},
		{
			name:  "text with standalone face",
			input: `你好 <faceType=4,faceId="",ext="eyJ0ZXh0IjoiW+WSrF0ifQ=="> 世界`,
			want:  `你好 [咬] 世界`,
		},
		{
			name:  "normal text without face tag",
			input: `普通聊天内容 12345`,
			want:  `普通聊天内容 12345`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CleanFaceTypeTags(tc.input)
			if got != tc.want {
				t.Errorf("CleanFaceTypeTags(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
