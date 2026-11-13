//go:build windows

package buntdb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSafeShrinkWindowsSharingViolationKeepsWriter(t *testing.T) {
	db, name := safeTestDB(t)
	for i := 0; i < 100; i++ {
		if err := safeSet(db, "profile", strings.Repeat("p", 4096)); err != nil {
			t.Fatal(err)
		}
	}
	blocker, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	original := db.file
	before, _ := os.Stat(name)
	if err := db.Shrink(); err == nil {
		t.Fatal("non-share-delete handle should block replacement")
	}
	if db.file != original {
		t.Fatal("failed replacement discarded writer")
	}
	after, _ := os.Stat(name)
	if before.Size() != after.Size() {
		t.Fatal("failed replacement changed original AOF")
	}
	if err := safeSet(db, "receipt", "after-failed-shrink"); err != nil {
		t.Fatal(err)
	}
	if safeGet(t, db, "receipt") != "after-failed-shrink" {
		t.Fatal("write after failure lost")
	}
	left, _ := filepath.Glob(name + ".compact-*")
	if len(left) != 0 {
		t.Fatal("failed swap leaked its temp")
	}
	blocker.Close()
	if err := db.Shrink(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if safeGet(t, reopened, "receipt") != "after-failed-shrink" {
		t.Fatal("recovery lost committed receipt")
	}
}

func TestSafeShrinkWindowsAutomaticFailureBackoffAndRetry(t *testing.T) {
	db, name := safeTestDB(t)
	events := make(chan error, 8)
	blocker, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	c := db.config
	c.AutoShrinkDisabled = false
	c.AutoShrinkMinSize = 1
	c.AutoShrinkPercentage = 0
	c.OnShrink = func(before, after int64, err error) { events <- err }
	if err := db.SetConfig(c); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		if err := safeSet(db, "profile", strings.Repeat("p", 4096)); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-events:
		if err == nil {
			t.Fatal("sharing violation expected")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("automatic attempt missing")
	}
	db.mu.RLock()
	retry := db.nextShrink
	db.mu.RUnlock()
	if retry.Before(time.Now().Add(4 * time.Minute)) {
		t.Fatal("missing retry backoff")
	}
	if err := safeSet(db, "profile", "still-writing"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
		t.Fatal("retried during cooldown")
	case <-time.After(1200 * time.Millisecond):
	}
	blocker.Close()
	db.mu.Lock()
	db.nextShrink = time.Time{}
	db.mu.Unlock()
	select {
	case err := <-events:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("automatic retry missing")
	}
	if safeGet(t, db, "profile") != "still-writing" {
		t.Fatal("automatic retry changed values")
	}
	after, _ := os.Stat(name)
	if after.Size() > 1024 {
		t.Fatal("automatic retry did not reclaim history")
	}
}
