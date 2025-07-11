package internal

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	resp "github.com/codecrafters-io/redis-starter-go/app/pkg"
)

type StoreItem struct {
	Value  string
	Expiry int
}

type Stream struct {
	id     string
	values map[string]string
}

type SetStore struct {
	sync.RWMutex
	items      map[string]StoreItem
	entries_mu sync.RWMutex
	entry      map[string]Stream
	stop       chan struct{}
}

type StreamStore struct {
	mu       sync.RWMutex
	entry    map[string]Stream
	stop     chan struct{}
	lastTime int64
	lastSeq  uint64
}

type Store struct {
	StreamStore
	SetStore
}

func NewConcurrentStore() *Store {
	set_store := &SetStore{
		items: make(map[string]StoreItem),
		entry: make(map[string]Stream),
		stop:  make(chan struct{}),
	}
	stream_store := &StreamStore{
		entry: make(map[string]Stream),
		stop:  make(chan struct{}),
	}
	// Cleanup expired items in set
	go set_store.startCleanupJob()

	return &Store{
		SetStore:    *set_store,
		StreamStore: *stream_store,
	}

}

func (s *SetStore) Stop() {
	close(s.stop)
}

func (s *SetStore) startCleanupJob() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.cleanupExpiredItems()
		}
	}
}

func (s *SetStore) cleanupExpiredItems() {
	s.Lock()
	defer s.Unlock()

	currentTime := int(time.Now().UnixMilli())
	for key, item := range s.items {
		if item.Expiry > 0 && item.Expiry < currentTime {
			fmt.Printf("Removing expired item: %s\n", key)
			delete(s.items, key)
		}
	}
}

func HandleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	store := NewConcurrentStore()

	for {
		command, args, err := resp.Parse(reader)
		if err != nil {
			conn.Write([]byte("-ERR " + err.Error() + "\r\n"))
			return
		}

		command = strings.ToUpper(command)
		switch command {
		case "PING":
			conn.Write([]byte("+PONG\r\n"))
		case "ECHO":
			conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(args[0]), args[0])))
		case "SET":
			expiry := 0

			// Check for PX argument if there are enough args
			if len(args) >= 4 && strings.ToUpper(args[2]) == "PX" {
				expiryMillis, err := strconv.Atoi(args[3])
				if err != nil || expiryMillis <= 0 {
					conn.Write([]byte("-ERR invalid expiry value\r\n"))
					return
				}
				if expiryMillis == 0 {
					expiry = -1
				} else {
					expiry = int(time.Now().UnixMilli()) + expiryMillis
				}

			}

			store.SetStore.Lock()
			store.items[args[0]] = StoreItem{
				Value:  args[1],
				Expiry: expiry,
			}
			store.SetStore.Unlock()

			conn.Write([]byte("+OK\r\n"))
		case "GET":
			store.SetStore.RLock()
			item, isExist := store.items[args[0]]
			if isExist {
				if item.Expiry == -1 {
					store.SetStore.RUnlock()
					conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(item.Value), item.Value)))
					continue
				}

				if item.Expiry > 0 && item.Expiry < int(time.Now().UnixMilli()) {
					store.SetStore.RUnlock()
					store.SetStore.Lock()
					delete(store.items, args[0])
					store.SetStore.Unlock()
					conn.Write([]byte("$-1\r\n"))
					continue
				}

				store.SetStore.RUnlock()
				conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(item.Value), item.Value)))
				continue
			}
			store.SetStore.RUnlock()

			conn.Write([]byte("$-1\r\n"))

		case "TYPE":
			_, isExist := store.items[args[0]]
			_, isStreamExist := store.StreamStore.entry[args[0]]

			if isStreamExist {
				conn.Write([]byte("+stream\r\n"))
				continue
			}
			if isExist {
				conn.Write([]byte("+string\r\n"))
				continue

			}

			conn.Write([]byte("+none\r\n"))

		case "XADD":
			if len(args) < 3 {
				conn.Write([]byte("-ERR wrong number of arguments for 'xadd' command\r\n"))
				continue
			}
			fields := args[2:]
			if len(fields)%2 != 0 {
				conn.Write([]byte("-ERR wrong number of fields for 'xadd' command\r\n"))
			}

			stream_name := args[0]
			entry_id := args[1]

			if entry_id != "*" {
				entryParts := strings.Split(entry_id, "-")
				if len(entryParts) != 2 {
					conn.Write([]byte("-ERR Invalid stream ID format\r\n"))
					continue
				}
				ms, err1 := strconv.ParseInt(entryParts[0], 10, 64)
				seq, err2 := strconv.ParseUint(entryParts[1], 10, 64)
				if err1 != nil || err2 != nil {
					conn.Write([]byte("-ERR Invalid stream ID numbers\r\n"))
					continue
				}

				store.StreamStore.mu.Lock()
				stream, exists := store.StreamStore.entry[stream_name]
				if !exists || len(stream.values) == 0 {
					if ms < 0 || (ms == 0 && seq == 0) {
						store.StreamStore.mu.Unlock()
						conn.Write([]byte("-ERR The ID specified in XADD must be greater than 0-0\r\n"))
						continue
					}
				} else {
					var lastMS int64
					var lastSeq uint64
					for id := range stream.values {
						parts := strings.Split(id, "-")
						if len(parts) != 2 {
							continue
						}
						cms, _ := strconv.ParseInt(parts[0], 10, 64)
						cseq, _ := strconv.ParseUint(parts[1], 10, 64)

						if cms > lastMS || (cms == lastMS && cseq > lastSeq) {
							lastMS = cms
							lastSeq = cseq
						}
					}
					if ms < lastMS || (ms == lastMS && seq <= lastSeq) {
						store.StreamStore.mu.Unlock()
						conn.Write([]byte("-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n"))
						continue
					}
				}
				store.StreamStore.mu.Unlock()
			}

			var pairs [][]string
			for i := 0; i < len(fields); i += 2 {
				pair := []string{fields[i], fields[i+1]}
				pairs = append(pairs, pair)
			}
			store.StreamStore.mu.Lock()
			if _, exists := store.StreamStore.entry[stream_name]; !exists {
				store.StreamStore.entry[stream_name] = Stream{
					id:     stream_name,
					values: make(map[string]string),
				}
			}
			store.StreamStore.entry[stream_name].values[entry_id] = strings.Join(fields, " ")
			store.StreamStore.mu.Unlock()

			conn.Write([]byte("+" + entry_id + "\r\n"))

		default:
			conn.Write([]byte("write a Valid command\r\n"))
		}

	}
}
