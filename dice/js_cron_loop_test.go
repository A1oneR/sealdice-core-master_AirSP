package dice //nolint:testpackage

import (
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"go.uber.org/zap"
)

func TestJsScriptTaskWaitsForEventLoop(t *testing.T) {
	loop := eventloop.NewEventLoop(eventloop.EnableConsole(false))
	loop.Start()
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); loop.Terminate() })
	loop.RunOnLoop(func(_ *goja.Runtime) { close(entered); <-release })
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("event loop did not start")
	}
	called := make(chan JsScriptTaskCtx, 1)
	task := JsScriptTask{loop: loop, key: "heartbeat", task: func(ctx JsScriptTaskCtx) { called <- ctx }}
	task.run()
	select {
	case <-called:
		t.Fatal("cron entered JS while the event loop was occupied")
	case <-time.After(50 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case ctx := <-called:
		if ctx.Key != "heartbeat" || ctx.Now <= 0 {
			t.Fatalf("invalid task context: %+v", ctx)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("queued cron callback did not execute")
	}
}

func TestJsScriptTaskConcurrentRunsPreservePromises(t *testing.T) {
	loop := eventloop.NewEventLoop(eventloop.EnableConsole(false))
	loop.Start()
	t.Cleanup(loop.Terminate)
	ready := make(chan error, 1)
	var task JsScriptTask
	loop.RunOnLoop(func(vm *goja.Runtime) {
		value, err := vm.RunString(`var cronCount = 0; var promiseCount = 0;
		(function () { cronCount++; Promise.resolve().then(function () { promiseCount++; }); })`)
		if err == nil {
			fn, ok := goja.AssertFunction(value)
			if !ok {
				t.Error("JS callback is not callable")
			} else {
				task = JsScriptTask{loop: loop, task: func(_ JsScriptTaskCtx) {
					if _, err := fn(goja.Undefined()); err != nil {
						panic(err)
					}
				}}
			}
		}
		ready <- err
	})
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("JS setup timed out")
	}
	var workers sync.WaitGroup
	for i := 0; i < 100; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); task.run() }()
	}
	workers.Wait()
	counts := make(chan [2]int64, 1)
	loop.RunOnLoop(func(vm *goja.Runtime) {
		counts <- [2]int64{vm.Get("cronCount").ToInteger(), vm.Get("promiseCount").ToInteger()}
	})
	select {
	case got := <-counts:
		if got != [2]int64{100, 100} {
			t.Fatalf("callbacks/promises = %v, want [100 100]", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("JS callbacks stalled")
	}
}

func TestJsScriptTaskCannotRunAfterLoopTermination(t *testing.T) {
	loop := eventloop.NewEventLoop(eventloop.EnableConsole(false))
	loop.Start()
	loop.Terminate()
	called := false
	task := JsScriptTask{loop: loop, task: func(_ JsScriptTaskCtx) { called = true }}
	task.run()
	if called {
		t.Fatal("old task executed after its JS loop terminated")
	}
}

func TestJsScriptTaskPanicDoesNotStopLaterCallbacks(t *testing.T) {
	loop := eventloop.NewEventLoop(eventloop.EnableConsole(false))
	loop.Start()
	t.Cleanup(loop.Terminate)
	task := JsScriptTask{loop: loop, logger: zap.NewNop().Sugar(), task: func(_ JsScriptTaskCtx) { panic("test cron panic") }}
	task.run()
	done := make(chan struct{})
	loop.RunOnLoop(func(vm *goja.Runtime) {
		_, err := vm.RunString(`Promise.resolve().then(function () {})`)
		if err != nil {
			t.Error(err)
		}
		close(done)
	})
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("event loop stopped after task panic")
	}
}
