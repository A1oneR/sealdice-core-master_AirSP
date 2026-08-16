package dice

import (
	"strings"
	"time"
)

func (ctx *MsgContext) characterAPIReady() bool {
	return ctx != nil && ctx.Dice != nil && ctx.Dice.AttrsManager != nil && ctx.Player != nil
}

func normalizeCharacterName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "\n", ""), "\r", ""))
	if len(name) > 90 {
		name = name[:90]
	}
	return name
}

// ChExists reports whether the current user owns a native character with name.
func (ctx *MsgContext) ChExists(name string) bool {
	if !ctx.characterAPIReady() {
		return false
	}
	name = normalizeCharacterName(name)
	return name != "" && ctx.Dice.AttrsManager.CharCheckExists(ctx.Player.UserID, name)
}

// ChNew creates a native character without changing the current binding.
func (ctx *MsgContext) ChNew(name string) bool {
	if !ctx.characterAPIReady() {
		return false
	}
	name = normalizeCharacterName(name)
	if name == "" || ctx.ChExists(name) {
		return false
	}
	sheetType := ""
	if ctx.Group != nil {
		sheetType = ctx.Group.System
	}
	_, err := ctx.Dice.AttrsManager.CharNew(ctx.Player.UserID, name, sheetType)
	return err == nil
}

// ChBindCur binds an owned native character to the current group.
func (ctx *MsgContext) ChBindCur(name string) bool {
	if !ctx.characterAPIReady() || ctx.Group == nil {
		return false
	}
	name = normalizeCharacterName(name)
	if name == "" {
		return false
	}
	charID, err := ctx.Dice.AttrsManager.CharIdGetByName(ctx.Player.UserID, name)
	if err != nil || charID == "" {
		return false
	}
	if err = ctx.Dice.AttrsManager.CharBind(charID, ctx.Group.GroupID, ctx.Player.UserID); err != nil {
		return false
	}
	ctx.Player.Name = name
	ctx.Player.UpdatedAtTime = time.Now().Unix()
	ctx.Group.MarkDirty(ctx.Dice)
	if ctx.Player.AutoSetNameTemplate != "" {
		_, _ = SetPlayerGroupCardByTemplate(ctx, ctx.Player.AutoSetNameTemplate)
	}
	return true
}

// ChBindCurGet returns the native character currently bound in this group.
func (ctx *MsgContext) ChBindCurGet() string {
	if !ctx.characterAPIReady() || ctx.Group == nil {
		return ""
	}
	charID, err := ctx.Dice.AttrsManager.CharGetBindingId(ctx.Group.GroupID, ctx.Player.UserID)
	if err != nil || charID == "" {
		return ""
	}
	attrs, err := ctx.Dice.AttrsManager.LoadById(charID)
	if err != nil || attrs == nil {
		return ""
	}
	return attrs.Name
}
