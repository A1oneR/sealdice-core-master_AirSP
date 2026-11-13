package dice

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
	ds "github.com/sealdice/dicescript"
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
		if test.command == ".cohero" && strings.Contains(reply, "\n0") {
			t.Fatalf("%s rendered a numeric zero instead of the heroic review text: %q", test.command, reply)
		}
	}
}

func TestCocCardCommandsKeepLegacyResultVariables(t *testing.T) {
	d, ep, adapter, cleanup := newExecuteNewTestDice(t)
	defer cleanup()

	heroicText := TextTemplateItemList{{"旧版传奇文案:\n{$t制卡结果文本2}", 1}}
	randomText := TextTemplateItemList{{"旧版随机文案:\n{$t制卡结果文本3}", 1}}
	d.TextMap["COC:制卡传奇"] = heroicText.toRandomPool()
	d.TextMap["COC:制卡随机"] = randomText.toRandomPool()

	for _, command := range []string{".cohero", ".coran"} {
		d.ImSession.ExecuteNew(ep, newGroupMsg("QQ-Group:coc-card-legacy", "QQ:coc-card-legacy", command))
		reply, ok := adapter.waitForMsg(2 * time.Second)
		if !ok {
			t.Fatalf("timeout waiting for %s", command)
		}
		if !strings.Contains(reply, "力量:") || strings.HasSuffix(strings.TrimSpace(reply), "0") {
			t.Fatalf("legacy custom text variable was not populated for %s: %q", command, reply)
		}
	}
}

func TestFormatCoCCandidates(t *testing.T) {
	tests := []struct {
		detail     string
		wantOK     bool
		wantLatex  string
		wantMD     string
	}{
		{
			detail:    "D100=82, 奖励 7",
			wantOK:    true,
			wantLatex: `[\cancel{\text{82}},\ \textbf{\text{72}}]`,
			wantMD:    `[~~82~~, 72]`,
		},
		{
			detail:    "72[D100=82, 奖励 7 8]",
			wantOK:    true,
			wantLatex: `[\cancel{\text{82}},\ \textbf{\text{72}},\ \cancel{\text{82}}]`,
			wantMD:    `[~~82~~, 72, ~~82~~]`,
		},
		{
			detail:    "D100=26, 惩罚 9 2",
			wantOK:    true,
			wantLatex: `[\cancel{\text{26}},\ \textbf{\text{96}},\ \cancel{\text{26}}]`,
			wantMD:    `[~~26~~, 96, ~~26~~]`,
		},
		{
			detail:    "(D100=100, 奖励 2)",
			wantOK:    true,
			wantLatex: `[\cancel{\text{100}},\ \textbf{\text{20}}]`,
			wantMD:    `[~~100~~, 20]`,
		},
		{
			detail:    "(D100=20, 惩罚 0)",
			wantOK:    true,
			wantLatex: `[\cancel{\text{20}},\ \textbf{\text{100}}]`,
			wantMD:    `[~~20~~, 100]`,
		},
		{
			detail:    "",
			wantOK:    false,
			wantLatex: "",
			wantMD:    "",
		},
	}

	for _, tc := range tests {
		lat, md, ok := FormatCoCCandidates(tc.detail)
		if ok != tc.wantOK {
			t.Fatalf("FormatCoCCandidates(%q) ok = %v, want %v", tc.detail, ok, tc.wantOK)
		}
		if ok {
			if lat != tc.wantLatex {
				t.Errorf("FormatCoCCandidates(%q) lat = %q, want %q", tc.detail, lat, tc.wantLatex)
			}
			if md != tc.wantMD {
				t.Errorf("FormatCoCCandidates(%q) md = %q, want %q", tc.detail, md, tc.wantMD)
			}
			// 验证 CleanLogMessage 清洗
			cleaned := CleanLogMessage(lat)
			if cleaned != tc.wantMD {
				t.Errorf("CleanLogMessage(%q) = %q, want %q", lat, cleaned, tc.wantMD)
			}
		}
	}
}

func TestFormatCoCDetailKaTeX(t *testing.T) {
	tests := []struct {
		detail    string
		outcome   int64
		want      string
		wantClean string
	}{
		{
			detail:    "D100=82, 奖励 7",
			outcome:   72,
			want:      `,\ (\text{D100}=\cancel{\text{82}},\ \text{奖励\ }\textbf{\text{7}})`,
			wantClean: `, (D100=~~82~~, 奖励 7)`,
		},
		{
			detail:    "D100=24, 奖励 7",
			outcome:   24,
			want:      `,\ (\text{D100}=\textbf{\text{24}},\ \text{奖励\ }\cancel{\text{7}})`,
			wantClean: `, (D100=24, 奖励 ~~7~~)`,
		},
		{
			detail:    "D100=82, 奖励 7 3",
			outcome:   32,
			want:      `,\ (\text{D100}=\cancel{\text{82}},\ \text{奖励\ }\cancel{\text{7}}\ \textbf{\text{3}})`,
			wantClean: `, (D100=~~82~~, 奖励 ~~7~~ 3)`,
		},
		{
			detail:    "D100=26, 惩罚 9 2",
			outcome:   96,
			want:      `,\ (\text{D100}=\cancel{\text{26}},\ \text{惩罚\ }\textbf{\text{9}}\ \cancel{\text{2}})`,
			wantClean: `, (D100=~~26~~, 惩罚 9 ~~2~~)`,
		},
		{
			detail:    "(D100=100, 奖励 2)",
			outcome:   20,
			want:      `,\ (\text{D100}=\cancel{\text{100}},\ \text{奖励\ }\textbf{\text{2}})`,
			wantClean: `, (D100=~~100~~, 奖励 2)`,
		},
		{
			detail:    "",
			outcome:   50,
			want:      "",
			wantClean: "",
		},
	}

	for _, tc := range tests {
		got := FormatCoCDetailKaTeX(tc.detail, tc.outcome)
		if got != tc.want {
			t.Errorf("FormatCoCDetailKaTeX(%q, %d) = %q, want %q", tc.detail, tc.outcome, got, tc.want)
		}
		cleaned := CleanLogMessage(got)
		if cleaned != tc.wantClean {
			t.Errorf("CleanLogMessage(%q) = %q, want %q", got, cleaned, tc.wantClean)
		}
	}
}

func TestCocCheckFormatting(t *testing.T) {
	d, ep, adapter, cleanup := newExecuteNewTestDice(t)
	defer cleanup()

	tmplText := "{%\n" +
		"$t输出 = \"\";\n" +
		"$t出目 = toStr($tD100文本 != 0 && $tD100文本 != \"\" ? $tD100文本 : ($t骰子出目 != 0 ? $t骰子出目 : $tD100));\n" +
		"$t线 = toStr($t判定值文本 != 0 && $t判定值文本 != \"\" ? $t判定值文本 : $t判定值);\n" +
		"$t过程 = toStr($t检定计算过程 != 0 ? $t检定计算过程 : \"\");\n" +
		"$t过程K = toStr($t检定计算过程_KaTeX != 0 ? $t检定计算过程_KaTeX : \"\");\n" +
		"$t结果 = toStr($t判定结果 != 0 ? $t判定结果 : \"\");\n" +
		"$t表达式 = toStr($t检定表达式文本 != 0 ? $t检定表达式文本 : \"\");\n" +
		"if $g特殊模块序号 == 1 {\n" +
		"\t$t颜色 = \"#EF4444\";\n" +
		"\tif $tSuccessRank == 4 { $t颜色 = \"#F59E0B\" };\n" +
		"\tif $tSuccessRank == 3 { $t颜色 = \"#A855F7\" };\n" +
		"\tif $tSuccessRank == 2 { $t颜色 = \"#3B82F6\" };\n" +
		"\tif $tSuccessRank == 1 { $t颜色 = \"#10B981\" };\n" +
		"\tif $tSuccessRank == -2 { $t颜色 = \"#991B1B\" };\n" +
		"\t$t输出 = \"$\\\\text{\" + $t表达式 + \"} = \\\\textcolor{\" + $t颜色 + \"}{\\\\textbf{\\\\text{\" + $t出目 + \"}}} / \\\\text{\" + $t线 + \"}\" + $t过程K + \"\\\\quad \\\\textcolor{\" + $t颜色 + \"}{\\\\textbf{\\\\text{[\" + $t结果 + \"]}}}$\";\n" +
		"};\n" +
		"if $g特殊模块序号 != 1 {\n" +
		"\t$t输出 = $t表达式 + \"=\" + $t出目 + \"/\" + $t线 + $t过程 + \" \" + $t结果;\n" +
		"};\n" +
		"%}{$t输出}"

	checkList := TextTemplateItemList{{tmplText, 1}}
	d.TextMap["COC:检定_单项结果文本"] = checkList.toRandomPool()

	// 1. 测试常规模块 ($g特殊模块序号 = 0)
	d.ImSession.ExecuteNew(ep, newGroupMsg("QQ-Group:coc-check", "QQ:coc-check", ".set 特殊模块序号 0"))
	adapter.waitForMsg(1 * time.Second)

	d.ImSession.ExecuteNew(ep, newGroupMsg("QQ-Group:coc-check", "QQ:coc-check", ".ra 50"))
	reply, ok := adapter.waitForMsg(2 * time.Second)
	if !ok {
		t.Fatalf("timeout waiting for .ra 50")
	}
	t.Logf("Mode 0 .ra 50 reply: %s", reply)
	if strings.Contains(reply, `\textcolor`) || strings.Contains(reply, `$`) || strings.Contains(reply, `~~`) {
		t.Fatalf("Mode 0 should not contain KaTeX or markdown formatting: %s", reply)
	}

	d.ImSession.ExecuteNew(ep, newGroupMsg("QQ-Group:coc-check", "QQ:coc-check", ".ra b 50"))
	replyB, ok := adapter.waitForMsg(2 * time.Second)
	if !ok {
		t.Fatalf("timeout waiting for .ra b 50")
	}
	t.Logf("Mode 0 .ra b 50 reply: %s", replyB)
	if strings.Contains(replyB, `\textcolor`) || strings.Contains(replyB, `$`) || strings.Contains(replyB, `~~`) {
		t.Fatalf("Mode 0 bonus dice should not contain KaTeX or markdown tags: %s", replyB)
	}
	if !strings.Contains(replyB, "D100=") || !strings.Contains(replyB, "奖励") {
		t.Fatalf("Mode 0 bonus dice should contain original SeaDice parenthesis format: %s", replyB)
	}

	// 2. 测试特殊模块 ($g特殊模块序号 = 1)
	groupAttrs := lo.Must(d.AttrsManager.LoadById("QQ-Group:coc-check"))
	groupAttrs.Store("$g特殊模块序号", ds.NewIntVal(1))

	d.ImSession.ExecuteNew(ep, newGroupMsg("QQ-Group:coc-check", "QQ:coc-check", ".ra 50"))
	replyK, ok := adapter.waitForMsg(2 * time.Second)
	if !ok {
		t.Fatalf("timeout waiting for .ra 50 in mode 1")
	}
	t.Logf("Mode 1 .ra 50 reply: %s", replyK)
	if !strings.Contains(replyK, `\textcolor{`) || !strings.Contains(replyK, `$`) {
		t.Fatalf("Mode 1 should contain KaTeX tags: %s", replyK)
	}

	d.ImSession.ExecuteNew(ep, newGroupMsg("QQ-Group:coc-check", "QQ:coc-check", ".ra b 50"))
	replyKB, ok := adapter.waitForMsg(2 * time.Second)
	if !ok {
		t.Fatalf("timeout waiting for .ra b 50 in mode 1")
	}
	t.Logf("Mode 1 .ra b 50 reply: %s", replyKB)
	if !strings.Contains(replyKB, `\cancel{`) || !strings.Contains(replyKB, `\textcolor{`) || !strings.Contains(replyKB, `(\text{D100}=`) {
		t.Fatalf("Mode 1 bonus dice should contain KaTeX \\cancel and D100 parenthesis: %s", replyKB)
	}

	// 3. 测试日志清洗是否正常剥除 KaTeX
	cleanedReplyKB := CleanLogMessage(replyKB)
	t.Logf("Mode 1 .ra b 50 cleaned reply: %s", cleanedReplyKB)
	if strings.Contains(cleanedReplyKB, `\cancel`) || strings.Contains(cleanedReplyKB, `\textcolor`) || strings.Contains(cleanedReplyKB, `$`) {
		t.Fatalf("CleanLogMessage should remove all KaTeX commands: %s", cleanedReplyKB)
	}
}

func TestTextMapCompatibleCheck(t *testing.T) {
	d, ep, _, cleanup := newExecuteNewTestDice(t)
	defer cleanup()
	d.UIEndpoint = ep

	tmplText := "{%\n" +
		"$t输出 = \"\";\n" +
		"$t出目 = toStr($tD100文本 != 0 && $tD100文本 != \"\" ? $tD100文本 : ($t骰子出目 != 0 ? $t骰子出目 : $tD100));\n" +
		"$t线 = toStr($t判定值文本 != 0 && $t判定值文本 != \"\" ? $t判定值文本 : $t判定值);\n" +
		"$t过程 = toStr($t检定计算过程 != 0 ? $t检定计算过程 : \"\");\n" +
		"$t过程K = toStr($t检定计算过程_KaTeX != 0 ? $t检定计算过程_KaTeX : \"\");\n" +
		"$t结果 = toStr($t判定结果 != 0 ? $t判定结果 : \"\");\n" +
		"$t表达式 = toStr($t检定表达式文本 != 0 ? $t检定表达式文本 : \"\");\n" +
		"if $g特殊模块序号 == 1 {\n" +
		"\t$t颜色 = \"#EF4444\";\n" +
		"\tif $tSuccessRank == 4 { $t颜色 = \"#F59E0B\" };\n" +
		"\tif $tSuccessRank == 3 { $t颜色 = \"#A855F7\" };\n" +
		"\tif $tSuccessRank == 2 { $t颜色 = \"#3B82F6\" };\n" +
		"\tif $tSuccessRank == 1 { $t颜色 = \"#10B981\" };\n" +
		"\tif $tSuccessRank == -2 { $t颜色 = \"#991B1B\" };\n" +
		"\t$t输出 = \"$\\\\text{\" + $t表达式 + \"} = \\\\textcolor{\" + $t颜色 + \"}{\\\\textbf{\\\\text{\" + $t出目 + \"}}} / \\\\text{\" + $t线 + \"}\" + $t过程K + \"\\\\quad \\\\textcolor{\" + $t颜色 + \"}{\\\\textbf{\\\\text{[\" + $t结果 + \"]}}}$\";\n" +
		"};\n" +
		"if $g特殊模块序号 != 1 {\n" +
		"\t$t输出 = $t表达式 + \"=\" + $t出目 + \"/\" + $t线 + $t过程 + \" \" + $t结果;\n" +
		"};\n" +
		"%}{$t输出}"

	items := []TextTemplateItem{
		{tmplText, 1},
	}
	// 初始化 _textMapTestData2
	for k, v := range _textMapTestData {
		attrs := ds.ValueMap{}
		if err := json.Unmarshal([]byte(v), &attrs); err == nil {
			_textMapTestData2[k] = &attrs
		}
	}

	for _, k := range []string{"检定_单项结果文本", "理智检定_单项结果文本"} {
		TextMapCompatibleCheck(d, "COC", k, items)

		infoMap, ok := d.TextMapCompatible.Load("COC:" + k)
		if !ok {
			t.Fatalf("TextMapCompatible not found for COC:%s", k)
		}
		info, ok := infoMap.Load(tmplText)
		if !ok {
			t.Fatalf("info not found for tmplText in %s", k)
		}
		t.Logf("Key [%s] Compat info: Version=%s, ErrV2=%s, TextV2=%s, TextV1=%s", k, info.Version, info.ErrV2, info.TextV2, info.TextV1)
		if info.ErrV2 != "" {
			t.Errorf("Key [%s] has ErrV2: %s", k, info.ErrV2)
		}
	}
}

func TestCardPercentSnippet(t *testing.T) {
	snippet := `{% $t百分比 = $t制卡胜过百分比 != null ? toFloat(toStr($t制卡胜过百分比)) : 0; $t百分比文本 = ` + "`" + `{$t百分比}` + "`" + `; $t颜色 = "#92400E"; if $t百分比 > 25 { $t颜色 = "#9CA3AF" }; if $t百分比 > 45 { $t颜色 = "" }; if $t百分比 > 55 { $t颜色 = "#10B981" }; if $t百分比 > 65 { $t颜色 = "#3B82F6" }; if $t百分比 > 75 { $t颜色 = "#A855F7" }; if $t百分比 > 85 { $t颜色 = "#FACC15" }; if $t百分比 > 95 { $t颜色 = "#F97316" }; if $t百分比 > 99 { $t颜色 = "#EF4444" }; $t输出 = "打败了" + $t百分比文本 + "%的调查员"; if $g特殊模块序号 == 1 { $t输出 = "$\\text{打败了 }\\textbf{\\text{" + $t百分比文本 + "\\%}}\\text{ 的调查员}$"; if $t颜色 != "" { $t输出 = "$\\text{打败了 }\\textcolor{" + $t颜色 + "}{\\textbf{\\text{" + $t百分比文本 + "\\%}}}\\text{ 的调查员}$" } }; $t输出 %}`

	d, ep, _, cleanup := newExecuteNewTestDice(t)
	defer cleanup()
	_ = d

	msg := _MsgCreate("group", "")
	ctx := CreateTempCtx(ep, msg)
	ctx.CreateVmIfNotExists()

	testCases := []struct {
		val       float64
		mode      int64
		wantKatex bool
		wantColor string
	}{
		{50, 0, false, ""},
		{50, 1, true, ""}, // 45-55%: default color (no textcolor)
		{20, 1, true, "#92400E"},
		{30, 1, true, "#9CA3AF"},
		{60, 1, true, "#10B981"},
		{96, 1, true, "#F97316"},
	}

	for _, tc := range testCases {
		ctx.vm.Attrs.Store("$t制卡胜过百分比", ds.NewFloatVal(tc.val))
		ctx.vm.Attrs.Store("$g特殊模块序号", ds.NewIntVal(ds.IntType(tc.mode)))
		res, err := DiceFormatV2(ctx, snippet)
		if err != nil {
			t.Fatalf("DiceFormatV2 failed for val=%v: %v", tc.val, err)
		}
		t.Logf("val=%v mode=%v -> %s", tc.val, tc.mode, res)
		if tc.mode == 0 {
			if strings.Contains(res, "$") {
				t.Fatalf("Mode 0 should not have KaTeX: %s", res)
			}
		} else {
			if !strings.Contains(res, "$") {
				t.Fatalf("Mode 1 should have KaTeX: %s", res)
			}
			if tc.wantColor == "" {
				if strings.Contains(res, `\textcolor`) {
					t.Fatalf("45-55%% should not have textcolor: %s", res)
				}
				if !strings.Contains(res, `\textbf{\text{50\%}}`) {
					t.Fatalf("45-55%% should have default text: %s", res)
				}
			} else {
				if !strings.Contains(res, `\textcolor{`+tc.wantColor+`}`) {
					t.Fatalf("val=%v should have color %s: %s", tc.val, tc.wantColor, res)
				}
			}
		}
	}
}

func TestHpMaxPriority(t *testing.T) {
	tmplIncDec := `{$t头部="";
$tCON = 体质 != null ? 体质 : 0;
$tSIZ = 体型 != null ? 体型 : 0;
$tCM = 克苏鲁神话 != null ? 克苏鲁神话 : 0;
$tHP_Cur = 生命值 != null ? 生命值 : 0;
$tSAN_Cur = 理智 != null ? 理智 : 0;
$tPlayer = $t玩家 != null ? $t玩家 : "";

$tHP_Max_Custom = 0;
if hpmax != null && hpmax > 0 { $tHP_Max_Custom = hpmax };
if $tHP_Max_Custom <= 0 && 生命值上限 != null && 生命值上限 > 0 { $tHP_Max_Custom = 生命值上限 };
if $tHP_Max_Custom <= 0 && 血量上限 != null && 血量上限 > 0 { $tHP_Max_Custom = 血量上限 };
if $tHP_Max_Custom <= 0 && 生命上限 != null && 生命上限 > 0 { $tHP_Max_Custom = 生命上限 };

$tHP_Max = $tHP_Max_Custom > 0 ? $tHP_Max_Custom : (($tCON + $tSIZ) / 10);
if $tHP_Max <= 0 { $tHP_Max = $tHP_Cur > 0 ? $tHP_Cur : 12 };
if $tHP_Cur > $tHP_Max { $tHP_Cur = $tHP_Max };

$tSAN_Max = 99 - $tCM;
if $tSAN_Max <= 0 || $tSAN_Max > 99 { $tSAN_Max = 99 };
if $tSAN_Cur > $tSAN_Max { $tSAN_Cur = $tSAN_Max };

$tHP_Cur_s = ` + "`" + `{$tHP_Cur}` + "`" + `;
$tHP_Max_s = ` + "`" + `{$tHP_Max}` + "`" + `;
$tSAN_Cur_s = ` + "`" + `{$tSAN_Cur}` + "`" + `;
$tSAN_Max_s = ` + "`" + `{$tSAN_Max}` + "`" + `;

if $g特殊模块序号 == 1 {
    $tHP_W1 = ($tHP_Cur * 65) / $tHP_Max;
    if $tHP_W1 < 0 { $tHP_W1 = 0 };
    if $tHP_W1 > 65 { $tHP_W1 = 65 };
    $tHP_W2 = 65 - $tHP_W1;
    $tHP_Color = $tHP_Cur < ($tHP_Max / 3)
  ? "#EF4444"
  : ($tHP_Cur < ($tHP_Max / 2) ? "#F59E0B" : "#10B981");

    $tSAN_W1 = ($tSAN_Cur * 65) / $tSAN_Max;
    if $tSAN_W1 < 0 { $tSAN_W1 = 0 };
    if $tSAN_W1 > 65 { $tSAN_W1 = 65 };
    $tSAN_W2 = 65 - $tSAN_W1;
    $tSAN_Color = $tSAN_Cur < 20
  ? "#6B7280"
  : ($tSAN_Cur <= 40 ? "#A855F7" : "#0EA5E9");

    $tHP_W1_s = ` + "`" + `{$tHP_W1}` + "`" + `;
    $tHP_W2_s = ` + "`" + `{$tHP_W2}` + "`" + `;
    $tSAN_W1_s = ` + "`" + `{$tSAN_W1}` + "`" + `;
    $tSAN_W2_s = ` + "`" + `{$tSAN_W2}` + "`" + `;

    $t条块 = "$\\text{HP: }\\textcolor{" + $tHP_Color + "}{\\rule{" + $tHP_W1_s + "px}{7px}}\\textcolor{#374151}{\\rule{" + $tHP_W2_s + "px}{7px}}\\ \\small \\text{" + $tHP_Cur_s + "/" + $tHP_Max_s + "}\\quad\\text{SAN: }\\textcolor{" + $tSAN_Color + "}{\\rule{" + $tSAN_W1_s + "px}{7px}}\\textcolor{#374151}{\\rule{" + $tSAN_W2_s + "px}{7px}}\\ \\small \\text{" + $tSAN_Cur_s + "/" + $tSAN_Max_s + "}$";
    $t头部 = $t条块 + "\n" + $tPlayer + "的属性变化:";
} else {
    $tHP_Rate = ($tHP_Cur * 5) / $tHP_Max;
    $tHP_TextBar = "□□□□□";
    if $tHP_Rate >= 1 { $tHP_TextBar = "■□□□□" };
    if $tHP_Rate >= 2 { $tHP_TextBar = "■■□□□" };
    if $tHP_Rate >= 3 { $tHP_TextBar = "■■■□□" };
    if $tHP_Rate >= 4 { $tHP_TextBar = "■■■■□" };
    if $tHP_Rate >= 5 { $tHP_TextBar = "■■■■■" };

    $tSAN_Rate = ($tSAN_Cur * 5) / $tSAN_Max;
    $tSAN_TextBar = "□□□□□";
    if $tSAN_Rate >= 1 { $tSAN_TextBar = "■□□□□" };
    if $tSAN_Rate >= 2 { $tSAN_TextBar = "■■□□□" };
    if $tSAN_Rate >= 3 { $tSAN_TextBar = "■■■□□" };
    if $tSAN_Rate >= 4 { $tSAN_TextBar = "■■■■□" };
    if $tSAN_Rate >= 5 { $tSAN_TextBar = "■■■■■" };

    $t条块 = "【HP: " + $tHP_TextBar + " " + $tHP_Cur_s + "/" + $tHP_Max_s + "】 【SAN: " + $tSAN_TextBar + " " + $tSAN_Cur_s + "/" + $tSAN_Max_s + "】";
    $t头部 = $t条块 + "\n" + $tPlayer + "的属性变化:";
};
$t头部
}`

	d, ep, _, cleanup := newExecuteNewTestDice(t)
	defer cleanup()
	_ = d

	msg := _MsgCreate("group", "")
	ctx := CreateTempCtx(ep, msg)
	ctx.CreateVmIfNotExists()

	// 1. 测试常规COC：体质60，体型60，无hpmax -> 应该计算出 (60+60)/10 = 12
	ctx.vm.Attrs.Store("体质", ds.NewIntVal(60))
	ctx.vm.Attrs.Store("体型", ds.NewIntVal(60))
	ctx.vm.Attrs.Store("生命值", ds.NewIntVal(10))
	ctx.vm.Attrs.Store("理智", ds.NewIntVal(50))
	ctx.vm.Attrs.Store("$g特殊模块序号", ds.NewIntVal(1))

	res, err := DiceFormatV2(ctx, tmplIncDec)
	if err != nil {
		t.Fatalf("DiceFormatV2 failed: %v", err)
	}
	t.Logf("Normal CoC result: %s", res)
	if !strings.Contains(res, "10/12") {
		t.Fatalf("Should be 10/12, got: %s", res)
	}

	// 2. 测试通俗COC：设置 hpmax = 24，生命值 20 -> 应该优先抓到 20/24，而不是 (60+60)/10 = 12
	ctx.vm.Attrs.Store("hpmax", ds.NewIntVal(24))
	ctx.vm.Attrs.Store("生命值", ds.NewIntVal(20))

	resPulp, err := DiceFormatV2(ctx, tmplIncDec)
	if err != nil {
		t.Fatalf("DiceFormatV2 failed: %v", err)
	}
	t.Logf("Pulp CoC result: %s", resPulp)
	if !strings.Contains(resPulp, "20/24") {
		t.Fatalf("Should prioritize hpmax and be 20/24, got: %s", resPulp)
	}

	// 3. 测试中文别名 生命值上限 = 30
	ctx.vm.Attrs.Delete("hpmax")
	ctx.vm.Attrs.Store("生命值上限", ds.NewIntVal(30))
	ctx.vm.Attrs.Store("生命值", ds.NewIntVal(25))

	resAlias, err := DiceFormatV2(ctx, tmplIncDec)
	if err != nil {
		t.Fatalf("DiceFormatV2 failed: %v", err)
	}
	t.Logf("Alias result: %s", resAlias)
	if !strings.Contains(resAlias, "25/30") {
		t.Fatalf("Should prioritize 生命值上限 and be 25/30, got: %s", resAlias)
	}
}
