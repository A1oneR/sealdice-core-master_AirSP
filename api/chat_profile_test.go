package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"sealdice-core/dice"
	"sealdice-core/message"
)

func TestChatResourceSendAndPathBoundaries(t *testing.T) {
	token, _ := setupTestChatAPI(t)
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("data/images", 0755); err != nil {
		t.Fatal(err)
	}
	name := "data/images/图片 [1], & 空格.png"
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jJawAAAAASUVORK5CYII=")
	if err := os.WriteFile(name, image, 0600); err != nil {
		t.Fatal(err)
	}
	code, err := chatResourceImageCode(name)
	if err != nil {
		t.Fatal(err)
	}
	elems := message.ConvertStringMessage(code)
	if len(elems) != 1 {
		t.Fatalf("image code parsed into %d segments", len(elems))
	}
	if _, ok := elems[0].(*message.ImageElement); !ok {
		t.Fatalf("image parsed as %T", elems[0])
	}
	dm.JustForTest = true
	body, _ := json.Marshal(map[string]string{"targetId": "QQ-Group:123456", "resourcePath": name})
	if rec := performReplyAPIRequest(t, http.MethodPost, "/sd-api/chat/send", string(body), token, chatMessageSend); rec.Code != 200 {
		t.Fatalf("resource-only send: %d %s", rec.Code, rec.Body.String())
	}
	for _, path := range []string{"../outside.png", "data/images-other/avatar.png", "data/images", "data/images/missing.png"} {
		if _, err := chatResourceImageCode(path); err == nil {
			t.Fatal("accepted invalid resource path")
		}
	}
	if err := os.MkdirAll("data/images-other", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("data/images-other/avatar.png", image, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := chatResourceImageCode("data/images-other/avatar.png"); err == nil {
		t.Fatal("accepted sibling directory")
	}
	if rec := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/image?path=data/images-other/avatar.png", "", token, chatImageGet); rec.Code != 200 {
		t.Fatal("allowed data subdirectory should remain readable")
	}
	if err := os.MkdirAll("data-other", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("data-other/avatar.png", image, 0600); err != nil {
		t.Fatal(err)
	}
	if rec := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/image?path=data-other/avatar.png", "", token, chatImageGet); rec.Code != http.StatusForbidden {
		t.Fatal("image endpoint accepted data sibling")
	}
	escaped, _ := filepath.Abs("data-other/avatar.png")
	if err := os.Symlink(escaped, "data/images/escaped.png"); err == nil {
		if _, err := chatResourceImageCode("data/images/escaped.png"); err == nil {
			t.Fatal("resource symlink escaped root")
		}
	}
}

func TestChatProfileAuthAndCachedName(t *testing.T) {
	token, testDice := setupTestChatAPI(t)
	if rec := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/profile?groupId=QQ-Group:123456", "", "wrong", chatSessionProfile); rec.Code != http.StatusForbidden {
		t.Fatal("profile lacks authentication")
	}
	group := &dice.GroupInfo{GroupID: "QQ-Group:123456", GroupName: "旧群名"}
	testDice.ImSession.ServiceAtNew.Store(group.GroupID, group)
	dm.GroupNameCache.Store(group.GroupID, &dice.GroupNameCacheItem{Name: "真实群名"})
	rec := performReplyAPIRequest(t, http.MethodGet, "/sd-api/chat/profile?groupId=QQ-Group:123456", "", token, chatSessionProfile)
	var profile dice.ChatGroupProfile
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || profile.GroupName != "真实群名" || profile.AvatarURL == "" {
		t.Fatal("cached profile missing")
	}
}
