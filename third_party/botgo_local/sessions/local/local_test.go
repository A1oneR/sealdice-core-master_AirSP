package local

import (
	"context"
	"testing"
	"time"

	wss "github.com/gorilla/websocket"
	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
)

func TestConnectDelay(t *testing.T) {
	const interval = 2 * time.Second
	now := time.Unix(100000, 0)
	for _, tc := range []struct {
		name    string
		last    time.Time
		planned bool
		want    time.Duration
	}{
		{"gateway rotation after an hour", now.Add(-time.Hour), true, 0},
		{"rapid repeated rotation", now.Add(-time.Millisecond * 500), true, time.Millisecond * 1500},
		{"exactly at the limit", now.Add(-interval), true, 0},
		{"initial start", time.Time{}, false, interval},
		{"failed dial still waits", now.Add(-time.Hour), false, interval},
		{"no last start", time.Time{}, true, interval},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := connectDelay(interval, tc.last, now, tc.planned); got != tc.want {
				t.Fatalf("delay = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPlannedResumeRequiresRetainedSession(t *testing.T) {
	session := dto.Session{ID: "local-test-session", LastSeq: 42}
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{errs.ErrNeedReConnect, true},
		{&wss.CloseError{Code: errs.WSCodeBackendSessionTimeOut}, true},
		{&wss.CloseError{Code: errs.WSCodeBackendSessionNoLongerValid}, false},
		{errs.ErrInvalidSession, false},
		{&wss.CloseError{Code: errs.WSCodeBackendRateLimit}, false},
		{context.DeadlineExceeded, false},
	} {
		if got := isPlannedResume(session, tc.err); got != tc.want {
			t.Errorf("error %v: fast resume=%v want %v", tc.err, got, tc.want)
		}
		if isPlannedResume(dto.Session{}, tc.err) {
			t.Errorf("error %v: a fresh IDENTIFY was marked as fast RESUME", tc.err)
		}
	}
}
