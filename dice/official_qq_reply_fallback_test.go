package dice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/sealdice/botgo/dto"
	qqerrs "github.com/sealdice/botgo/errs"
)

func TestOfficialQQReplyFallbackPacket(t *testing.T) {
	for _, tc := range []struct {
		name           string
		optIn          bool
		passiveErr     error
		eventAnchor    bool
		firstResult    bool
		activeErr      error
		quotaErr       error
		wantPosts      int
		wantQuotaCalls int
		wantFallback   bool
		wantError      bool
	}{
		{name: "passive success", optIn: true, wantPosts: 1},
		{name: "expired msg_id", optIn: true, passiveErr: qqerrs.New(400, `{"code":40034005}`), wantPosts: 2, wantQuotaCalls: 1, wantFallback: true},
		{name: "expired event", optIn: true, eventAnchor: true, passiveErr: qqerrs.New(400, `{"err_code":304027}`), wantPosts: 2, wantQuotaCalls: 1, wantFallback: true},
		{name: "passive limit, active failure", optIn: true, passiveErr: qqerrs.New(40034128, ""), activeErr: errors.New("active forbidden"), wantPosts: 2, wantQuotaCalls: 1, wantFallback: true, wantError: true},
		{name: "quota prevents fallback send", optIn: true, passiveErr: qqerrs.New(40034128, ""), quotaErr: errors.New("quota"), wantPosts: 1, wantQuotaCalls: 1, wantFallback: true, wantError: true},
		{name: "ordinary reply unchanged", passiveErr: qqerrs.New(304027, ""), wantPosts: 1, wantError: true},
		{name: "uncertain timeout", optIn: true, passiveErr: context.DeadlineExceeded, wantPosts: 1, wantError: true},
		{name: "not permitted", optIn: true, passiveErr: qqerrs.New(40034024, ""), wantPosts: 1, wantError: true},
		{name: "pending review", optIn: true, passiveErr: qqerrs.New(304024, ""), wantPosts: 1, wantError: true},
		{name: "returned a result", optIn: true, firstResult: true, passiveErr: qqerrs.New(304027, ""), wantPosts: 1, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &dto.MessageToCreate{
				MsgID: "inbound-id", MsgSeq: 22, MsgType: 7, Content: "report",
				Media:            &dto.MediaInfo{FileInfo: []byte("uploaded-report")},
				MessageReference: &dto.MessageReference{MessageID: "quote"},
			}
			if tc.eventAnchor {
				p.MsgID, p.EventID = "", "event-id"
			}
			original, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			posts, quotaCalls, notes := 0, 0, 0
			res, err := sendOfficialQQPacket(p, tc.optIn, func() error {
				quotaCalls++
				return tc.quotaErr
			}, func(got *dto.MessageToCreate) (*dto.Message, error) {
				posts++
				wire, e := json.Marshal(got)
				if e != nil {
					t.Fatal(e)
				}
				if posts == 1 {
					if string(wire) != string(original) {
						t.Fatalf("passive payload changed: %s", wire)
					}
					if tc.passiveErr == nil || tc.firstResult {
						return &dto.Message{ID: "passive-result"}, tc.passiveErr
					}
					return nil, fmt.Errorf("SDK: %w", tc.passiveErr)
				}
				var obj map[string]json.RawMessage
				if e := json.Unmarshal(wire, &obj); e != nil {
					t.Fatal(e)
				}
				for _, key := range []string{"msg_id", "event_id", "msg_seq", "message_reference"} {
					if _, ok := obj[key]; ok {
						t.Errorf("active payload contains %s: %s", key, wire)
					}
				}
				if got.Media != p.Media || got.Content != p.Content || got.MsgType != p.MsgType {
					t.Error("active send lost already-uploaded media or content")
				}
				if tc.activeErr != nil {
					return nil, tc.activeErr
				}
				return &dto.Message{ID: "active-result"}, nil
			}, func(code int) {
				notes++
				if !officialQQPassiveReplyRejected(code) {
					t.Errorf("unexpected fallback code: %d", code)
				}
			})
			if posts != tc.wantPosts || quotaCalls != tc.wantQuotaCalls || (notes == 1) != tc.wantFallback {
				t.Fatalf("posts=%d quota=%d fallback notes=%d", posts, quotaCalls, notes)
			}
			if (err != nil) != tc.wantError || (!tc.wantError && res == nil) {
				t.Fatalf("result=%v error=%v", res, err)
			}
			after, _ := json.Marshal(p)
			if string(after) != string(original) {
				t.Error("fallback mutated the passive payload")
			}
		})
	}
}

func TestOfficialQQReplyFallbackAlreadyActiveDoesNotRetry(t *testing.T) {
	count, quota := 0, 0
	want := qqerrs.New(304027, "")
	_, err := sendOfficialQQPacket(&dto.MessageToCreate{Content: "result"}, true, func() error {
		quota++
		return nil
	}, func(*dto.MessageToCreate) (*dto.Message, error) {
		count++
		return nil, want
	}, func(int) { t.Fatal("already active") })
	if count != 1 || quota != 1 || err != want {
		t.Fatalf("count=%d quota=%d err=%v", count, quota, err)
	}
}
