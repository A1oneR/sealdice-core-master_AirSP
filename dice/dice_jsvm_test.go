package dice //nolint:testpackage

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dop251/goja"
	"go.uber.org/zap"

	"sealdice-core/model"
)

type countingDiceSource struct {
	values []uint64
	index  int
}

func TestGetOfficialQQBindingLogNamesOnlyReturnsTargetGroup(t *testing.T) {
	dbOperator, err := newMockDatabaseOperator(filepath.Join(t.TempDir(), "binding-log.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbOperator.Close()
	if err := dbOperator.db.AutoMigrate(&model.LogInfo{}, &model.LogOneItem{}); err != nil {
		t.Fatal(err)
	}
	const (
		legacyGroup     = "QQ-Group:123456789"
		fallbackGroup   = "QQ-Group:222222222"
		historicalGroup = "OpenQQ-Group:test-historical-group"
	)
	if !RegisterOfficialQQIdentityAlias(historicalGroup, legacyGroup) {
		t.Fatal("failed to register historical group alias")
	}
	t.Cleanup(func() { RemoveOfficialQQIdentityAlias(historicalGroup) })

	rows := []model.LogInfo{
		{Name: "第一夜", GroupID: legacyGroup, UpdatedAt: 100},
		{Name: "雾港终夜", GroupID: legacyGroup, UpdatedAt: 300},
		{Name: "旧纯数字记录", GroupID: "123456789", UpdatedAt: 250},
		{Name: "历史官方群记录", GroupID: historicalGroup, UpdatedAt: 450},
		{Name: "条目归属修复", GroupID: "OpenQQ-Group:test-unrelated-header", UpdatedAt: 10},
		{Name: "其他群秘密", GroupID: "QQ-Group:987654321", UpdatedAt: 400},
	}
	if err := dbOperator.db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	items := []model.LogOneItem{
		{LogID: rows[4].ID, GroupID: fallbackGroup, Time: 350},
		{LogID: rows[5].ID, GroupID: "QQ-Group:987654321", Time: 500},
	}
	if err := dbOperator.db.Create(&items).Error; err != nil {
		t.Fatal(err)
	}

	result := getOfficialQQBindingLogNames(&Dice{DBOperator: dbOperator}, legacyGroup)
	if result["ok"] != true {
		t.Fatalf("query failed: %#v", result)
	}
	names, ok := result["names"].([]string)
	if !ok {
		t.Fatalf("names type = %T", result["names"])
	}
	if want := []string{"历史官方群记录", "雾港终夜", "旧纯数字记录", "第一夜"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %#v, want %#v", names, want)
	}

	fallback := getOfficialQQBindingLogNames(&Dice{DBOperator: dbOperator}, fallbackGroup)
	if fallback["ok"] != true {
		t.Fatalf("fallback query failed: %#v", fallback)
	}
	fallbackNames, ok := fallback["names"].([]string)
	if !ok {
		t.Fatalf("fallback names type = %T", fallback["names"])
	}
	if want := []string{"条目归属修复"}; !reflect.DeepEqual(fallbackNames, want) {
		t.Fatalf("fallback names = %#v, want %#v", fallbackNames, want)
	}

	invalid := getOfficialQQBindingLogNames(&Dice{DBOperator: dbOperator}, "OpenQQ-Group:bot-group")
	if invalid["ok"] != false || invalid["error"] == nil {
		t.Fatalf("invalid group result = %#v", invalid)
	}
}

func TestGetOfficialQQBindingLogNamesWithoutLogItemsTable(t *testing.T) {
	dbOperator, err := newMockDatabaseOperator(filepath.Join(t.TempDir(), "binding-log-header-only.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbOperator.Close()
	if err := dbOperator.db.AutoMigrate(&model.LogInfo{}); err != nil {
		t.Fatal(err)
	}

	rows := []model.LogInfo{
		{Name: "旧日志", GroupID: "QQ-Group:123456789", UpdatedAt: 100},
		{Name: "旧日志", GroupID: "123456789", UpdatedAt: 200},
		{Name: "其他群日志", GroupID: "QQ-Group:987654321", UpdatedAt: 300},
	}
	if err := dbOperator.db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	result := getOfficialQQBindingLogNames(&Dice{DBOperator: dbOperator}, "QQ-Group:123456789")
	if result["ok"] != true {
		t.Fatalf("query failed: %#v", result)
	}
	names, ok := result["names"].([]string)
	if !ok {
		t.Fatalf("names type = %T", result["names"])
	}
	if want := []string{"旧日志"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %#v, want %#v", names, want)
	}
}

func (s *countingDiceSource) Uint64() uint64 {
	if len(s.values) == 0 {
		return 0
	}
	if s.index >= len(s.values) {
		return s.values[len(s.values)-1]
	}
	v := s.values[s.index]
	s.index++
	return v
}

func TestJsInit_WhenExtLoopManagerNil_DoesNotPanic(t *testing.T) {
	d := &Dice{
		Logger: zap.NewNop().Sugar(),
		BaseConfig: BaseConfig{
			DataDir: t.TempDir(),
		},
		ImSession: &IMSession{
			ServiceAtNew: new(SyncMap[string, *GroupInfo]),
			EndPoints:    []*EndPointInfo{},
		},
		DirtyGroups:  new(SyncMap[string, int64]),
		AttrsManager: &AttrsManager{},
	}

	// 模拟“调用 shutdown 后重启”：JsEnable=false 且 ExtLoopManager 尚未初始化。
	d.Config.JsEnable = false
	d.ExtLoopManager = nil

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("JsInit should not panic when ExtLoopManager is nil, got: %v", r)
		}
		// 清理后台任务，避免测试进程残留 goroutine
		if d.JsScriptCron != nil {
			d.JsScriptCron.Stop()
			d.JsScriptCron = nil
		}
		if d.ExtLoopManager != nil {
			d.ExtLoopManager.SetLoop(nil)
		}
	}()

	d.JsInit()

	if d.ExtLoopManager == nil {
		t.Fatalf("expected ExtLoopManager to be initialized")
	}
	if !d.Config.JsEnable {
		t.Fatalf("expected JsEnable to be true after JsInit")
	}
}

func TestJsInit_BindsGojaRandSourceToGlobalRandSource(t *testing.T) {
	src := &countingDiceSource{values: []uint64{1 << 63}}
	installGlobalDiceSourceState(t, completeSourceMap(
		sourceOverride{mode: DiceRandomModePCG, src: src},
	), nil)
	if _, err := globalRandSource.SetActive(DiceRandomModePCG); err != nil {
		t.Fatalf("activate pcg source: %v", err)
	}
	d := &Dice{
		Logger: zap.NewNop().Sugar(),
		Config: NewConfig(nil),
	}
	if d.Logger == nil {
		t.Fatal("expected logger to be initialized")
	}
	d.Config.DiceRandomMode = string(DiceRandomModePCG)

	vm := goja.New()
	vm.SetRandSource(func() float64 {
		return float64(globalRandSource.Uint64()>>11) / (1 << 53)
	})
	var got float64
	var runErr error
	value, err := vm.RunString("Math.random()")
	if err != nil {
		runErr = err
	} else {
		got = value.ToFloat()
	}
	if runErr != nil {
		t.Fatalf("Math.random() error = %v", runErr)
	}

	want := float64((uint64(1)<<63)>>11) / (1 << 53)
	if got != want {
		t.Fatalf("Math.random() = %.16f, want %.16f", got, want)
	}

	if src.index != 1 {
		t.Fatalf("system dice source consumed %d values, want 1", src.index)
	}
}
