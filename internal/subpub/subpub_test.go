package subpub

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newSilentBus(buf int) SubPub {
	return NewSubPubWithParams(buf, zap.NewNop())
}

func TestPublishOrderFIFO(t *testing.T) {
	bus := newSilentBus(8)
	defer bus.Close(context.Background())

	var got []int
	mu := sync.Mutex{}
	done := make(chan struct{})

	sub, err := bus.Subscribe("nums", func(msg interface{}) {
		mu.Lock()
		got = append(got, msg.(int))
		if len(got) == 100 {
			close(done)
		}
		mu.Unlock()
	})
	assert.NoError(t, err)
	defer sub.Unsubscribe()
	for i := 0; i < 100; i++ {
		assert.NoError(t, bus.Publish("nums", i))
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	bus.Close(context.Background())
	excepted := make([]int, 100)
	for i := 0; i < 100; i++ {
		excepted[i] = i
	}
	mu.Lock()
	assert.Equal(t, excepted, got)
	mu.Unlock()
}

func TestSlowSubscriberIsolation(t *testing.T) {
	bus := newSilentBus(4)
	defer bus.Close(context.Background())
	_, _ = bus.Subscribe("isolate", func(any) { time.Sleep(5 * time.Millisecond) })
	var fast int32
	_, _ = bus.Subscribe("isolate", func(any) { atomic.AddInt32(&fast, 1) })

	for i := 0; i < 50; i++ {
		_ = bus.Publish("isolate", i)
	}

	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&fast) == 50
	}, time.Second, 10*time.Millisecond)
}

func TestUnsubscribeStopDelivery(t *testing.T) {
	bus := newSilentBus(8)
	defer bus.Close(context.Background())
	var got int32
	var recv = make(chan struct{}, 1)
	sub, _ := bus.Subscribe("stop", func(any) {
		if atomic.AddInt32(&got, 1) == 1 {
			recv <- struct{}{}
		}
	})
	_ = bus.Publish("stop", 1)
	<-recv
	sub.Unsubscribe()
	_ = bus.Publish("stop", 2)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&got))
}

func TestErrorWithClosed(t *testing.T) {
	bus := newSilentBus(1)
	assert.NoError(t, bus.Close(context.Background()))
	_, errSub := bus.Subscribe("closed", func(any) {})
	assert.ErrorIs(t, errSub, ErrClosed)
	errPub := bus.Publish("closed", 1)
	assert.ErrorIs(t, errPub, ErrClosed)
}

func TestConcurrentFIFO(t *testing.T) {
	bus := newSilentBus(16)
	defer bus.Close(context.Background())
	const nSubs = 10
	const nMsgs = 200
	done := make(chan struct{}, nSubs)
	for i := 0; i < nSubs; i++ {
		prev := -1
		_, _ = bus.Subscribe("concure", func(m any) {
			if m.(int) < prev {
				t.Errorf("Wrong order for subscriber %d: %d < %d", i, m.(int), prev)
			}
			prev = m.(int)
			if prev == nMsgs-1 {
				done <- struct{}{}
			}
		})
	}
	for i := 0; i < nMsgs; i++ {
		_ = bus.Publish("concure", i)
	}
	timeout := time.After(2 * time.Second)
	for received := 0; received < nSubs; {
		select {
		case <-done:
			received++
		case <-timeout:
			t.Fatalf("only %d/%d subscribers received messages", received, nSubs)
		}
	}
}
