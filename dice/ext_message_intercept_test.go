package dice

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func newMessageInterceptTestContext(command string, intercept bool) (*MsgContext, *Message, *mockPlatformAdapter, *ExtInfo) {
	adapter := newMockPlatformAdapter()
	d := &Dice{Logger: zap.NewNop().Sugar(), DirtyGroups: new(SyncMap[string, int64]), ExtUpdateTime: 1}
	d.Config = NewConfig(d)
	session := &IMSession{Parent: d}
	ep := &EndPointInfo{Adapter: adapter}
	ext := &ExtInfo{
		Name:                     "message-intercept-test",
		MessageInterceptCommands: []string{"help"},
	}
	ext.OnMessageSendIntercept = func(_ *MsgContext, _ *Message, _ string, _ string, _ string, _ string) bool {
		return intercept
	}
	group := &GroupInfo{Active: true}
	group.SetActivatedExtList([]*ExtInfo{ext}, d)
	ctx := &MsgContext{
		Dice:            d,
		Session:         session,
		EndPoint:        ep,
		Group:           group,
		IsCurGroupBotOn: true,
		CommandName:     command,
	}
	msg := &Message{MessageType: "group", GroupID: "test-group"}
	return ctx, msg, adapter, ext
}

func TestReplyMessageIntercept(t *testing.T) {
	ctx, msg, adapter, _ := newMessageInterceptTestContext("HELP", true)
	ReplyToSender(ctx, msg, "original")
	if reply, ok := adapter.waitForMsg(100 * time.Millisecond); ok {
		t.Fatalf("intercepted reply was sent: %q", reply)
	}
}

func TestReplyMessageInterceptCommandFilter(t *testing.T) {
	ctx, msg, adapter, _ := newMessageInterceptTestContext("roll", true)
	ReplyToSender(ctx, msg, "original")
	if reply, ok := adapter.waitForMsg(time.Second); !ok || reply != "original" {
		t.Fatalf("unmatched command reply = %q, received=%v, want original", reply, ok)
	}
}

func TestReplyToSenderBypassIntercept(t *testing.T) {
	ctx, msg, adapter, _ := newMessageInterceptTestContext("help", true)
	ReplyToSenderBypassIntercept(ctx, msg, "image")
	if reply, ok := adapter.waitForMsg(time.Second); !ok || reply != "image" {
		t.Fatalf("bypass reply = %q, received=%v, want image", reply, ok)
	}
}

func TestBuiltinCommandSetsReplyInterceptContext(t *testing.T) {
	ctx, msg, adapter, ext := newMessageInterceptTestContext("", true)
	ctx.Session.Parent.CmdMap = CmdMapCls{
		"help": {
			Name: "help",
			Solve: func(_ *MsgContext, _ *Message, _ *CmdArgs) CmdExecuteResult {
				ReplyToSender(ctx, msg, "core help")
				return CmdExecuteResult{Solved: true}
			},
		},
	}
	ctx.Group.SetActivatedExtList([]*ExtInfo{ext}, ctx.Dice)
	ctx.Session.commandSolve(ctx, msg, &CmdArgs{Command: "help", RawArgs: "roll"})
	if reply, ok := adapter.waitForMsg(100 * time.Millisecond); ok {
		t.Fatalf("builtin command reply was not intercepted: %q", reply)
	}
	if ctx.CommandName != "" || ctx.CommandRawArgs != "" {
		t.Fatalf("command context leaked after solve: %q %q", ctx.CommandName, ctx.CommandRawArgs)
	}
}

func TestExtensionCommandSetsReplyInterceptContext(t *testing.T) {
	ctx, msg, adapter, ext := newMessageInterceptTestContext("", true)
	ext.MessageInterceptCommands = []string{"ra"}
	ext.DefaultSetting = &ExtDefaultSettingItem{DisabledCommand: map[string]bool{}}
	ext.CmdMap = CmdMapCls{
		"ra": {
			Name: "ra",
			Solve: func(_ *MsgContext, _ *Message, _ *CmdArgs) CmdExecuteResult {
				ReplyToSender(ctx, msg, "extension reply")
				return CmdExecuteResult{Solved: true}
			},
		},
	}
	ctx.Group.SetActivatedExtList([]*ExtInfo{ext}, ctx.Dice)
	ctx.Session.commandSolve(ctx, msg, &CmdArgs{Command: "ra", RawArgs: "侦查"})
	if reply, ok := adapter.waitForMsg(100 * time.Millisecond); ok {
		t.Fatalf("extension command reply was not intercepted: %q", reply)
	}
	if ctx.CommandName != "" || ctx.CommandRawArgs != "" {
		t.Fatalf("command context leaked after extension solve: %q %q", ctx.CommandName, ctx.CommandRawArgs)
	}
}
