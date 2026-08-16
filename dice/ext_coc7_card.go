package dice

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	cocCardStrength = iota
	cocCardDexterity
	cocCardPower
	cocCardConstitution
	cocCardAppearance
	cocCardEducation
	cocCardSize
	cocCardIntelligence
	cocCardLuck
	cocCardAttrCount
)

var cocCardAttrNames = [cocCardAttrCount]string{
	"力量", "敏捷", "意志", "体质", "外貌", "教育", "体型", "智力", "幸运",
}

type cocCardSpec struct {
	expressions    [cocCardAttrCount]string
	reviewTemplate string
	withPercentile bool
	withExtremes   bool
}

var (
	cocCardStandardSpec = cocCardSpec{
		expressions: [cocCardAttrCount]string{
			"3d6*5", "3d6*5", "3d6*5", "3d6*5", "3d6*5",
			"(2d6+6)*5", "(2d6+6)*5", "(2d6+6)*5", "3d6*5",
		},
		reviewTemplate: "COC:制卡_普通_评价",
		withPercentile: true,
		withExtremes:   true,
	}
	cocCardHeroicSpec = cocCardSpec{
		expressions: [cocCardAttrCount]string{
			"4d6*5", "4d6*5", "5d6*5", "5d6*5", "3d6*5+5",
			"(3d6+6)*5", "(2d6+6)*5", "(3d6+6)*5", "4d6*5",
		},
		reviewTemplate: "COC:制卡_传奇_评价",
	}
	cocCardRandomSpec = cocCardSpec{
		expressions: [cocCardAttrCount]string{
			"1d20*5", "1d20*5", "1d20*5", "1d20*5", "1d20*5",
			"1d20*5", "1d20*5", "1d20*5", "1d20*5",
		},
		reviewTemplate: "COC:制卡_随机_评价",
	}
)

type cocCardValues [cocCardAttrCount]int64

func rollCocCard(ctx *MsgContext, spec cocCardSpec) (cocCardValues, error) {
	var card cocCardValues
	for index, expression := range spec.expressions {
		result := ctx.Eval(expression, nil)
		if result == nil || result.vm == nil || result.vm.Error != nil {
			return card, fmt.Errorf("roll %s failed", cocCardAttrNames[index])
		}
		value, ok := result.ReadInt()
		if !ok {
			return card, fmt.Errorf("roll %s returned a non-integer value", cocCardAttrNames[index])
		}
		card[index] = int64(value)
	}
	return card, nil
}

func cocCardDamageBonus(total int64) string {
	switch {
	case total < 65:
		return "-2"
	case total < 85:
		return "-1"
	case total < 125:
		return "0"
	case total < 165:
		return "+1d4"
	case total < 205:
		return "+1d6"
	default:
		return fmt.Sprintf("+%dd6", 2+(total-205)/80)
	}
}

func formatCocCard(ctx *MsgContext, card cocCardValues, spec cocCardSpec) string {
	var totalWithoutLuck int64
	for index, value := range card {
		VarSetValueInt64(ctx, "$t制卡"+cocCardAttrNames[index], value)
		if index != cocCardLuck {
			totalWithoutLuck += value
		}
	}
	totalWithLuck := totalWithoutLuck + card[cocCardLuck]
	VarSetValueInt64(ctx, "$t制卡生命值", (card[cocCardConstitution]+card[cocCardSize])/10)
	VarSetValueStr(ctx, "$t制卡伤害加值", cocCardDamageBonus(card[cocCardStrength]+card[cocCardSize]))
	VarSetValueInt64(ctx, "$t制卡总值", totalWithoutLuck)
	VarSetValueInt64(ctx, "$t制卡总值含幸运", totalWithLuck)

	var reviewParts []string
	if review := strings.TrimSpace(DiceFormatTmpl(ctx, spec.reviewTemplate)); review != "" {
		reviewParts = append(reviewParts, review)
	}
	if spec.withPercentile {
		const mean = 510.0
		const stdDev = 41.83
		zScore := (float64(totalWithLuck) - mean) / stdDev
		beatRate := 0.5 * (1 + math.Erf(zScore/math.Sqrt2)) * 100
		beatRate = min(99.99, max(0, beatRate))
		VarSetValueStr(ctx, "$t制卡胜过百分比", fmt.Sprintf("%.2f", beatRate))
		if percentile := strings.TrimSpace(DiceFormatTmpl(ctx, "COC:制卡_胜过调查员")); percentile != "" {
			reviewParts = append(reviewParts, percentile)
		}
	}
	if spec.withExtremes {
		for index, value := range card {
			if value != 90 && value != 15 {
				continue
			}
			VarSetValueStr(ctx, "$t制卡属性名", cocCardAttrNames[index])
			template := "COC:制卡_极端属性_高"
			if value == 15 {
				template = "COC:制卡_极端属性_低"
			}
			if comment := strings.TrimSpace(DiceFormatTmpl(ctx, template)); comment != "" {
				reviewParts = append(reviewParts, comment)
			}
		}
	}
	VarSetValueStr(ctx, "$t制卡评价", strings.Join(reviewParts, "\n"))
	return DiceFormatTmpl(ctx, "COC:制卡_单卡")
}

func generateCocCards(ctx *MsgContext, count int64, spec cocCardSpec) []string {
	cards := make([]string, 0, max(0, count))
	for range count {
		card, err := rollCocCard(ctx, spec)
		if err != nil {
			ctx.Dice.Logger.Errorf("COC character generation failed: %v", err)
			break
		}
		cards = append(cards, formatCocCard(ctx, card, spec))
	}
	return cards
}

func newCocCardCommand(name string, shortHelp string, help string, spec cocCardSpec, outputTemplate string, titleTemplate string) *CmdItemInfo {
	return &CmdItemInfo{
		Name:      name,
		ShortHelp: shortHelp,
		Help:      help,
		Solve: func(ctx *MsgContext, msg *Message, cmdArgs *CmdArgs) CmdExecuteResult {
			arg := cmdArgs.GetArgN(1)
			count, err := strconv.ParseInt(arg, 10, 64)
			if err != nil {
				if arg == "" {
					count = 1
				} else {
					return CmdExecuteResult{Matched: true, Solved: true, ShowHelp: true}
				}
			}
			if count > ctx.Dice.Config.MaxCocCardGen {
				count = ctx.Dice.Config.MaxCocCardGen
			}

			cards := generateCocCards(ctx, count, spec)
			separator := DiceFormatTmpl(ctx, "COC:制卡_分隔符")
			VarSetValueStr(ctx, "$t制卡结果文本", strings.Join(cards, separator))
			text := DiceFormatTmpl(ctx, outputTemplate)
			if ctx.Dice.Config.CocCardMergeForward {
				title := DiceFormatTmpl(ctx, titleTemplate)
				if TryReplyToSenderMergedForward(ctx, msg, title, cards) {
					return CmdExecuteResult{Matched: true, Solved: true}
				}
			}
			ReplyToSender(ctx, msg, text)
			return CmdExecuteResult{Matched: true, Solved: true}
		},
	}
}
