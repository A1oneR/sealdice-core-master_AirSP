package dice

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func CustomReplyConfigRead(dice *Dice, filename string) (*ReplyConfig, error) {
	attrConfigFn := dice.GetExtConfigFilePath("reply", filename)
	return CustomReplyConfigReadFromPath(dice, attrConfigFn, filename)
}

// CustomReplyConfigReadFromPath 从指定路径读取自定义回复配置
func CustomReplyConfigReadFromPath(dice *Dice, filePath string, filename string) (*ReplyConfig, error) {
	rc := &ReplyConfig{Enable: false, Filename: filename}

	if _, err := os.Stat(filePath); err == nil {
		// 如果文件存在，那么读取
		af, err := os.ReadFile(filePath)
		if err == nil {
			err = yaml.Unmarshal(af, rc)
			if err != nil {
				dice.Logger.Error("读取自定义回复配置文件发生异常，请检查格式是否正确")
				return nil, err
			}
			if normalizeReplyVMVersion(rc.VMVersion) == ReplyVMVersionV2 || hasReplyVMVersionV2Marker(af) {
				rc.VMVersion = ReplyVMVersionV2
			} else {
				rc.VMVersion = ""
			}
		}
	}

	if rc.Items == nil {
		rc.Items = []*ReplyItem{}
	}
	if rc.Conditions == nil {
		rc.Conditions = []ReplyConditionBase{}
	}

	return rc, nil
}

func CustomReplyConfigCheckExists(dice *Dice, filename string) bool {
	attrConfigFn := dice.GetExtConfigFilePath("reply", filename)
	if _, err := os.Stat(attrConfigFn); err == nil {
		return true
	}
	return false
}

func hasReplyVMVersionV2Marker(data []byte) bool {
	text := strings.TrimPrefix(string(data), "\ufeff")
	firstLine, _, _ := strings.Cut(text, "\n")
	return strings.TrimSpace(firstLine) == ReplyVMVersionV2Marker
}

func (c *ReplyConfig) vmVersionForExecution(dice *Dice) string {
	if normalizeReplyVMVersion(c.VMVersion) == ReplyVMVersionV2 {
		return ReplyVMVersionV2
	}
	return dice.getTargetVmEngineVersion(VMVersionReply)
}

func CustomReplyConfigNew(dice *Dice, filename string) *ReplyConfig {
	return CustomReplyConfigNewWithVMVersion(dice, filename, ReplyVMVersionV2)
}

func CustomReplyConfigNewWithVMVersion(dice *Dice, filename string, vmVersion string) *ReplyConfig {
	for _, i := range dice.CustomReplyConfig {
		if i.PackageID == "" && strings.EqualFold(i.Filename, filename) {
			return nil
		}
	}

	nowTime := time.Now().Unix()
	rc := &ReplyConfig{
		VMVersion:       normalizeReplyVMVersion(vmVersion),
		Enable:          true,
		Filename:        filename,
		Name:            filename,
		Items:           []*ReplyItem{},
		UpdateTimestamp: nowTime,
		CreateTimestamp: nowTime,
		Author:          []string{"无名海豹"},
		Conditions:      []ReplyConditionBase{},
	}
	dice.CustomReplyConfig = append(dice.CustomReplyConfig, rc)
	rc.Save(dice)
	return rc
}

func CustomReplyConfigDelete(dice *Dice, filename string) bool {
	attrConfigFn := dice.GetExtConfigFilePath("reply", filename)
	if _, err := os.Stat(attrConfigFn); err == nil {
		err := os.Remove(attrConfigFn)
		if err == nil {
			var rcs []*ReplyConfig
			for _, i := range dice.CustomReplyConfig {
				if i.Filename != filename {
					rcs = append(rcs, i)
				}
			}
			dice.CustomReplyConfig = rcs
		}
		return true
	}
	return false
}

func ReplyReload(dice *Dice) {
	var rcs []*ReplyConfig

	// 1. 从全局 extensions/reply 目录加载
	filenames := []string{"reply.yaml"}
	_ = filepath.Walk(dice.GetExtDataDir("reply"), func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			dice.Logger.Warnf("扫描自定义回复目录失败: %v", err)
			return err
		}
		if info == nil {
			return nil
		}
		if info.IsDir() && strings.EqualFold(info.Name(), "assets") {
			return fs.SkipDir
		}
		if info.IsDir() && strings.EqualFold(info.Name(), "images") {
			return fs.SkipDir
		}
		if !info.IsDir() && strings.HasPrefix(info.Name(), ".reply") {
			return nil
		}
		if info.Name() == "info.yaml" {
			return nil
		}

		if !info.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".yaml" || ext == "" {
				if info.Name() != "reply.yaml" {
					filenames = append(filenames, info.Name())
				}
			}
		}
		return nil
	})

	for _, i := range filenames {
		rc, err := CustomReplyConfigRead(dice, i)
		if err == nil {
			dice.Logger.Info("读取自定义回复配置:", i)
			rc.Save(dice)
			rcs = append(rcs, rc)
		} else {
			dice.Logger.Info("读取自定义回复配置 - 失败:", i)
			dice.Logger.Error(err)
		}
	}

	// 2. 从已启用扩展包加载自定义回复配置
	if dice.PackageManager != nil {
		for _, replyFile := range dice.PackageManager.GetEnabledContentFiles("reply") {
			ext := filepath.Ext(replyFile.Path)
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			rc, err := CustomReplyConfigReadFromPath(dice, replyFile.Path, filepath.Base(replyFile.Path))
			if err == nil {
				rc.PackageID = replyFile.PackageID
				rc.CacheBacked = true
				dice.Logger.Infof("读取扩展包自定义回复配置: %s", replyFile.Path)
				rcs = append(rcs, rc)
			} else {
				dice.Logger.Warnf("读取扩展包自定义回复配置失败: %s, %v", replyFile.Path, err)
			}
		}
	}

	dice.CustomReplyConfig = rcs
}

// CustomReplyDo 检查并执行自定义回复匹配
// forceText: 若非空，使用该文本进行匹配（例如传入剥离了指令前导符后的文本）；若为空，则使用 msg.Message
// 返回值: executed (bool) 是否有自定义回复被命中并执行
func CustomReplyDo(ctx *MsgContext, msg *Message, forceText string) bool {
	if ctx == nil || ctx.Dice == nil || msg == nil {
		return false
	}
	rcs := ctx.Dice.CustomReplyConfig
	if !ctx.Dice.Config.CustomReplyConfigEnable {
		return false
	}
	executed := false
	log := ctx.Dice.Logger

	var rawMatchText string
	if forceText != "" {
		rawMatchText = forceText
	} else {
		rawMatchText = msg.Message
	}

	cleanText, _ := AtParse(rawMatchText, "")
	cleanText = strings.TrimSpace(cleanText)
	VarSetValueInt64(ctx, "$t文本长度", int64(len(cleanText)))

	if ctx.Dice.Config.ReplyDebugMode {
		log.Infof("[回复调试]当前文本:“%s” hex: %x 字节形式: %v", cleanText, cleanText, []byte(cleanText))
	}

	// 在判定条件前，先设置一轮变量，以免条件中的变量出问题
	SetTempVars(ctx, msg.Sender.Nickname)

	for _, rc := range rcs {
		if executed {
			break
		}
		if !rc.Enable {
			continue
		}
		vmVersion := rc.vmVersionForExecution(ctx.Dice)
		condIndex := -1
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("异常: %v 堆栈: %v", r, string(debug.Stack()))
				if condIndex != -1 {
					ReplyToSender(ctx, msg, fmt.Sprintf(
						"自定义回复匹配成功(序号%d)，但回复内容触发异常，请联系骰主修改:\n%s",
						condIndex, DiceFormatTmpl(ctx, "核心:骰子执行异常")))
				}
			}
		}()

		{
			commonCondOK := true
			for _, cond := range rc.Conditions {
				if !cond.Check(ctx, msg, nil, cleanText, vmVersion) {
					commonCondOK = false
					break
				}
			}
			if !commonCondOK {
				continue
			}
		}

		checkInCoolDown := func() bool {
			if ctx.Group == nil {
				return false
			}
			lastTime := ctx.Group.LastCustomReplyTime
			now := float64(time.Now().UnixMilli()) / 1000
			interval := rc.Interval
			if interval < 2 {
				interval = 2
			}

			if now-lastTime < interval {
				return true // 未达到冷却，退出
			}
			ctx.Group.LastCustomReplyTime = now
			return false
		}

		for index, i := range rc.Items {
			if !i.Enable {
				continue
			}

			checkTrue := true
			for _, cond := range i.Conditions {
				if !cond.Check(ctx, msg, nil, cleanText, vmVersion) {
					checkTrue = false
					break
				}
			}

			condIndex = index
			if len(i.Conditions) == 0 || !checkTrue {
				continue
			}

			if checkInCoolDown() {
				// 仍在冷却，拒绝回复
				log.Infof("自定义回复[%s]: 条件满足，但正处于冷却", rc.Filename)
				return false
			}
			log.Infof("自定义回复[%s]: 条件满足", rc.Filename)

			SetTempVars(ctx, msg.Sender.Nickname)
			VarSetValueStr(ctx, "$tMsgID", fmt.Sprintf("%v", msg.RawID))
			for _, j := range i.Results {
				j.Execute(ctx, msg, nil, vmVersion)
			}
			executed = true
			break
		}
	}
	return executed
}

// CustomReplyCheckMatch 仅测试指定文本是否匹配任何启用的自定义回复规则（不实际执行回复也不计算冷却）
func CustomReplyCheckMatch(ctx *MsgContext, msg *Message, matchText string) bool {
	if ctx == nil || ctx.Dice == nil {
		return false
	}
	rcs := ctx.Dice.CustomReplyConfig
	if !ctx.Dice.Config.CustomReplyConfigEnable {
		return false
	}
	var rawMatchText string
	if matchText != "" {
		rawMatchText = matchText
	} else if msg != nil {
		rawMatchText = msg.Message
	} else {
		return false
	}

	cleanText, _ := AtParse(rawMatchText, "")
	cleanText = strings.TrimSpace(cleanText)

	for _, rc := range rcs {
		if !rc.Enable {
			continue
		}
		vmVersion := rc.vmVersionForExecution(ctx.Dice)
		commonCondOK := true
		for _, cond := range rc.Conditions {
			if !cond.Check(ctx, msg, nil, cleanText, vmVersion) {
				commonCondOK = false
				break
			}
		}
		if !commonCondOK {
			continue
		}

		for _, item := range rc.Items {
			if !item.Enable || len(item.Conditions) == 0 {
				continue
			}
			checkTrue := true
			for _, cond := range item.Conditions {
				if !cond.Check(ctx, msg, nil, cleanText, vmVersion) {
					checkTrue = false
					break
				}
			}
			if checkTrue {
				return true
			}
		}
	}
	return false
}

// CustomReplyGetActiveRules 汇总获取所有生效的自定义回复规则摘要
func CustomReplyGetActiveRules(dice *Dice) []map[string]interface{} {
	if dice == nil {
		return []map[string]interface{}{}
	}
	var rules []map[string]interface{}
	for _, rc := range dice.CustomReplyConfig {
		if !rc.Enable {
			continue
		}
		for idx, item := range rc.Items {
			if !item.Enable {
				continue
			}
			var condList []map[string]string
			for _, cond := range item.Conditions {
				switch c := cond.(type) {
				case *ReplyConditionTextMatch:
					condList = append(condList, map[string]string{
						"condType":  c.CondType,
						"matchType": c.MatchType,
						"value":     c.Value,
					})
				case *ReplyConditionTextLenLimit:
					condList = append(condList, map[string]string{
						"condType": c.CondType,
						"matchOp":  c.MatchOp,
						"value":    strconv.Itoa(c.Value),
					})
				case *ReplyConditionExprTrue:
					condList = append(condList, map[string]string{
						"condType": c.CondType,
						"value":    c.Value,
					})
				default:
					condList = append(condList, map[string]string{
						"condType": fmt.Sprintf("%T", cond),
						"value":    "",
					})
				}
			}
			rules = append(rules, map[string]interface{}{
				"filename":   rc.Filename,
				"name":       rc.Name,
				"itemIndex":  idx,
				"conditions": condList,
			})
		}
	}
	return rules
}

func RegisterBuiltinExtReply(dice *Dice) {
	ReplyReload(dice)

	theExt := &ExtInfo{
		Name:       "reply", // 扩展的名称，需要用于开启和关闭指令中，写简短点
		Version:    "1.2.0",
		Brief:      "自定义回复模块，支持各种文本匹配和简易脚本",
		Author:     "木落",
		AutoActive: true, // 是否自动开启
		Official:   true,
		OnNotCommandReceived: func(ctx *MsgContext, msg *Message) {
			CustomReplyDo(ctx, msg, "")
		},
		GetDescText: GetExtensionDesc,
		CmdMap:      CmdMapCls{},
	}

	dice.RegisterExtension(theExt)
}
