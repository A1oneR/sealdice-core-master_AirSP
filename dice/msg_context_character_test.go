package dice

import (
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"

	"sealdice-core/model"
)

func TestMsgContextNativeCharacterAPIAppearsInPCList(t *testing.T) {
	d, ep, adapter, cleanup := newExecuteNewTestDice(t)
	defer cleanup()

	mockDB := d.DBOperator.(*mockDatabaseOperator)
	if err := mockDB.db.AutoMigrate(&model.AttributesItemModel{}); err != nil {
		t.Fatalf("migrate attrs table: %v", err)
	}

	ctx := &MsgContext{
		Dice:     d,
		Session:  d.ImSession,
		EndPoint: ep,
		Group: &GroupInfo{
			GroupID: "QQ-Group:character-api",
			System:  "coc7",
		},
		Player: &GroupPlayerInfo{
			UserID: "QQ:character-api",
			Name:   "Tester",
		},
	}

	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("jsbind", true))
	if err := vm.Set("ctx", ctx); err != nil {
		t.Fatalf("expose MsgContext to JavaScript: %v", err)
	}
	result, err := vm.RunString(`ctx.chNew("Alice") && ctx.chExists("Alice") && ctx.chBindCur("Alice") && ctx.chBindCurGet() === "Alice"`)
	if err != nil {
		t.Fatalf("call native character API from JavaScript: %v", err)
	}
	if !result.ToBoolean() {
		t.Fatal("JavaScript native character API did not create and bind Alice")
	}

	list, err := d.AttrsManager.GetCharacterList(ctx.Player.UserID)
	if err != nil {
		t.Fatalf("GetCharacterList: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Alice" {
		t.Fatalf("native character list = %#v, want Alice", list)
	}

	d.ImSession.ExecuteNew(ep, newGroupMsg(ctx.Group.GroupID, ctx.Player.UserID, ".pc list"))
	reply, ok := adapter.waitForMsg(2 * time.Second)
	if !ok {
		t.Fatal("timeout waiting for native .pc list reply")
	}
	if !strings.Contains(reply, "Alice") {
		t.Fatalf("native .pc list did not contain Alice: %q", reply)
	}
}

func TestNativePCListUsesOriginalQQCharactersFromSecondaryOfficialGroup(t *testing.T) {
	d, ep, adapter, cleanup := newExecuteNewTestDice(t)
	defer cleanup()

	mockDB := d.DBOperator.(*mockDatabaseOperator)
	if err := mockDB.db.AutoMigrate(&model.AttributesItemModel{}); err != nil {
		t.Fatalf("migrate attrs table: %v", err)
	}

	legacyUserID := "QQ:567890123"
	primaryUserID := "OpenQQ:3889204361-primary-native-pc-list"
	secondaryUserID := "OpenQQ:3889204361-secondary-native-pc-list"
	for _, officialID := range []string{primaryUserID, secondaryUserID} {
		RemoveOfficialQQIdentityAlias(officialID)
		if !RegisterOfficialQQIdentityAlias(officialID, legacyUserID) {
			t.Fatalf("failed to register alias %q", officialID)
		}
		defer RemoveOfficialQQIdentityAlias(officialID)
	}

	card, err := d.AttrsManager.CharNew(legacyUserID, "旧QQ调查员", "coc7")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AttrsManager.CharBind(card.Id, "OpenQQ-Group:3889204361-primary-native-list", primaryUserID); err != nil {
		t.Fatal(err)
	}

	d.ImSession.ExecuteNew(ep, newGroupMsg(
		"OpenQQ-Group:3889204361-secondary-native-list",
		secondaryUserID,
		".pc list",
	))
	reply, ok := adapter.waitForMsg(2 * time.Second)
	if !ok {
		t.Fatal("timeout waiting for native .pc list reply")
	}
	if !strings.Contains(reply, "旧QQ调查员") {
		t.Fatalf("secondary Official QQ .pc list omitted original card: %q", reply)
	}
}
