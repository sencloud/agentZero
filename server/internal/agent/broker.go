// Package agent 实现"特工"运转的核心：派遣管理、Agent loop、事件广播。
package agent

import (
	"sync"

	"github.com/agentzero/server/internal/model"
)

// Broker 是 mission 维度的 pub/sub。
//
// 它的目的是把 agent loop 产出的 Step 实时推给可能正在 SSE 连接的客户端。
// Step 同时还会被持久化到 DB，所以新订阅者可以先从 DB 拉历史，然后 Subscribe
// 跟上后续增量。
//
// 设计取舍：
//   - 订阅者收不过来时只发警告日志，不会阻塞 publisher（agent loop 必须不阻塞）；
//   - 每个 chan buffer 设 256 条，正常任务一轮 ~ 几十 step，留够余量；
//   - 没有持久化在 broker 里——重连读历史靠 DB。
type Broker struct {
	mu   sync.RWMutex
	subs map[string]map[chan *model.Step]struct{} // missionID -> set of channels
}

func NewBroker() *Broker {
	return &Broker{subs: map[string]map[chan *model.Step]struct{}{}}
}

// Subscribe 返回一个 chan，每当该 mission 有新 Step 时会被推送。
// 调用方必须保证最终调用 Unsubscribe，否则 chan 不会被 close。
func (b *Broker) Subscribe(missionID string) chan *model.Step {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan *model.Step, 256)
	set, ok := b.subs[missionID]
	if !ok {
		set = map[chan *model.Step]struct{}{}
		b.subs[missionID] = set
	}
	set[ch] = struct{}{}
	return ch
}

// Unsubscribe 移除订阅并关闭 chan。
func (b *Broker) Unsubscribe(missionID string, ch chan *model.Step) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if set, ok := b.subs[missionID]; ok {
		if _, exists := set[ch]; exists {
			delete(set, ch)
			close(ch)
		}
		if len(set) == 0 {
			delete(b.subs, missionID)
		}
	}
}

// Publish 把一条 Step 广播到该 mission 的所有订阅者。
// 收不过来的订阅者会被静默丢弃（保护 agent loop 不被慢客户端阻塞）。
func (b *Broker) Publish(missionID string, step *model.Step) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	set, ok := b.subs[missionID]
	if !ok {
		return
	}
	for ch := range set {
		select {
		case ch <- step:
		default:
			// 慢订阅者，本条丢弃。客户端可以靠重连 + DB 历史重放追回。
		}
	}
}

// FinishMission 在 mission 进入终态时关闭所有还连着的订阅，让 SSE handler 可以正常退出。
func (b *Broker) FinishMission(missionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	set, ok := b.subs[missionID]
	if !ok {
		return
	}
	for ch := range set {
		close(ch)
	}
	delete(b.subs, missionID)
}
