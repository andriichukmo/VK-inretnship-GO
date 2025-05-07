package subpub

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"
)

type MessageHandler func(msg interface{})

type Subscription interface {
	Unsubscribe()
}

type SubPub interface {
	Subscribe(subject string, cb MessageHandler) (Subscription, error)
	Publish(subject string, msg interface{}) error
	Close(ctx context.Context) error
}

func NewSubPubWithParams(buffer int, logger *zap.Logger) SubPub {
	if buffer <= 0 {
		buffer = 1
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &bus{
		buffer:   buffer,
		subjects: make(map[string]*subject),
		logger:   logger,
	}
}

func NewSubPub() SubPub {
	return NewSubPubWithParams(1024, nil)
}

var (
	ErrClosed = errors.New("subpub: bus closed")
)

type bus struct {
	mu       sync.RWMutex
	closed   bool
	wg       sync.WaitGroup
	buffer   int
	subjects map[string]*subject
	logger   *zap.Logger
}

type subject struct {
	mu      sync.RWMutex
	handler map[*subscriber]struct{}
}

type subscriber struct {
	ch     chan interface{}
	done   chan struct{}
	cancel context.CancelFunc
}

type subscriptionImpl struct {
	once    sync.Once
	b       *bus
	subjkey string
	sub     *subscriber
}

func (s *subscriptionImpl) Unsubscribe() {
	s.once.Do(func() {
		s.b.logger.Debug("unsubscribe", zap.String("subject", s.subjkey))
		s.b.removeSubscriber(s.subjkey, s.sub)
	})
}

func (b *bus) Subscribe(key string, cb MessageHandler) (Subscription, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		b.logger.Warn("subscribe to closed bus", zap.String("subject", key))
		return nil, ErrClosed
	}
	b.mu.RUnlock()
	ctx, cancel := context.WithCancel(context.Background())
	sub := &subscriber{
		ch:     make(chan interface{}, b.buffer),
		done:   make(chan struct{}),
		cancel: cancel,
	}
	b.mu.Lock()
	s, ok := b.subjects[key]
	if !ok {
		s = &subject{
			handler: make(map[*subscriber]struct{}),
		}
		b.subjects[key] = s
	}
	s.mu.Lock()
	s.handler[sub] = struct{}{}
	s.mu.Unlock()
	b.mu.Unlock()
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer close(sub.done)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sub.ch:
				if !ok {
					return
				}
				cb(msg)
			}
		}
	}()
	b.logger.Debug("subscriber added", zap.String("subject", key), zap.Int("buffer", b.buffer))
	return &subscriptionImpl{
		b:       b,
		subjkey: key,
		sub:     sub,
	}, nil
}

func (b *bus) Publish(key string, msg interface{}) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		b.logger.Warn("publish to closed bus", zap.String("subject", key))
		return ErrClosed
	}
	s, ok := b.subjects[key]
	if !ok {
		b.mu.RUnlock()
		return nil
	}
	s.mu.RLock()
	subs := make([]*subscriber, 0, len(s.handler))
	for sub := range s.handler {
		subs = append(subs, sub)
	}
	s.mu.RUnlock()
	b.mu.RUnlock()
	for _, sub := range subs {
		select {
		case <-sub.done:
			continue
		case sub.ch <- msg:
		}
	}

	return nil
}

func (b *bus) Close(ctx context.Context) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	allSub := make([]*subscriber, 0)
	for _, s := range b.subjects {
		s.mu.Lock()
		for sub := range s.handler {
			allSub = append(allSub, sub)
		}
		s.mu.Unlock()
	}
	b.mu.Unlock()
	for _, sub := range allSub {
		sub.cancel()
	}
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		b.logger.Info("bus closed", zap.Int("subscribers", len(allSub)))
		return nil
	case <-ctx.Done():
		b.logger.Warn("bus close timeout", zap.Error(ctx.Err()))
		return ctx.Err()
	}
}
func (b *bus) removeSubscriber(key string, sub *subscriber) {
	sub.cancel()
	b.mu.RLock()
	s, ok := b.subjects[key]
	b.mu.RUnlock()
	if !ok {
		return
	}
	s.mu.Lock()
	delete(s.handler, sub)
	s.mu.Unlock()
	b.logger.Debug("subscriber removed", zap.String("subject", key))
}
