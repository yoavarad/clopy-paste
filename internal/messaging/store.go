package messaging

import (
	"container/list"
	"log/slog"
	"sync"
)

const (
	maxUsers           = 100
	maxMessagesPerUser = 100
)

// NewStore returns the in-memory Store using the default logger.
func NewStore() Store {
	return NewStoreWithLogger(slog.Default())
}

// NewStoreWithLogger returns the in-memory Store logging evictions to logger.
func NewStoreWithLogger(logger *slog.Logger) Store {
	return &memStore{
		logger:  logger,
		buckets: make(map[string]*list.Element, maxUsers),
		lru:     list.New(),
	}
}

type bucket struct {
	user string
	msgs []Message // oldest first
}

// memStore: one mutex guards the map and the recency list (front = MRU).
type memStore struct {
	mu      sync.Mutex
	logger  *slog.Logger
	buckets map[string]*list.Element
	lru     *list.List
}

// Add implements Store.
func (s *memStore) Add(user string, msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	el, ok := s.buckets[user]
	if ok {
		s.lru.MoveToFront(el)
	} else {
		if s.lru.Len() >= maxUsers {
			old := s.lru.Back()
			ob := old.Value.(*bucket)
			s.lru.Remove(old)
			delete(s.buckets, ob.user)
			s.logger.Info("user evicted", "user", ob.user)
		}
		el = s.lru.PushFront(&bucket{user: user})
		s.buckets[user] = el
	}
	b := el.Value.(*bucket)
	if len(b.msgs) >= maxMessagesPerUser {
		// Drop oldest; reslicing would pin the dropped element, so shift.
		copy(b.msgs, b.msgs[1:])
		b.msgs = b.msgs[:len(b.msgs)-1]
	}
	b.msgs = append(b.msgs, msg)
}

// Get implements Store.
func (s *memStore) Get(user string) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	el, ok := s.buckets[user]
	if !ok {
		return []Message{}
	}
	s.lru.MoveToFront(el)
	msgs := el.Value.(*bucket).msgs
	out := make([]Message, len(msgs))
	for i, m := range msgs {
		out[len(msgs)-1-i] = m
	}
	return out
}
