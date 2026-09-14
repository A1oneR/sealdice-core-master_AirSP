package dice

import (
	"path/filepath"
	"testing"

	"sealdice-core/dice/service"
	"sealdice-core/model"
	"sealdice-core/utils/constant"
)

func TestCleanLogMessage(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal plain text message",
			input:    "<木落> 掷骰 1D100: 45/50 (困难成功)",
			expected: "<木落> 掷骰 1D100: 45/50 (困难成功)",
		},
		{
			name: "HP and SAN vector bars with character change",
			input: `$\text{HP: }\textcolor{#10B981}{\rule{65px}{7px}}\textcolor{#374151}{\rule{0px}{7px}}\ \small \text{14/14}\quad\text{SAN: }\textcolor{#0EA5E9}{\rule{42px}{7px}}\textcolor{#374151}{\rule{23px}{7px}}\ \small \text{65/99}$
<普利姆·银石>的属性变化:
$\text{“}\textbf{\text{生命值}}\text{” }\text{13} \xrightarrow[\text{增加1}]{} \textcolor{#10B981}{\textbf{\text{14}}}$
[√] 已绑卡`,
			expected: `HP: 14/14 SAN: 65/99
<普利姆·银石>的属性变化:
“生命值” 13 ➯ (增加1) 14
[√] 已绑卡`,
		},
		{
			name:     "Colorbox pill badge with bold title",
			input:    `$\colorbox{#DBEAFE}{\textcolor{#1E40AF}{\textbf{\text{ <普利姆·银石> }}}}\ \textbf{\text{的属性变化:}}$`,
			expected: `<普利姆·银石> 的属性变化:`,
		},
		{
			name: "Complex boss raid status",
			input: `$$ \fcolorbox{#B71C1C}{#FFEBEE}{\textcolor{#B71C1C}{\textbf{ BOSS RAID 讨伐进行中 }}} \tag*{\fcolorbox{#212121}{#424242}{\textcolor{white}{\small \text{阶段二}}}} $$

$\text{【讨伐目标】}\overset{\substack{\textcolor{red}{\tiny ★ 远古君王 ★} \\ \textcolor{gray}{\tiny [Lv.95]}}}{\textbf{\text{炼狱炎魔 · 厄里斯}}}$
$\text{生命值: }\textcolor{#F44336}{\rule{120px}{8px}}\textcolor{#616161}{\rule{30px}{8px}}\ \small \text{16.5W / 20W}\ \left( \textcolor{red}{\searrow 17\%} \right)$

$\text{战斗链路：}\text{先锋突刺} \xtwoheadrightarrow{\text{全速冲锋}} \text{破盾} \xrightarrow[\text{护甲穿透 50\%}]{\textcolor{orange}{\text{蓄力斩击}}} \textcolor{red}{\textbf{\text{暴击!}}}$`,
			expected: `BOSS RAID 讨伐进行中 [ 阶段二]

【讨伐目标】炼狱炎魔 · 厄里斯
生命值: 16.5W / 20W ( ↘ 17% )

战斗链路：先锋突刺 ➯ (全速冲锋) 破盾 ➯ [蓄力斩击] (护甲穿透 50%) 暴击!`,
		},
		{
			name:     "Discount cancelto macro",
			input:    `$\text{价格：}\cancelto{\textcolor{#DC2626}{\textbf{\text{1,200 金币}}}}{\text{3,000 金币}}$`,
			expected: `价格：3,000 金币 ➯ 1,200 金币`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := CleanLogMessage(c.input)
			if actual != c.expected {
				t.Errorf("\nExpected:\n%q\nGot:\n%q", c.expected, actual)
			}
		})
	}
}

func TestLogAppendCleansKaTeXMessage(t *testing.T) {
	mockDB, err := newMockDatabaseOperator(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("newMockDatabaseOperator: %v", err)
	}
	t.Cleanup(mockDB.Close)

	db := mockDB.GetLogDB(constant.WRITE)
	if err := db.AutoMigrate(&model.LogInfo{}, &model.LogOneItem{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	diceObj := &Dice{
		DBOperator: mockDB,
		Config: Config{
			StoryLogConfig: StoryLogConfig{
				LogCleanRichText: true,
			},
		},
	}
	mctx := &MsgContext{Dice: diceObj}
	groupID := "QQ-Group:2001"
	logName := "test-story"

	rawInput := `$\text{HP: }\textcolor{#10B981}{\rule{65px}{7px}}\ \small \text{14/14}$
<测试员>的属性变化:
$\text{“}\textbf{\text{生命值}}\text{” }\text{13} \xrightarrow[\text{增加1}]{} \textcolor{#10B981}{\textbf{\text{14}}}$`

	ok := LogAppend(mctx, groupID, 0, logName, &model.LogOneItem{
		Nickname: "骰娘",
		IMUserID: "10001",
		Message:  rawInput,
		IsDice:   true,
	})
	if !ok {
		t.Fatalf("LogAppend failed")
	}

	lines, err := service.LogGetAllLines(mockDB, groupID, logName)
	if err != nil {
		t.Fatalf("LogGetAllLines failed: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	expectedCleaned := `HP: 14/14
<测试员>的属性变化:
“生命值” 13 ➯ (增加1) 14`

	if lines[0].Message != expectedCleaned {
		t.Errorf("DB saved message mismatch:\nExpected:\n%q\nGot:\n%q", expectedCleaned, lines[0].Message)
	}
}
