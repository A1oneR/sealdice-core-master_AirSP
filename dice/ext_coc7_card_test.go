package dice

import (
	"strings"
	"testing"
	"time"
)

func TestCocCardDamageBonus(t *testing.T) {
	tests := []struct {
		total int64
		want  string
	}{
		{64, "-2"}, {65, "-1"}, {85, "0"}, {125, "+1d4"},
		{165, "+1d6"}, {205, "+2d6"}, {285, "+3d6"},
	}
	for _, test := range tests {
		if got := cocCardDamageBonus(test.total); got != test.want {
			t.Errorf("cocCardDamageBonus(%d) = %q, want %q", test.total, got, test.want)
		}
	}
}

func TestCocCardCommandsUseCustomText(t *testing.T) {
	d, ep, adapter, cleanup := newExecuteNewTestDice(t)
	defer cleanup()

	tests := []struct {
		command string
		marker  string
	}{
		{".coc", "打败了"},
		{".cohero", "传奇人物作成"},
		{".coran", "随机人物作成"},
	}
	for _, test := range tests {
		d.ImSession.ExecuteNew(ep, newGroupMsg("QQ-Group:coc-card", "QQ:coc-card", test.command))
		reply, ok := adapter.waitForMsg(2 * time.Second)
		if !ok {
			t.Fatalf("timeout waiting for %s", test.command)
		}
		if !strings.Contains(reply, "力量:") || !strings.Contains(reply, test.marker) {
			t.Fatalf("unexpected %s reply: %q", test.command, reply)
		}
		if strings.Contains(reply, "%未知项-") {
			t.Fatalf("%s used an unregistered custom text item: %q", test.command, reply)
		}
	}
}
