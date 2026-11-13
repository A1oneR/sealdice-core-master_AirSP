package client

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sealdice/botgo/dto"
	"github.com/stretchr/testify/assert"
)

func TestClientKeepaliveAndTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := dto.Session{
		URL: "wss://test.local/ws",
	}

	cli := (&Client{}).New(ctx, session).(*Client)
	assert.NotNil(t, cli)

	// 1. 默认心跳周期 30s，读写超时应为 30*2 + 15 = 75s
	assert.Equal(t, 75*time.Second, cli.getReadTimeout())

	// 2. 模拟收到 Hello 包 HeartbeatInterval 为 45000ms
	helloJSON := []byte(`{"op":10,"d":{"heartbeat_interval":45000}}`)
	cli.startHeartBeatTicker(helloJSON)
	assert.Equal(t, 45*time.Second, time.Duration(atomic.LoadInt64(&cli.heartbeatIntervalNs)))
	assert.Equal(t, 105*time.Second, cli.getReadTimeout())

	// 3. 模拟极小心跳周期 10000ms，最低保障不少于 60s
	helloSmallJSON := []byte(`{"op":10,"d":{"heartbeat_interval":10000}}`)
	cli.startHeartBeatTicker(helloSmallJSON)
	assert.Equal(t, 60*time.Second, cli.getReadTimeout())

	// 4. markAlive 更新测试
	oldTime := atomic.LoadInt64(&cli.lastAliveTime)
	time.Sleep(10 * time.Millisecond)
	cli.markAlive()
	newTime := atomic.LoadInt64(&cli.lastAliveTime)
	assert.Greater(t, newTime, oldTime)

	// 5. notifyClose 非阻塞验证
	// closeChan 缓冲为 10，连续塞入 15 个错误不应死锁
	for i := 0; i < 15; i++ {
		cli.notifyClose(fmt.Errorf("err-%d", i))
	}
	assert.Equal(t, 10, len(cli.closeChan))
}
