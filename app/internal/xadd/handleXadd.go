package xadd

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	redisStore "github.com/codecrafters-io/redis-starter-go/app/pkg/store"
)

func HandleXadd(store *redisStore.Store, conn net.Conn, args []string) {
	if len(args) < 3 {
		conn.Write([]byte("-ERR wrong number of arguments for 'xadd' command\r\n"))
		return
	}
	fields := args[2:]
	if len(fields)%2 != 0 {
		conn.Write([]byte("-ERR wrong number of fields for 'xadd' command\r\n"))
		return
	}

	stream_name := args[0]
	entry_id := args[1]

	var ms int64
	var seq uint64

	store.StreamStore.Mu.Lock()
	stream, exists := store.StreamStore.Entry[stream_name]
	if !exists {
		stream = redisStore.Stream{
			Id:     stream_name,
			Values: make(map[string]string),
			LastID: "",
		}
	}

	if entry_id == "*" {
		now := time.Now().UnixMilli()
		if stream.LastID != "" {
			lastParts := strings.Split(stream.LastID, "-")
			lastMS, _ := strconv.ParseInt(lastParts[0], 10, 64)
			lastSeq, _ := strconv.ParseUint(lastParts[1], 10, 64)

			if now == lastMS {
				seq = lastSeq + 1
			} else {
				seq = 0
			}
		} else {
			seq = 0
		}
		ms = now
		entry_id = fmt.Sprintf("%d-%d", ms, seq)
	} else {
		entryParts := strings.Split(entry_id, "-")
		if len(entryParts) != 2 {
			store.StreamStore.Mu.Unlock()
			conn.Write([]byte("-ERR Invalid stream ID format\r\n"))
			return
		}

		msPart := entryParts[0]
		seqPart := entryParts[1]

		var err1, err2 error

		// Handle ms
		if msPart == "*" {
			ms = time.Now().UnixMilli()
		} else {
			ms, err1 = strconv.ParseInt(msPart, 10, 64)
			if err1 != nil {
				store.StreamStore.Mu.Unlock()
				conn.Write([]byte("-ERR Invalid milliseconds time in stream ID\r\n"))
				return
			}
		}

		// Handle seq
		if seqPart == "*" {
			// Auto-increment if same ms
			if stream.LastID != "" {
				lastParts := strings.Split(stream.LastID, "-")
				lastMS, _ := strconv.ParseInt(lastParts[0], 10, 64)
				lastSeq, _ := strconv.ParseUint(lastParts[1], 10, 64)

				if ms == lastMS {
					seq = lastSeq + 1
				} else {
					seq = 0
				}
			} else {
				seq = 0
			}
		} else {
			seq, err2 = strconv.ParseUint(seqPart, 10, 64)
			if err2 != nil {
				store.StreamStore.Mu.Unlock()
				conn.Write([]byte("-ERR Invalid sequence number in stream ID\r\n"))
				return
			}
		}

		// Rebuild final entry_id after resolving "*"
		entry_id = fmt.Sprintf("%d-%d", ms, seq)

		// Validate > 0-0
		if ms == 0 && seq == 0 {
			store.StreamStore.Mu.Unlock()
			conn.Write([]byte("-ERR The ID specified in XADD must be greater than 0-0\r\n"))
			return
		}

		// Validate strictly increasing
		if stream.LastID != "" {
			lastParts := strings.Split(stream.LastID, "-")
			lastMS, _ := strconv.ParseInt(lastParts[0], 10, 64)
			lastSeq, _ := strconv.ParseUint(lastParts[1], 10, 64)

			if ms < lastMS || (ms == lastMS && seq <= lastSeq) {
				store.StreamStore.Mu.Unlock()
				conn.Write([]byte("-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n"))
				return
			}
		}

	}

	// Build the field-value pairs
	for i := 0; i < len(fields); i += 2 {
		pair := []string{fields[i], fields[i+1]}
		stream.Values[entry_id] = strings.Join(pair, " ")
	}

	stream.LastID = entry_id
	store.StreamStore.Entry[stream_name] = stream
	store.StreamStore.Mu.Unlock()

	conn.Write([]byte("+" + entry_id + "\r\n"))

}
