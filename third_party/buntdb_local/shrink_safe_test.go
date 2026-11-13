package buntdb

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func safeTestDB(t *testing.T) (*DB, string) {
	t.Helper()
	name := filepath.Join(t.TempDir(), "存档.db")
	db, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	c := db.config
	c.AutoShrinkDisabled = true
	if err = db.SetConfig(c); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, name
}

func safeSet(db *DB, key, value string) error {
	return db.Update(func(tx *Tx) error { _, _, err := tx.Set(key, value, nil); return err })
}

func safeGet(t *testing.T, db *DB, key string) string {
	t.Helper()
	var value string
	if err := db.View(func(tx *Tx) error { var e error; value, e = tx.Get(key); return e }); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestSafeShrinkRepeatedSwapAndRestart(t *testing.T) {
	db, name := safeTestDB(t)
	for round := 0; round < 4; round++ {
		for i := 0; i < 100; i++ {
			if err := safeSet(db, "profile", fmt.Sprint(round, i)+strings.Repeat("x", 4096)); err != nil {
				t.Fatal(err)
			}
		}
		before, _ := os.Stat(name)
		if err := db.Shrink(); err != nil {
			t.Fatal(err)
		}
		after, _ := os.Stat(name)
		if after.Size() >= before.Size()/4 {
			t.Fatalf("not compacted: %d -> %d", before.Size(), after.Size())
		}
		if db.path != name {
			t.Fatal("canonical path changed to temp name")
		}
	}
	if err := db.Update(func(tx *Tx) error {
		_, _, err := tx.Set("ttl", "receipt", &SetOptions{Expires: true, TTL: time.Hour})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Shrink(); err != nil {
		t.Fatal(err)
	}
	expected := safeGet(t, db, "profile")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if safeGet(t, reopened, "profile") != expected || safeGet(t, reopened, "ttl") != "receipt" {
		t.Fatal("restart changed values")
	}
	if err := reopened.View(func(tx *Tx) error {
		ttl, e := tx.TTL("ttl")
		if e != nil {
			return e
		}
		if ttl < 58*time.Minute || ttl > time.Hour {
			t.Fatal("TTL changed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(name + ".compact-*")
	if len(left) != 0 {
		t.Fatal("successful swaps leaked temp files")
	}
}

func TestSafeShrinkConcurrentTransactionsKeepProfileAndReceipts(t *testing.T) {
	db, name := safeTestDB(t)
	var wg sync.WaitGroup
	errors := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 1000; i++ {
			err := db.Update(func(tx *Tx) error {
				if _, _, e := tx.Set("profile", fmt.Sprint(i), nil); e != nil {
					return e
				}
				_, _, e := tx.Set(fmt.Sprintf("receipt:%d", i), "committed", nil)
				return e
			})
			if err != nil {
				errors <- err
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	for i := 0; i < 3; i++ {
		if err := db.Shrink(); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	select {
	case err := <-errors:
		t.Fatal(err)
	default:
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if safeGet(t, reopened, "profile") != "1000" {
		t.Fatal("profile rollback")
	}
	for i := 1; i <= 1000; i++ {
		if safeGet(t, reopened, fmt.Sprintf("receipt:%d", i)) != "committed" {
			t.Fatal("receipt lost")
		}
	}
}

func TestSafeShrinkPreparationFailureKeepsOriginalWriter(t *testing.T) {
	db, name := safeTestDB(t)
	if err := safeSet(db, "profile", "before"); err != nil {
		t.Fatal(err)
	}
	original := db.file
	db.path = filepath.Join(t.TempDir(), "missing-folder", "storage.db")
	if err := db.Shrink(); err == nil {
		t.Fatal("expected temp create failure")
	}
	db.path = name
	if db.file != original {
		t.Fatal("failure closed or replaced original writer")
	}
	if err := safeSet(db, "profile", "after"); err != nil {
		t.Fatal(err)
	}
	if safeGet(t, db, "profile") != "after" {
		t.Fatal("writer unusable after failure")
	}
}

func TestSafeShrinkCloseDuringSnapshot(t *testing.T) {
	db, _ := safeTestDB(t)
	if err := safeSet(db, "profile", "before"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- db.Shrink() }()
	deadline := time.Now().Add(time.Second)
	for {
		db.mu.RLock()
		running := db.shrinking
		db.mu.RUnlock()
		if running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("shrink did not start")
		}
		time.Sleep(time.Millisecond)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != ErrDatabaseClosed {
			t.Fatalf("unexpected close result: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shrink deadlocked")
	}
}

func TestSafeShrinkProcessExitBoundaries(t *testing.T) {
	if filename := os.Getenv("BUNTDB_SAFE_EXIT_FIXTURE"); filename != "" {
		db, err := Open(filename)
		if err != nil {
			panic(err)
		}
		c := db.config
		c.SyncPolicy = Always
		c.AutoShrinkDisabled = true
		_ = db.SetConfig(c)
		for i := 0; i < 100; i++ {
			if err := safeSet(db, "profile", "latest-assets"); err != nil {
				panic(err)
			}
		}
		if err := safeSet(db, "receipt", "committed"); err != nil {
			panic(err)
		}
		if os.Getenv("BUNTDB_SAFE_EXIT_STAGE") == "after" {
			if err := db.Shrink(); err != nil {
				panic(err)
			}
		}
		os.Exit(0) // Deliberately skip Close, as in abrupt process termination.
	}
	for _, stage := range []string{"before", "after"} {
		t.Run(stage, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "storage.db")
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(binary, "-test.run=^TestSafeShrinkProcessExitBoundaries$")
			command.Env = append(os.Environ(), "BUNTDB_SAFE_EXIT_FIXTURE="+filename, "BUNTDB_SAFE_EXIT_STAGE="+stage)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("child: %v %s", err, output)
			}
			db, err := Open(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if safeGet(t, db, "profile") != "latest-assets" || safeGet(t, db, "receipt") != "committed" {
				t.Fatal("abrupt exit lost committed state")
			}
		})
	}
}
