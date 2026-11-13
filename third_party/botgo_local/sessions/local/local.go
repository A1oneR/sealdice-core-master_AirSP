// Package local 基于 golang chan 实现的单机 manager。
package local

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	wss "github.com/gorilla/websocket"
	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/log"
	"github.com/sealdice/botgo/sessions/manager"
	"github.com/sealdice/botgo/websocket"
	"golang.org/x/oauth2"
)

// New 创建本地session管理器
func New() *ChanManager {
	return &ChanManager{}
}

// ChanManager 默认的本地 session manager 实现
type ChanManager struct {
	sessionChan chan connectRequest
}

type connectRequest struct {
	session       dto.Session
	plannedResume bool
}

// A gateway-requested RESUME can use a slot that is already available.
// All other attempts keep the existing backoff, and rapid RESUMEs are spaced.
func connectDelay(interval time.Duration, lastStart, now time.Time, plannedResume bool) time.Duration {
	if !plannedResume || lastStart.IsZero() {
		return interval
	}
	delay := lastStart.Add(interval).Sub(now)
	if delay < 0 {
		return 0
	}
	return delay
}

func (l *ChanManager) enqueue(ctx context.Context, req connectRequest) {
	select {
	case l.sessionChan <- req:
	case <-ctx.Done():
	}
}

func isPlannedResume(session dto.Session, err error) bool {
	return session.ID != "" &&
		(errors.Is(err, errs.ErrNeedReConnect) || wss.IsCloseError(err, errs.WSCodeBackendSessionTimeOut))
}

// Start 启动本地 session manager
func (l *ChanManager) Start(ctx context.Context, apInfo *dto.WebsocketAP, tokenSource oauth2.TokenSource, intents *dto.Intent) error {
	defer log.Sync()
	if err := manager.CheckSessionLimit(apInfo); err != nil {
		log.Errorf("[ws/session/local] session limited apInfo: %+v", apInfo)
		return err
	}
	startInterval := manager.CalcInterval(apInfo.SessionStartLimit.MaxConcurrency)
	log.Infof("[ws/session/local] will start %d sessions and per session start interval is %s",
		apInfo.Shards, startInterval)

	var appID string
	if botTokenSource, ok := tokenSource.(interface{ GetAppID() string }); ok {
		appID = botTokenSource.GetAppID()
	}

	// 按照shards数量初始化，用于启动连接的管理
	l.sessionChan = make(chan connectRequest, apInfo.Shards)
	for i := uint32(0); i < apInfo.Shards; i++ {
		session := dto.Session{
			URL:         apInfo.URL,
			TokenSource: tokenSource,
			Intent:      *intents,
			LastSeq:     0,
			Shards: dto.ShardConfig{
				ShardID:    i,
				ShardCount: apInfo.Shards,
			},
			AppID: appID,
		}
		l.sessionChan <- connectRequest{session: session}
	}

	var lastStart time.Time
	for {
		var req connectRequest
		select {
		case <-ctx.Done():
			log.Infof("[ws/session/local] context done, stop start session")
			return nil
		case req = <-l.sessionChan:
		}
		delay := connectDelay(startInterval, lastStart, time.Now(), req.plannedResume)
		if req.plannedResume {
			log.Infof("[ws/session/local] gateway requested RESUME; local scheduling delay %s", delay.Round(time.Millisecond))
		}
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			return nil
		}
		lastStart = time.Now()
		go l.newConnect(ctx, req.session)
	}
}

// newConnect 启动一个新的连接，如果连接在监听过程中报错了，或者被远端关闭了链接，需要识别关闭的原因，能否继续 resume
// 如果能够 resume，则往 sessionChan 中放入带有 sessionID 的 session
// 如果不能，则清理掉 sessionID，将 session 放入 sessionChan 中
// session 的启动，交给 start 中的 for 循环执行，session 不自己递归进行重连，避免递归深度过深
func (l *ChanManager) newConnect(ctx context.Context, session dto.Session) {
	defer func() {
		// panic 留下日志，放回 session
		if err := recover(); err != nil {
			websocket.PanicHandler(err, &session)
			l.enqueue(ctx, connectRequest{session: session})
		}
	}()
	wsClient := websocket.ClientImpl.New(ctx, session)
	if err := wsClient.Connect(); err != nil {
		log.Error(err)
		l.enqueue(ctx, connectRequest{session: session}) // 连接失败，按原间隔重连
		return
	}
	var err error
	// 如果 session id 不为空，则执行的是 resume 操作，如果为空，则执行的是 identify 操作
	if session.ID != "" {
		err = wsClient.Resume()
	} else {
		// 初次鉴权
		err = wsClient.Identify()
	}
	if err != nil {
		log.Errorf("[ws/session] Identify/Resume err %+v", err)
		wsClient.Close()
		session.ID = ""
		session.LastSeq = 0
		l.enqueue(ctx, connectRequest{session: session})
		return
	}
	if err = wsClient.Listening(); err != nil {
		log.Errorf("[ws/session] Listening err %+v", err)
		currentSession := wsClient.Session()
		// 如果是 4014 disallowed intents 错误，降级 intents 避开高级事件，ws没有权限，所以不进行后续请求
		if strings.Contains(err.Error(), "4014") || strings.Contains(err.Error(), "disallowed intents") {
			log.Infof("[ws/session] disallowed intents (4014) detected, downgrading intents to standard public bot intents")
			currentSession.Intent = currentSession.Intent &^ (dto.IntentGroupMembers | dto.IntentEnterAIO)
		}
		// 对于不能够进行重连的session，需要清空 session id 与 seq
		if manager.CanNotResume(err) {
			currentSession.ID = ""
			currentSession.LastSeq = 0
		}
		// 一些错误不能够鉴权，比如机器人被封禁，这里就直接退出了
		if manager.CanNotIdentify(err) {
			msg := fmt.Sprintf("can not identify because server return %+v, so process exit", err)
			log.Errorf(msg)
			panic(msg) // 当机器人被下架，或者封禁，将不能再连接，所以 panic
		}
		// 将 session 放到 session chan 中，用于启动新的连接，当前连接退出
		l.enqueue(ctx, connectRequest{session: *currentSession, plannedResume: isPlannedResume(*currentSession, err)})
		return
	}
}
