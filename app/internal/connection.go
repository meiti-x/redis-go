package internal

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/app/internal/xadd"
	resp "github.com/codecrafters-io/redis-starter-go/app/pkg"
	redisStore "github.com/codecrafters-io/redis-starter-go/app/pkg/store"
)

func HandleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	store := redisStore.NewRedisStore()

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
			store.Items[args[0]] = redisStore.StoreItem{
				Value:  args[1],
				Expiry: expiry,
			}
			store.SetStore.Unlock()

			conn.Write([]byte("+OK\r\n"))
		case "GET":
			store.SetStore.RLock()
			item, isExist := store.Items[args[0]]
			if isExist {
				if item.Expiry == -1 {
					store.SetStore.RUnlock()
					conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(item.Value), item.Value)))
					continue
				}

				if item.Expiry > 0 && item.Expiry < int(time.Now().UnixMilli()) {
					store.SetStore.RUnlock()
					store.SetStore.Lock()
					delete(store.Items, args[0])
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
			_, isExist := store.Items[args[0]]
			_, isStreamExist := store.StreamStore.Entry[args[0]]

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
			xadd.HandleXadd(store, conn, args)
		case "XRANGE":
			if len(args) < 3 {
				conn.Write([]byte("-ERR wrong number of arguments for 'xrange' command\r\n"))
				return
			}

			streamName := args[0]
			startID := args[1]
			endID := args[2]

			results, err := store.StreamStore.GetRange(streamName, startID, endID)
			if err != nil {
				conn.Write([]byte("-ERR " + err.Error() + "\r\n"))
				return
			}

			if len(results) == 0 {
				conn.Write([]byte("*0\r\n"))
				return
			}

			conn.Write([]byte(fmt.Sprintf("*%d\r\n", len(results))))

			store.StreamStore.WriteStreamItems(conn, results)

		case "XREAD":
			if len(args) < 3 {
				conn.Write([]byte("-ERR wrong number of arguments for 'xread' command\r\n"))
				return
			}

			isStream := strings.ToUpper(args[0]) == "STREAMS"
			stream_name := args[1]
			entry_id := args[2]

			if !isStream {
				conn.Write([]byte("-ERR missing 'STREAMS' keyword\r\n"))
				return
			}

			streamMap, exists := store.StreamStore.Entry[stream_name]
			if !exists {
				conn.Write([]byte("-ERR stream does not exist\r\n"))
				return
			}

			streamValue, isExist := streamMap.Values[entry_id]
			if !isExist {
				conn.Write([]byte("-ERR entry does not exist\r\n"))
				return
			}

			entryStr := fmt.Sprintf("%s: %s", entry_id, streamValue)
			store.StreamStore.WriteStreamItems(conn, []string{entryStr})
		default:
			conn.Write([]byte("write a Valid command\r\n"))
		}

	}
}
