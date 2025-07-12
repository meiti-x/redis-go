package store

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type StoreItem struct {
	Value  string
	Expiry int
}

type Stream struct {
	Id     string
	Values map[string]string
	LastID string
}

type SetStore struct {
	sync.RWMutex
	Items      map[string]StoreItem
	Entries_mu sync.RWMutex
	Entry      map[string]Stream
	Stop       chan struct{}
}

type StreamStore struct {
	Mu       sync.RWMutex
	Entry    map[string]Stream
	Stop     chan struct{}
	LastTime int64
	LastSeq  uint64
}

type Store struct {
	StreamStore
	SetStore
}

func NewRedisStore() *Store {
	set_store := &SetStore{
		Items: make(map[string]StoreItem),
		Entry: make(map[string]Stream),
		Stop:  make(chan struct{}),
	}
	stream_store := &StreamStore{
		Entry: make(map[string]Stream),
		Stop:  make(chan struct{}),
	}
	// Cleanup expired items in set
	go set_store.startCleanupJob()
	go stream_store.startStreamCleanupJob()

	return &Store{
		SetStore:    *set_store,
		StreamStore: *stream_store,
	}

}

func (s *SetStore) StopChannel() {
	close(s.Stop)
}

func (s *SetStore) startCleanupJob() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.Stop:
			return
		case <-ticker.C:
			s.cleanupExpiredItems()
		}
	}
}

func (s *StreamStore) startStreamCleanupJob() {
	for {
		select {
		case <-s.Stop:
			return
		}
	}
}

func (s *SetStore) cleanupExpiredItems() {
	s.Lock()
	defer s.Unlock()

	currentTime := int(time.Now().UnixMilli())
	for key, item := range s.Items {
		if item.Expiry > 0 && item.Expiry < currentTime {
			fmt.Printf("Removing expired item: %s\n", key)
			delete(s.Items, key)
		}
	}
}

func (s *StreamStore) GetRange(streamName, startID, endID string) ([]string, error) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	stream, exists := s.Entry[streamName]
	if !exists {
		return nil, fmt.Errorf("stream %s does not exist", streamName)
	}

	var keys []string
	for id := range stream.Values {
		if id >= startID && id <= endID {
			keys = append(keys, id)
		}
	}
	sort.Strings(keys) // Ensures deterministic order

	var results []string
	for _, id := range keys {
		value := stream.Values[id]
		results = append(results, fmt.Sprintf("%s: %s", id, value))
	}

	return results, nil
}
