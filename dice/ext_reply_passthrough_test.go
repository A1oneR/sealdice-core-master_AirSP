//nolint:testpackage
package dice

import (
	"testing"

	"go.uber.org/zap"
)

func TestCustomReplyPassthroughFunctions(t *testing.T) {
	testDice := &Dice{
		Logger: zap.NewNop().Sugar(),
	}
	testDice.Config.CustomReplyConfigEnable = true

	rc := &ReplyConfig{
		Enable:   true,
		Filename: "test_reply.yaml",
		Name:     "测试回复",
		Items: []*ReplyItem{
			{
				Enable: true,
				Conditions: []ReplyConditionBase{
					&ReplyConditionTextMatch{
						CondType:  "textMatch",
						MatchType: "matchExact",
						Value:     "今日运势",
					},
				},
				Results: []ReplyResultBase{
					&ReplyResultReplyToSender{
						ResultType: "replyToSender",
						Message: TextTemplateItemList{
							{"运势极佳！", 1},
						},
					},
				},
			},
			{
				Enable: true,
				Conditions: []ReplyConditionBase{
					&ReplyConditionTextMatch{
						CondType:  "textMatch",
						MatchType: "matchContains",
						Value:     "签到",
					},
				},
				Results: []ReplyResultBase{
					&ReplyResultReplyToSender{
						ResultType: "replyToSender",
						Message: TextTemplateItemList{
							{"签到成功！", 1},
						},
					},
				},
			},
		},
	}

	testDice.CustomReplyConfig = []*ReplyConfig{rc}

	mctx := &MsgContext{
		Dice:        testDice,
		MessageType: "group",
		Group: &GroupInfo{
			GroupID: "UI-Group:test_1",
			Active:  true,
		},
		Player: &GroupPlayerInfo{
			Name:   "测试员",
			UserID: "QQ:123456",
		},
		EndPoint: &EndPointInfo{
			EndPointInfoBase: EndPointInfoBase{
				UserID:   "QQ:10000",
				Nickname: "测试骰",
			},
		},
	}

	// 1. 测试 CustomReplyGetActiveRules
	rules := CustomReplyGetActiveRules(testDice)
	if len(rules) != 2 {
		t.Fatalf("CustomReplyGetActiveRules len = %d, want 2", len(rules))
	}

	// 2. 测试 CustomReplyCheckMatch
	if !CustomReplyCheckMatch(mctx, nil, "今日运势") {
		t.Errorf("CustomReplyCheckMatch('今日运势') = false, want true")
	}
	if !CustomReplyCheckMatch(mctx, nil, "我要签到啦") {
		t.Errorf("CustomReplyCheckMatch('我要签到啦') = false, want true")
	}
	if CustomReplyCheckMatch(mctx, nil, "未知无关文本") {
		t.Errorf("CustomReplyCheckMatch('未知无关文本') = true, want false")
	}

	// 3. 测试 CustomReplyDo
	msg := &Message{
		Message:     "/今日运势",
		MessageType: "group",
		GroupID:     "UI-Group:test_1",
		Sender: SenderBase{
			Nickname: "测试员",
			UserID:   "QQ:123456",
		},
	}

	// 传入剥离先导符后的文本 "今日运势"
	executed := CustomReplyDo(mctx, msg, "今日运势")
	if !executed {
		t.Errorf("CustomReplyDo with forceText '今日运势' = false, want true")
	}

	// 传入未命中文本
	executedNotFound := CustomReplyDo(mctx, msg, "不存在的指令")
	if executedNotFound {
		t.Errorf("CustomReplyDo with forceText '不存在的指令' = true, want false")
	}

	// 4. 测试 TriggerCommand 兜底自动回退触发自定义回复
	session := &IMSession{Parent: testDice}
	mctx.Session = session
	mctx.IsCurGroupBotOn = true
	mctx.Group.Active = true
	mctx.Group.LastCustomReplyTime = 0 // 重置冷却时间，避免被2秒冷却拦截

	cmdArgs := &CmdArgs{
		Command:   "今日运势",
		CleanArgs: "",
		RawText:   "/今日运势",
	}
	trigRet := mctx.EndPoint.TriggerCommand(mctx, msg, cmdArgs)
	if !trigRet {
		t.Errorf("TriggerCommand for unknown command matching custom reply = false, want true")
	}
}
