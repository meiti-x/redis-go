package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
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

type ConcurrentStore struct {
	sync.RWMutex
	items map[string]StoreItem
	stop  chan struct{}
}

func NewConcurrentStore() *ConcurrentStore {
	store := &ConcurrentStore{
		items: make(map[string]StoreItem),
		stop:  make(chan struct{}),
	}
	go store.startCleanupJob()
	return store
}

func (s *ConcurrentStore) Stop() {
	close(s.stop)
}

func (s *ConcurrentStore) startCleanupJob() {
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

func (s *ConcurrentStore) cleanupExpiredItems() {
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

func handleConnection(conn net.Conn) {
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

			store.Lock()
			store.items[args[0]] = StoreItem{
				Value:  args[1],
				Expiry: expiry,
			}
			store.Unlock()

			conn.Write([]byte("+OK\r\n"))
		case "GET":
			store.RLock()
			item, isExist := store.items[args[0]]
			if isExist {
				if item.Expiry == -1 {
					store.RUnlock()
					conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(item.Value), item.Value)))
					continue
				}

				if item.Expiry > 0 && item.Expiry < int(time.Now().UnixMilli()) {
					store.RUnlock()
					store.Lock()
					delete(store.items, args[0])
					store.Unlock()
					conn.Write([]byte("$-1\r\n"))
					continue
				}

				store.RUnlock()
				conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(item.Value), item.Value)))
				continue
			}
			store.RUnlock()

			conn.Write([]byte("$-1\r\n"))

		case "TYPE":
			_, isExist := store.items[args[0]]
			if isExist {
				conn.Write([]byte("+string\r\n"))
				continue

			}

			conn.Write([]byte("+none\r\n"))
		default:
			conn.Write([]byte("write a Valid command\r\n"))
		}

	}
}

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(conn)
	}
}
