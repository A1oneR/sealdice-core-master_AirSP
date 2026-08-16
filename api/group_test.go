package api //nolint:testpackage

import (
	"encoding/json"
	"net/http"
	"testing"

	"go.uber.org/zap"

	"sealdice-core/dice"
)

func TestGroupDisableAll(t *testing.T) {
	testDice := &dice.Dice{
		Logger:      zap.NewNop().Sugar(),
		DirtyGroups: new(dice.SyncMap[string, int64]),
	}
	testDice.ImSession = &dice.IMSession{
		Parent:       testDice,
		ServiceAtNew: new(dice.SyncMap[string, *dice.GroupInfo]),
		EndPoints: []*dice.EndPointInfo{
			{EndPointInfoBase: dice.EndPointInfoBase{UserID: "QQ:10001"}},
			{EndPointInfoBase: dice.EndPointInfoBase{UserID: "QQ:10002"}},
		},
	}

	active := &dice.GroupInfo{
		Active:          true,
		GroupID:         "QQ-Group:100",
		DiceIDActiveMap: new(dice.SyncMap[string, bool]),
		DiceIDExistsMap: new(dice.SyncMap[string, bool]),
	}
	active.DiceIDActiveMap.Store("QQ:10001", true)
	active.DiceIDActiveMap.Store("QQ:10002", true)
	active.DiceIDExistsMap.Store("QQ:10001", true)
	active.DiceIDExistsMap.Store("QQ:10002", true)

	inactive := &dice.GroupInfo{
		GroupID:         "QQ-Group:200",
		DiceIDActiveMap: new(dice.SyncMap[string, bool]),
		DiceIDExistsMap: new(dice.SyncMap[string, bool]),
	}
	inactive.DiceIDExistsMap.Store("QQ:10001", true)

	testDice.ImSession.ServiceAtNew.Store(active.GroupID, active)
	testDice.ImSession.ServiceAtNew.Store(inactive.GroupID, inactive)

	const token = "group-test-token"
	manager := &dice.DiceManager{Dice: []*dice.Dice{testDice}}
	manager.AccessTokens.Store(token, true)
	testDice.Parent = manager
	myDice = testDice
	dm = manager
	t.Cleanup(func() {
		myDice = nil
		dm = nil
	})

	rec := performReplyAPIRequest(t, http.MethodPost, "/sd-api/group/disable_all", `{}`, token, groupDisableAll)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var result struct {
		Total            int `json:"total"`
		Changed          int `json:"changed"`
		AlreadyDisabled  int `json:"alreadyDisabled"`
		DisabledAccounts int `json:"disabledAccounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("response JSON error: %v", err)
	}
	if result.Total != 2 || result.Changed != 1 || result.AlreadyDisabled != 1 || result.DisabledAccounts != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if active.Active || active.DiceIDActiveMap.Len() != 0 {
		t.Fatalf("active group was not fully disabled: active=%v accounts=%d", active.Active, active.DiceIDActiveMap.Len())
	}
	if !testDice.DirtyGroups.Exists(active.GroupID) {
		t.Fatal("disabled group was not marked dirty")
	}
}

func TestGroupDisableAllRequiresAuth(t *testing.T) {
	testDice := &dice.Dice{
		Logger: zap.NewNop().Sugar(),
		ImSession: &dice.IMSession{
			ServiceAtNew: new(dice.SyncMap[string, *dice.GroupInfo]),
		},
	}
	manager := &dice.DiceManager{Dice: []*dice.Dice{testDice}}
	testDice.Parent = manager
	myDice = testDice
	dm = manager
	t.Cleanup(func() {
		myDice = nil
		dm = nil
	})

	rec := performReplyAPIRequest(t, http.MethodPost, "/sd-api/group/disable_all", `{}`, "invalid-token", groupDisableAll)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
