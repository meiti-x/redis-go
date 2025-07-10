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
}

func NewConcurrentStore() *ConcurrentStore {
	return &ConcurrentStore{
		items: make(map[string]StoreItem),
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
					conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(item.Value), item.Value)))
					return
				}

				if item.Expiry > 0 && item.Expiry < int(time.Now().UnixMilli()) {
					delete(store.items, args[0])
					isExist = false
				}

			}

			store.RUnlock()

			if !isExist {
				conn.Write([]byte("null bulk string\r\n"))
				return
			}

			conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(item.Value), item.Value)))
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
