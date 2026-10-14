package api

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"sealdice-core/dice"
)

func chatSessionProfile(c echo.Context) error {
	if !doAuth(c) {
		return c.JSON(http.StatusForbidden, nil)
	}
	id := strings.TrimSpace(c.QueryParam("groupId"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "groupId is required"})
	}
	if myDice.ImSession == nil || myDice.ImSession.ServiceAtNew == nil {
		return c.NoContent(http.StatusNotFound)
	}
	group, ok := myDice.ImSession.ServiceAtNew.Load(id)
	if !ok || group == nil {
		return c.NoContent(http.StatusNotFound)
	}
	profile := dice.ChatGroupProfile{GroupName: group.GroupName, AvatarURL: dice.ChatGroupAvatar(id)}
	if dm != nil {
		if cached, ok := dm.GroupNameCache.Load(id); ok && cached.Name != "" && cached.Name != "%未知群名%" {
			profile.GroupName = cached.Name
		}
	}
	available := false
	for _, ep := range myDice.ImSession.EndPoints {
		if ep == nil || !ep.Enable || ep.State != dice.StateConnected || ep.Adapter == nil {
			continue
		}
		if pa, ok := ep.Adapter.(*dice.PlatformAdapterOfficialQQ); ok && strings.HasPrefix(id, "OpenQQ-Group:"+pa.UIN+"-") {
			ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
			fresh, _ := pa.ChatGroupProfile(ctx, id)
			cancel()
			if fresh.GroupName != "" {
				profile = fresh
				available = true
			}
			break
		}
		if strings.HasPrefix(id, "QQ-Group:") && ep.Platform == "QQ" {
			if (group.DiceIDExistsMap != nil && group.DiceIDExistsMap.Exists(ep.UserID)) || (group.DiceIDActiveMap != nil && group.DiceIDActiveMap.Exists(ep.UserID)) {
				if dm != nil && dm.ShouldRefreshGroupInfo(id) {
					go ep.Adapter.GetGroupInfoAsync(id)
				}
				available = true
				break
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"groupName": profile.GroupName, "avatarUrl": profile.AvatarURL, "memberNum": profile.MemberNum, "profileAvailable": available})
}

func chatPathInside(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func chatResourceImageCode(path string) (string, error) {
	abs, err := filepath.Abs(filepath.FromSlash(path))
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs("data/images")
	if err != nil {
		return "", err
	}
	if !chatPathInside(root, abs) {
		return "", os.ErrPermission
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	if !chatPathInside(resolvedRoot, resolved) {
		return "", os.ErrPermission
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	kind, _ := getResourceType(resolved)
	if !info.Mode().IsRegular() || kind != Image {
		return "", os.ErrPermission
	}
	urlPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	uri := (&url.URL{Scheme: "file", Path: urlPath}).String()
	uri = strings.NewReplacer("&", "&amp;", "[", "&#91;", "]", "&#93;", ",", "&#44;").Replace(uri)
	return "[CQ:image,file=" + uri + "]", nil
}
