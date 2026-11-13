package dice

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/interaction/signature"
)

func TestOfficialQQWebhookBackport(t *testing.T) {
	pa := &PlatformAdapterOfficialQQ{AppID: "fixture-app", AppSecret: "fixture-secret"}
	const ts = "1794530000"
	const token = "fixture-token"
	const dispatchBody = `{"op":0,"t":"GROUP_AT_MESSAGE_CREATE","d":{}}`
	calls := 0
	request := func(method, body, signedBody string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/webhook", strings.NewReader(body))
		r.Header.Set("X-Bot-Appid", pa.AppID)
		if signedBody != "" {
			r.Header.Set(signature.HeaderTimestamp, ts)
			sig, err := signature.Generate(pa.AppSecret, r.Header, []byte(signedBody))
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set(signature.HeaderSig, sig)
		}
		w := httptest.NewRecorder()
		pa.respondWebhook(w, r, func(payload *dto.WSPayload) {
			calls++
			if string(payload.RawMessage) != dispatchBody || payload.Session.AppID != pa.AppID {
				t.Fatal("dispatch lost original body or bot identity")
			}
		})
		return w
	}
	w := request(http.MethodPost, `{"op":13,"d":{"plain_token":"fixture-token","event_ts":"1794530000"}}`, "")
	var ack dto.WHValidationRsp
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &ack) != nil || ack.PlainToken != token {
		t.Fatalf("invalid challenge response, status %d", w.Code)
	}
	h := http.Header{}
	h.Set(signature.HeaderTimestamp, ts)
	h.Set(signature.HeaderSig, ack.Signature)
	if ok, err := signature.Verify(pa.AppSecret, h, []byte(token)); !ok || err != nil {
		t.Fatal("challenge signature did not verify")
	}
	for _, tc := range []struct {
		name, method, body, signed string
		want                       int
	}{
		{"reject arbitrary signing data", "POST", `{"op":13,"d":{"plain_token":"{}","event_ts":"1794530000"}}`, "", 400},
		{"unsigned event", "POST", dispatchBody, "", 401},
		{"modified body", "POST", dispatchBody + " ", dispatchBody, 401},
		{"missing op", "POST", `{"t":"GROUP_AT_MESSAGE_CREATE"}`, "", 400},
		{"wrong method", "GET", "", "", 405},
		{"oversized body", "POST", strings.Repeat("x", (2<<20)+1), "", 413},
		{"valid signed event", "POST", dispatchBody, dispatchBody, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := request(tc.method, tc.body, tc.signed).Code; got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
	if calls != 1 {
		t.Fatalf("dispatched %d events, want 1", calls)
	}
}
