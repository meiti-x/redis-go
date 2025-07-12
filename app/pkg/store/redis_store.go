package store

import (
	"fmt"
	"net"
	"sort"
	"strings"
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
		if endID == "+" {
			endID = "9999999999999-9999999999999" // Set a high value for endID :)))
		}
		if id >= startID && id <= endID {
			keys = append(keys, id)
		}
	}
	sort.Strings(keys)

	var results []string
	for _, id := range keys {
		value := stream.Values[id]
		results = append(results, fmt.Sprintf("%s: %s", id, value))
	}

	return results, nil
}

func (s *StreamStore) WriteStreamItems(conn net.Conn, results []string) {
	for _, entry := range results {
		parts := strings.SplitN(entry, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		id := parts[0]
		valueStr := parts[1]

		fields := strings.Fields(valueStr)
		conn.Write([]byte("*2\r\n"))
		// 1) ID
		conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(id), id)))

		// 2) Inner array for field-value pairs
		conn.Write([]byte(fmt.Sprintf("*%d\r\n", len(fields))))
		for _, field := range fields {
			conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(field), field)))
		}
	}
}
