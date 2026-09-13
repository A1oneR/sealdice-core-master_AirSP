package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"sealdice-core/dice"
	"sealdice-core/dice/service"
)

// chatGroupList returns the list of groups for the chat sidebar.
func chatGroupList(c echo.Context) error {
	if !doAuth(c) {
		return c.JSON(http.StatusForbidden, nil)
	}

	cm := myDice.GetChatManager()
	var summaries []*dice.ChatGroupSummary

	// Default to filtering out groups with no messages
	// Can be toggled with hasMessagesOnly=false or showAll=true
	onlyWithMsgs := true
	if val := c.QueryParam("hasMessagesOnly"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			onlyWithMsgs = b
		}
	}
	if c.QueryParam("showAll") == "true" || c.QueryParam("showAll") == "1" {
		onlyWithMsgs = false
	}

	if myDice != nil && myDice.ImSession != nil && myDice.ImSession.ServiceAtNew != nil {
		myDice.ImSession.ServiceAtNew.Range(func(groupID string, group *dice.GroupInfo) bool {
			if group == nil || strings.HasPrefix(groupID, "PG-") {
				return true
			}

			lastMsg := cm.GetLastMessage(groupID)
			msgCount := cm.GetMessageCount(groupID)

			// Filter out groups with zero messages if onlyWithMsgs is true
			if onlyWithMsgs && msgCount == 0 && lastMsg == nil {
				return true
			}

			group.GroupID = groupID
			name := group.GroupName
			if name == "" && dm != nil {
				name = dm.TryGetGroupName(groupID)
			}
			if name == "" {
				name = groupID
			}

			isOfficial := strings.HasPrefix(groupID, "OpenQQ-Group:") || strings.HasPrefix(groupID, "OpenQQCH-Channel:")
			platform := "QQ"
			if isOfficial {
				platform = "OpenQQ"
			} else if dashIdx := strings.Index(groupID, "-"); dashIdx != -1 {
				platform = groupID[:dashIdx]
			}

			recentTime := group.RecentDiceSendTime
			if lastMsg != nil && lastMsg.Timestamp > recentTime {
				recentTime = lastMsg.Timestamp
			}

			active := group.Active
			if group.DiceIDActiveMap != nil && group.DiceIDActiveMap.Len() > 0 {
				active = true
			}

			summary := &dice.ChatGroupSummary{
				GroupID:    groupID,
				GroupName:  name,
				Platform:   platform,
				IsOfficial: isOfficial,
				Active:     active,
				MemberNum:  0, // lazy load only for paginated slice below
				LastMsg:    lastMsg,
				MsgCount:   msgCount,
				RecentTime: recentTime,
			}
			summaries = append(summaries, summary)
			return true
		})
	}

	// Sort groups: groups with recent activity first
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].RecentTime != summaries[j].RecentTime {
			return summaries[i].RecentTime > summaries[j].RecentTime
		}
		if summaries[i].MsgCount != summaries[j].MsgCount {
			return summaries[i].MsgCount > summaries[j].MsgCount
		}
		return summaries[i].GroupName < summaries[j].GroupName
	})

	total := len(summaries)
	page := 1
	if p := c.QueryParam("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}

	pageSize := 20
	if ps := c.QueryParam("pageSize"); ps != "" {
		if val, err := strconv.Atoi(ps); err == nil {
			pageSize = val
		}
	}

	var paged []*dice.ChatGroupSummary
	var totalPages int

	if pageSize <= 0 {
		// Return all without pagination if pageSize <= 0
		paged = summaries
		totalPages = 1
		page = 1
	} else {
		totalPages = (total + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		if page > totalPages {
			page = totalPages
		}
		start := (page - 1) * pageSize
		end := start + pageSize
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
		paged = summaries[start:end]
	}

	// Query member count only for the paginated subset to avoid massive SQL queries
	if myDice != nil && myDice.DBOperator != nil {
		for _, item := range paged {
			item.MemberNum, _ = service.GroupPlayerNumGet(myDice.DBOperator, item.GroupID)
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"items":           paged,
		"total":           total,
		"page":            page,
		"pageSize":        pageSize,
		"totalPages":      totalPages,
		"hasMessagesOnly": onlyWithMsgs,
	})
}

// chatMessageList returns recent messages for a specific group.
func chatMessageList(c echo.Context) error {
	if !doAuth(c) {
		return c.JSON(http.StatusForbidden, nil)
	}

	groupID := strings.TrimSpace(c.QueryParam("groupId"))
	if groupID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "groupId is required",
		})
	}

	limit := 40
	if l := c.QueryParam("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			if val > 500 {
				limit = 500
			} else {
				limit = val
			}
		}
	}

	var since int64
	if s := c.QueryParam("since"); s != "" {
		if val, err := strconv.ParseInt(s, 10, 64); err == nil {
			since = val
		}
	}

	var before int64
	if b := c.QueryParam("before"); b != "" {
		if val, err := strconv.ParseInt(b, 10, 64); err == nil {
			before = val
		}
	}

	messages, hasMore, total := myDice.GetChatManager().GetMessagesPaged(groupID, limit, since, before)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"messages": messages,
		"hasMore":  hasMore,
		"total":    total,
	})
}

// chatMessageSend handles sending pure messages to a group without extra markers.
func chatMessageSend(c echo.Context) (err error) {
	if !doAuth(c) {
		return c.JSON(http.StatusForbidden, nil)
	}

	// Defensive recovery: prevent any panic from killing the process and producing 502
	defer func() {
		if r := recover(); r != nil {
			if myDice != nil && myDice.Logger != nil {
				myDice.Logger.Errorf("chatMessageSend 异常恢复 (panic recovered): %v", r)
			}
			err = c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": fmt.Sprintf("消息发送服务内部异常: %v", r),
			})
		}
	}()

	if dm != nil && dm.JustForTest {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"testMode": true,
			"ok":       true,
		})
	}

	v := struct {
		GroupID string `json:"groupId"`
		Message string `json:"message"`
	}{}
	if bindErr := c.Bind(&v); bindErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "参数解析失败",
		})
	}

	v.GroupID = strings.TrimSpace(v.GroupID)
	v.Message = strings.TrimSpace(v.Message)

	if v.GroupID == "" || v.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "群组ID和消息内容不能为空",
		})
	}

	if myDice == nil || myDice.ImSession == nil || myDice.ImSession.ServiceAtNew == nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "核心会话未就绪",
		})
	}

	group, exists := myDice.ImSession.ServiceAtNew.Load(v.GroupID)
	if !exists {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "未找到对应的群组信息",
		})
	}

	var targetEP *dice.EndPointInfo

	// Helper to check if an endpoint is alive, connected, and has an active adapter
	isEpUsable := func(ep *dice.EndPointInfo) bool {
		return ep != nil && ep.Enable && ep.State == dice.StateConnected && ep.Adapter != nil
	}

	// 1. Check endpoints active/present in this group that are connected
	if group.DiceIDActiveMap != nil {
		group.DiceIDActiveMap.Range(func(diceID string, active bool) bool {
			if active {
				for _, ep := range myDice.ImSession.EndPoints {
					if ep.UserID == diceID && isEpUsable(ep) {
						targetEP = ep
						return false
					}
				}
			}
			return true
		})
	}

	if targetEP == nil && group.DiceIDExistsMap != nil {
		group.DiceIDExistsMap.Range(func(diceID string, exists bool) bool {
			if exists {
				for _, ep := range myDice.ImSession.EndPoints {
					if ep.UserID == diceID && isEpUsable(ep) {
						targetEP = ep
						return false
					}
				}
			}
			return true
		})
	}

	// 2. Fallback matching platform prefix
	if targetEP == nil {
		isOfficial := strings.HasPrefix(v.GroupID, "OpenQQ-Group:") || strings.HasPrefix(v.GroupID, "OpenQQCH-Channel:")
		for _, ep := range myDice.ImSession.EndPoints {
			if isEpUsable(ep) {
				if isOfficial && ep.Platform == "OpenQQ" {
					targetEP = ep
					break
				}
				if !isOfficial && ep.Platform == "QQ" {
					targetEP = ep
					break
				}
			}
		}
	}

	// 3. Fallback to any usable enabled endpoint
	if targetEP == nil {
		for _, ep := range myDice.ImSession.EndPoints {
			if isEpUsable(ep) {
				targetEP = ep
				break
			}
		}
	}

	if targetEP == nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "没有在线连接的机器人端点，无法发送消息，请先在连接管理中连接机器人",
		})
	}

	masterID := "UI:Master"
	masterName := "Master"

	ctx := &dice.MsgContext{
		Dice:           myDice,
		EndPoint:       targetEP,
		Session:        myDice.ImSession,
		Group:          group,
		MessageType:    "group",
		PrivilegeLevel: 100,
		Player: &dice.GroupPlayerInfo{
			Name:   masterName,
			UserID: masterID,
		},
	}

	// For Official QQ, if there is a recent member message within 5 minutes (300s),
	// set $tMsgID so Official QQ platform uses passive reply (被动回复),
	// which is exempt from proactive monthly quota limits and group-owner toggle restriction.
	if strings.HasPrefix(v.GroupID, "OpenQQ-Group:") || strings.HasPrefix(v.GroupID, "OpenQQCH-Channel:") {
		lastMsg := myDice.GetChatManager().GetLastMessage(v.GroupID)
		if lastMsg != nil && !lastMsg.IsBot && lastMsg.RawID != nil {
			rawIDStr := fmt.Sprint(lastMsg.RawID)
			if rawIDStr != "" && time.Now().Unix()-lastMsg.Timestamp < 300 {
				dice.VarSetValueStr(ctx, "$tMsgID", rawIDStr)
			}
		}
	}

	sendMsg := &dice.Message{
		Time:        time.Now().Unix(),
		MessageType: "group",
		Platform:    targetEP.Platform,
		GroupID:     v.GroupID,
		Sender: dice.SenderBase{
			UserID:   masterID,
			Nickname: masterName,
		},
		Message: v.Message,
	}

	// Send message directly to group without any extra markers
	dice.ReplyGroup(ctx, sendMsg, v.Message)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": "发送成功",
	})
}

// chatMessageClear clears cached chat messages for a group.
func chatMessageClear(c echo.Context) error {
	if !doAuth(c) {
		return c.JSON(http.StatusForbidden, nil)
	}

	v := struct {
		GroupID string `json:"groupId"`
	}{}
	if err := c.Bind(&v); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "参数解析失败",
		})
	}

	v.GroupID = strings.TrimSpace(v.GroupID)
	if v.GroupID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "groupId 不能为空",
		})
	}

	myDice.GetChatManager().ClearMessages(v.GroupID)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// chatImageGet serves local image files safely from data/images, data/, or cache/ directories.
func chatImageGet(c echo.Context) error {
	if !doAuth(c) {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error": "auth forbidden",
		})
	}

	relPath := strings.TrimSpace(c.QueryParam("path"))
	if relPath == "" {
		return c.NoContent(http.StatusBadRequest)
	}

	// Sanitize and resolve path
	relPath = filepath.FromSlash(relPath)
	abs, err := filepath.Abs(relPath)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	// Security check: strictly ensure the file is within cwd/data or cwd/cache
	dataPrefix := filepath.Join(cwd, "data")
	cachePrefix := filepath.Join(cwd, "cache")
	if !strings.HasPrefix(abs, dataPrefix) && !strings.HasPrefix(abs, cachePrefix) {
		return c.NoContent(http.StatusForbidden)
	}

	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		return c.NoContent(http.StatusNotFound)
	}

	return c.File(abs)
}
