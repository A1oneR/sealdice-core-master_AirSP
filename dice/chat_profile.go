package dice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	qqconstant "github.com/sealdice/botgo/constant"
)

var chatNumericID = regexp.MustCompile(`^[1-9][0-9]{4,19}$`)

// OpenIDs are opaque: never substitute one into a numeric QQ avatar URL.
func ChatUserAvatar(userID, supplied string) string {
	if u, err := url.Parse(strings.TrimSpace(supplied)); err == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http") {
		return u.String()
	}
	if id, ok := strings.CutPrefix(userID, "QQ:"); ok && chatNumericID.MatchString(id) {
		return "https://q1.qlogo.cn/g?b=qq&nk=" + id + "&s=100"
	}
	return ""
}

func ChatGroupAvatar(groupID string) string {
	if id, ok := strings.CutPrefix(groupID, "QQ-Group:"); ok && chatNumericID.MatchString(id) {
		return "https://p.qlogo.cn/gh/" + id + "/" + id + "/100"
	}
	return ""
}

type ChatGroupProfile struct {
	GroupName string `json:"groupName"`
	MemberNum int64  `json:"memberNum"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

type officialQQChatProfileCache struct {
	profile ChatGroupProfile
	expires time.Time
}

func fetchOfficialQQChatGroupProfile(ctx context.Context, api officialQQTransport, groupOpenID string) (ChatGroupProfile, error) {
	body, err := api.Transport(ctx, http.MethodGet, qqconstant.APIDomain+"/v2/groups/"+url.PathEscape(groupOpenID)+"/info", nil)
	if err != nil {
		return ChatGroupProfile{}, err
	}
	var info struct {
		Name    string `json:"group_name"`
		Members int64  `json:"group_member_num"`
	}
	if err = json.Unmarshal(body, &info); err != nil {
		return ChatGroupProfile{}, err
	}
	if strings.TrimSpace(info.Name) == "" {
		return ChatGroupProfile{}, errors.New("group info has no name")
	}
	// The documented group-info response has no avatar field.
	return ChatGroupProfile{GroupName: info.Name, MemberNum: info.Members}, nil
}

// ChatGroupProfile caches successful reads for ten minutes, failures for one
// minute, and spaces requests by two seconds (the upstream limit is 30 QPM).
func (pa *PlatformAdapterOfficialQQ) ChatGroupProfile(ctx context.Context, groupID string) (ChatGroupProfile, error) {
	openid, ok := strings.CutPrefix(groupID, "OpenQQ-Group:"+pa.UIN+"-")
	if !ok || openid == "" || pa.Api == nil {
		return ChatGroupProfile{}, errors.New("group info unavailable")
	}
	now := time.Now()
	pa.chatProfileMu.Lock()
	cached := pa.chatProfiles[groupID]
	if now.Before(cached.expires) {
		pa.chatProfileMu.Unlock()
		return cached.profile, nil
	}
	if now.Before(pa.chatProfileNext) {
		pa.chatProfileMu.Unlock()
		return cached.profile, errors.New("group info rate limited")
	}
	if pa.chatProfiles == nil {
		pa.chatProfiles = make(map[string]officialQQChatProfileCache)
	}
	pa.chatProfileNext = now.Add(2 * time.Second)
	pa.chatProfiles[groupID] = officialQQChatProfileCache{profile: cached.profile, expires: now.Add(time.Minute)}
	pa.chatProfileMu.Unlock()

	profile, err := fetchOfficialQQChatGroupProfile(ctx, pa.Api, openid)
	if err != nil {
		return cached.profile, err
	}
	pa.chatProfileMu.Lock()
	pa.chatProfiles[groupID] = officialQQChatProfileCache{profile: profile, expires: time.Now().Add(10 * time.Minute)}
	pa.chatProfileMu.Unlock()
	if pa.EndPoint != nil && pa.EndPoint.Session != nil && pa.EndPoint.Session.Parent != nil && pa.EndPoint.Session.Parent.Parent != nil {
		pa.EndPoint.Session.Parent.Parent.GroupNameCache.Store(groupID, &GroupNameCacheItem{Name: profile.GroupName, time: time.Now().Unix()})
	}
	return profile, nil
}
