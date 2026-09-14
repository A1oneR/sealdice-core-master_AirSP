package dice

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reRuleWithOpt = regexp.MustCompile(`\\rule\[[^\]]*\]\{[^{}]*\}\{[^{}]*\}`)
	reRule        = regexp.MustCompile(`\\rule\{[^{}]*\}\{[^{}]*\}`)
	reFcolorbox   = regexp.MustCompile(`\\fcolorbox\{[^{}]*\}\{[^{}]*\}\{([^{}]*)\}`)
	reColorbox    = regexp.MustCompile(`\\colorbox\{[^{}]*\}\{([^{}]*)\}`)
	reTextcolor   = regexp.MustCompile(`\\textcolor\{[^{}]*\}\{([^{}]*)\}`)
	reBoxed       = regexp.MustCompile(`\\boxed\{([^{}]*)\}`)
	reText        = regexp.MustCompile(`\\(?:text|textbf|textit|textsf|texttt|mathrm|mathbf|boldsymbol|mathring|reflectbox|substack)\{([^{}]*)\}`)
	rePhantom     = regexp.MustCompile(`\\phantom\{[^{}]*\}`)
	reTag         = regexp.MustCompile(`\\tag\*?\{([^{}]*)\}`)
	reCancelto    = regexp.MustCompile(`\\cancelto\{([^{}]*)\}\{([^{}]*)\}`)
	reOverset     = regexp.MustCompile(`\\(?:overset|underset)\{([^{}]*)\}\{([^{}]*)\}`)

	reXrightarrowOpt = regexp.MustCompile(`\\(?:xrightarrow|xtwoheadrightarrow|xhookrightarrow)\[([^\]]*)\]\{([^{}]*)\}`)
	reXrightarrow    = regexp.MustCompile(`\\(?:xrightarrow|xtwoheadrightarrow|xhookrightarrow)\{([^{}]*)\}`)
	reXleftarrowOpt  = regexp.MustCompile(`\\xleftarrow\[([^\]]*)\]\{([^{}]*)\}`)
	reXleftarrow     = regexp.MustCompile(`\\xleftarrow\{([^{}]*)\}`)

	reSizes     = regexp.MustCompile(`\\(?:Huge|huge|LARGE|Large|large|normalsize|small|footnotesize|scriptsize|tiny)`)
	reSpaces    = regexp.MustCompile(`\\(?:quad|qquad|,|;|!|\s)`)
	reDelim     = regexp.MustCompile(`\$\$|\$|\\\[|\\\]|\\\(|\\\)`)
	reEnv       = regexp.MustCompile(`\\(?:begin|end)\{[^{}]*\}`)
	reLeftRight = regexp.MustCompile(`\\(?:left|right)([\(\)\[\]\{\}\|\.\\])`)
	reLineBreak = regexp.MustCompile(`\\\\`)
	reHline     = regexp.MustCompile(`\\hline`)
)

// CleanLogMessage 清洗消息中的 KaTeX / LaTeX 排版代码与装饰性富文本，保留纯净的自然语言文案与数值
func CleanLogMessage(s string) string {
	if s == "" || !strings.ContainsAny(s, `$\`) {
		return s
	}

	// 1. 符号与环境初步转化
	s = reHline.ReplaceAllString(s, "")
	s = reLineBreak.ReplaceAllString(s, "\n")
	s = reLeftRight.ReplaceAllString(s, "$1")
	s = reEnv.ReplaceAllString(s, "")

	// 常用数学/KaTeX 符号映射
	symMap := map[string]string{
		`\searrow`:            "↘",
		`\nearrow`:            "↗",
		`\bigstar`:            "★",
		`\star`:               "★",
		`\ast`:                "*",
		`\odot`:               "⊙",
		`\circlearrowright`:   "↻",
		`\circlearrowleft`:    "↺",
		`\bullet`:             "•",
		`\circ`:               "○",
		`\square`:             "□",
		`\boxtimes`:           "☒",
		`\oplus`:              "(+)",
		`\ominus`:             "(-)",
		`\ulcorner`:           "┌",
		`\urcorner`:           "┐",
		`\llcorner`:           "└",
		`\lrcorner`:           "┘",
		`\xleftrightharpoons`: "⇋",
		`\&`:                  "&",
		`\%`:                  "%",
		`\_`:                  "_",
		`\#`:                  "#",
	}
	for k, v := range symMap {
		s = strings.ReplaceAll(s, k, v)
	}

	// 2. 循环展开内层宏，最多迭代 20 次处理嵌套结构
	for i := 0; i < 20; i++ {
		changed := false

		// 移除视觉 rule 与占位符
		if reRuleWithOpt.MatchString(s) {
			s = reRuleWithOpt.ReplaceAllString(s, "")
			changed = true
		}
		if reRule.MatchString(s) {
			s = reRule.ReplaceAllString(s, "")
			changed = true
		}
		if rePhantom.MatchString(s) {
			s = rePhantom.ReplaceAllString(s, "")
			changed = true
		}

		// 转化动态箭头
		if reXrightarrowOpt.MatchString(s) {
			s = reXrightarrowOpt.ReplaceAllStringFunc(s, func(m string) string {
				match := reXrightarrowOpt.FindStringSubmatch(m)
				sub := strings.TrimSpace(match[1])
				sup := strings.TrimSpace(match[2])
				if sub != "" && sup != "" {
					return fmt.Sprintf(" ➯ [%s] (%s) ", sup, sub)
				} else if sub != "" {
					return fmt.Sprintf(" ➯ (%s) ", sub)
				} else if sup != "" {
					return fmt.Sprintf(" ➯ (%s) ", sup)
				}
				return " ➯ "
			})
			changed = true
		}
		if reXrightarrow.MatchString(s) {
			s = reXrightarrow.ReplaceAllStringFunc(s, func(m string) string {
				match := reXrightarrow.FindStringSubmatch(m)
				sup := strings.TrimSpace(match[1])
				if sup != "" {
					return fmt.Sprintf(" ➯ (%s) ", sup)
				}
				return " ➯ "
			})
			changed = true
		}
		if reXleftarrowOpt.MatchString(s) {
			s = reXleftarrowOpt.ReplaceAllStringFunc(s, func(m string) string {
				match := reXleftarrowOpt.FindStringSubmatch(m)
				sub := strings.TrimSpace(match[1])
				sup := strings.TrimSpace(match[2])
				if sub != "" && sup != "" {
					return fmt.Sprintf(" ⮤ [%s] (%s) ", sup, sub)
				} else if sub != "" {
					return fmt.Sprintf(" ⮤ (%s) ", sub)
				} else if sup != "" {
					return fmt.Sprintf(" ⮤ (%s) ", sup)
				}
				return " ⮤ "
			})
			changed = true
		}
		if reXleftarrow.MatchString(s) {
			s = reXleftarrow.ReplaceAllString(s, " ⮤ ")
			changed = true
		}

		// 转化改动宏
		if reCancelto.MatchString(s) {
			s = reCancelto.ReplaceAllString(s, "$2 ➯ $1")
			changed = true
		}

		// 转化上下标注记
		if reOverset.MatchString(s) {
			s = reOverset.ReplaceAllString(s, "$2")
			changed = true
		}

		// 剥除框与颜色
		if reFcolorbox.MatchString(s) {
			s = reFcolorbox.ReplaceAllString(s, "$1")
			changed = true
		}
		if reColorbox.MatchString(s) {
			s = reColorbox.ReplaceAllString(s, "$1")
			changed = true
		}
		if reTextcolor.MatchString(s) {
			s = reTextcolor.ReplaceAllString(s, "$1")
			changed = true
		}
		if reBoxed.MatchString(s) {
			s = reBoxed.ReplaceAllString(s, "$1")
			changed = true
		}

		// 剥除纯文本与字体
		if reText.MatchString(s) {
			s = reText.ReplaceAllString(s, "$1")
			changed = true
		}
		if reTag.MatchString(s) {
			s = reTag.ReplaceAllString(s, " [$1]")
			changed = true
		}

		if !changed {
			break
		}
	}

	// 3. 移除尺寸与空白调节宏
	s = reSizes.ReplaceAllString(s, "")
	s = reSpaces.ReplaceAllString(s, " ")

	// 4. 剥离定界符与孤立斜杠
	s = reDelim.ReplaceAllString(s, "")
	s = regexp.MustCompile(`\\([A-Za-z]+)`).ReplaceAllString(s, "") // 移除未匹配的未知宏
	s = regexp.MustCompile(`\\+`).ReplaceAllString(s, "")

	// 5. 移除残留孤立花括号（在 KaTeX 宏剥除后）
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")

	// 6. 整理行与空白
	lines := strings.Split(s, "\n")
	var cleanedLines []string
	lastEmpty := false

	for _, l := range lines {
		// 消除连续多余空格
		l = regexp.MustCompile(`[ \t]+`).ReplaceAllString(l, " ")
		l = strings.TrimSpace(l)
		if l == "" {
			if !lastEmpty && len(cleanedLines) > 0 {
				cleanedLines = append(cleanedLines, "")
				lastEmpty = true
			}
		} else {
			cleanedLines = append(cleanedLines, l)
			lastEmpty = false
		}
	}

	return strings.Join(cleanedLines, "\n")
}
