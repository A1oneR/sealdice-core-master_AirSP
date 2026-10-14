package dice //nolint:testpackage

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/tidwall/buntdb"
	"go.uber.org/zap"
)

func storageTestDice(t *testing.T) *Dice {
	t.Helper()
	return &Dice{Logger: zap.NewNop().Sugar(), BaseConfig: BaseConfig{DataDir: t.TempDir()},
		JsExtRegistry: new(SyncMap[string, *ExtInfo]), ExtRegistry: new(SyncMap[string, *ExtInfo]), ExtLoopManager: NewJsLoopManager()}
}

func storageTestExt(t *testing.T, d *Dice) *ExtInfo {
	t.Helper()
	e := &ExtInfo{Name: "骰娘好感度（小游戏合集）", dice: d}
	d.JsExtRegistry.Store(e.Name, e)
	if err := e.StorageSet("profile", "saved-assets"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.StorageClose(); time.Sleep(1100 * time.Millisecond) })
	return e
}

func TestJsClearStopsCallbacksBeforeClosingStorage(t *testing.T) {
	d := storageTestDice(t)
	loop := eventloop.NewEventLoop(eventloop.EnableConsole(false))
	d.ExtLoopManager.SetLoop(loop)
	loop.Start()
	e := storageTestExt(t, d)
	db := e.Storage
	entered, release, callbackDone, clearDone := make(chan struct{}), make(chan struct{}), make(chan error, 1), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }); loop.Terminate() })
	loop.RunOnLoop(func(_ *goja.Runtime) { close(entered); <-release; callbackDone <- e.StorageSet("late", "callback") })
	<-entered
	go func() { d.jsClear(); close(clearDone) }()
	// In the old implementation, the DB closes while this callback is running.
	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		if errors.Is(db.View(func(_ *buntdb.Tx) error { return nil }), buntdb.ErrDatabaseClosed) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	once.Do(func() { close(release) })
	select {
	case <-callbackDone:
	case <-time.After(3 * time.Second):
		t.Fatal("callback stalled")
	}
	select {
	case <-clearDone:
	case <-time.After(3 * time.Second):
		t.Fatal("JS cleanup stalled")
	}
	if e.Storage != nil {
		t.Fatal("old callback reopened storage and leaked its database handle")
	}
	if !errors.Is(db.View(func(_ *buntdb.Tx) error { return nil }), buntdb.ErrDatabaseClosed) {
		t.Fatal("database was not closed")
	}
	if err := e.StorageSet("stale", "must-not-open"); err == nil {
		t.Fatal("retired extension reopened storage")
	}
}

func TestJsLoopTerminationDoesNotHoldManagerLock(t *testing.T) {
	m := NewJsLoopManager()
	loop := eventloop.NewEventLoop(eventloop.EnableConsole(false))
	m.SetLoop(loop)
	loop.Start()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	loop.RunOnLoop(func(_ *goja.Runtime) { close(entered); <-release })
	<-entered
	go func() { m.SetLoop(nil); close(done) }()
	time.Sleep(30 * time.Millisecond)
	queried := make(chan struct{})
	go func() { m.GetWebLoop(); close(queried) }()
	select {
	case <-queried:
	case <-time.After(300 * time.Millisecond):
		once.Do(func() { close(release) })
		<-done
		t.Fatal("termination held manager lock; callback querying its loop would deadlock")
	}
	once.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("termination stalled")
	}
}

func TestExtensionStorageRejectsDuplicateOwner(t *testing.T) {
	d := storageTestDice(t)
	first := storageTestExt(t, d)
	second := &ExtInfo{Name: first.Name, dice: d}
	t.Cleanup(func() { _ = second.StorageClose() })
	if err := second.StorageSet("profile", "overwrite"); err == nil {
		t.Fatal("second owner opened the same storage file")
	}
	got, err := first.StorageGet("profile")
	if err != nil || got != "saved-assets" {
		t.Fatalf("first owner changed: %q, %v", got, err)
	}
	if err := first.StorageClose(); err != nil {
		t.Fatal(err)
	}
	if err := second.StorageSet("next", "owner"); err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
}

func TestExtensionStorageSkipsIdenticalWrites(t *testing.T) {
	d := storageTestDice(t)
	e := storageTestExt(t, d)
	filename := filepath.Join(d.GetExtDataDir(e.Name), "storage.db")
	before, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 100; n++ {
		if err := e.StorageSet("profile", "saved-assets"); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != before.Size() {
		t.Fatalf("unchanged state grew the AOF: %d -> %d", before.Size(), after.Size())
	}
	if err := e.StorageSet("profile", "changed-assets"); err != nil {
		t.Fatal(err)
	}
	if err := e.StorageClose(); err != nil {
		t.Fatal(err)
	}
	got, err := e.StorageGet("profile")
	if err != nil || got != "changed-assets" {
		t.Fatalf("changed value not durable: %q, %v", got, err)
	}
}

func TestExtensionStorageWindowsDoesNotAutoShrink(t *testing.T) {
	d := storageTestDice(t)
	e := storageTestExt(t, d)
	var config buntdb.Config
	if err := e.Storage.ReadConfig(&config); err != nil {
		t.Fatal(err)
	}
	if config.AutoShrinkDisabled != (runtime.GOOS == "windows") {
		t.Fatalf("unexpected auto shrink policy on %s: %+v", runtime.GOOS, config)
	}
	if config.SyncPolicy != buntdb.EverySecond {
		t.Fatal("durability policy changed")
	}
}

func TestExtensionStorageConcurrentCloseAndRead(t *testing.T) {
	d := storageTestDice(t)
	e := storageTestExt(t, d)
	var workers sync.WaitGroup
	for n := 0; n < 4; n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 100; j++ {
				if _, err := e.StorageGet("profile"); err != nil {
					t.Error(err)
					return
				}
				if err := e.StorageSet("profile", "saved-assets"); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		for j := 0; j < 100; j++ {
			if err := e.StorageClose(); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	workers.Wait()
}

func TestExtensionStorageLockAcrossProcesses(t *testing.T) {
	if directory := os.Getenv("SEAL_STORAGE_LOCK_TEST_DIR"); directory != "" {
		d := &Dice{Logger: zap.NewNop().Sugar(), BaseConfig: BaseConfig{DataDir: directory}}
		e := &ExtInfo{Name: "骰娘好感度（小游戏合集）", dice: d}
		if err := e.StorageInit(); err == nil {
			_ = e.StorageClose()
			t.Fatal("child process obtained occupied storage")
		}
		return
	}
	d := storageTestDice(t)
	storageTestExt(t, d)
	cmd := exec.Command(os.Args[0], "-test.run=^TestExtensionStorageLockAcrossProcesses$", "-test.count=1")
	cmd.Env = append(os.Environ(), "SEAL_STORAGE_LOCK_TEST_DIR="+d.BaseConfig.DataDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child lock check: %v\n%s", err, output)
	}
}

func TestExtensionStorageRemovedCannotReopen(t *testing.T) {
	d := storageTestDice(t)
	e := storageTestExt(t, d)
	e.IsJsExt = true
	d.ExtRemove(e)
	if e.Storage != nil {
		t.Fatal("removed extension retained storage")
	}
	if err := e.StorageSet("profile", "wrong-assets"); err == nil {
		t.Fatal("removed extension reopened storage")
	}
	next := &ExtInfo{Name: e.Name, dice: d}
	t.Cleanup(func() { _ = next.StorageClose() })
	got, err := next.StorageGet("profile")
	if err != nil || got != "saved-assets" {
		t.Fatalf("removed storage lost assets: %q, %v", got, err)
	}
}

func TestExtensionStorageWindowsHeldFileKeepsWriting(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows sharing violation regression")
	}
	d := storageTestDice(t)
	e := storageTestExt(t, d)
	filename := filepath.Join(d.GetExtDataDir(e.Name), "storage.db")
	reader, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var config buntdb.Config
	if err := e.Storage.ReadConfig(&config); err != nil {
		t.Fatal(err)
	}
	config.AutoShrinkMinSize = 0
	config.AutoShrinkPercentage = 0
	if err := e.Storage.SetConfig(config); err != nil {
		t.Fatal(err)
	}
	for j := 0; j < 30; j++ {
		if err := e.StorageSet("profile", string(make([]byte, 5000+j))); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(1200 * time.Millisecond)
	if err := e.StorageSet("profile", "still-durable"); err != nil {
		t.Fatal(err)
	}
	reader.Close()
	if err := e.StorageClose(); err != nil {
		t.Fatal(err)
	}
	got, err := e.StorageGet("profile")
	if err != nil || got != "still-durable" {
		t.Fatalf("held-file recovery: %q, %v", got, err)
	}
}

func TestExtensionStorageRepeatedRealJSReload(t *testing.T) {
	d := storageTestDice(t)
	d.ImSession = &IMSession{ServiceAtNew: new(SyncMap[string, *GroupInfo]), EndPoints: []*EndPointInfo{}}
	d.DirtyGroups = new(SyncMap[string, int64])
	d.AttrsManager = &AttrsManager{}
	t.Cleanup(func() { d.jsClear(); time.Sleep(1100 * time.Millisecond) })
	for cycle := 0; cycle < 12; cycle++ {
		d.JsInit()
		loop := d.ExtLoopManager.GetWebLoop()
		done := make(chan error, 1)
		loop.RunOnLoop(func(vm *goja.Runtime) {
			_, err := vm.RunString(`
		var plugin=seal.ext.find('骰娘好感度（小游戏合集）');
		if(!plugin){plugin=seal.ext.new('骰娘好感度（小游戏合集）','test','1.48.8');seal.ext.register(plugin);}
		if(plugin.storageGet('profile') && plugin.storageGet('profile')!=='saved-assets'){throw Error('assets changed');}
		plugin.storageSet('profile','saved-assets');
		var abandoned=plugin;
		plugin=seal.ext.new('骰娘好感度（小游戏合集）','test','1.48.8');seal.ext.register(plugin);
		if(plugin.storageGet('profile')!=='saved-assets'){throw Error('replacement lost assets');}
		var rejected=false;try{abandoned.storageSet('profile','wrong-assets');}catch(_){rejected=true;}
		if(!rejected){throw Error('replaced extension can reopen storage');}`)
			done <- err
		})
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("cycle %d: %v", cycle, err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("real JS setup stalled")
		}
		old, _ := d.JsExtRegistry.Load("骰娘好感度（小游戏合集）")
		d.jsClear()
		if old.Storage != nil {
			t.Fatal("real JS reload leaked database")
		}
		if err := old.StorageSet("profile", "wrong-assets"); err == nil {
			t.Fatal("real JS stale extension reopened")
		}
		filename := filepath.Join(d.GetExtDataDir(old.Name), "storage.db")
		// Windows will refuse this rename if an old file handle was left open.
		if err := os.Rename(filename, filename+".probe"); err != nil {
			t.Fatalf("reload %d retained a handle: %v", cycle, err)
		}
		if err := os.Rename(filename+".probe", filename); err != nil {
			t.Fatal(err)
		}
	}
}
