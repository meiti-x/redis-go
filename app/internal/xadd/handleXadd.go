package xadd

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	redisStore "github.com/codecrafters-io/redis-starter-go/app/pkg/store"
)

func ResolveStreamID(entryID string, lastID string) (string, int64, uint64, error) {
	var ms int64
	var seq uint64

	if entryID == "*" {
		now := time.Now().UnixMilli()
		if lastID != "" {
			lastParts := strings.Split(lastID, "-")
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
		return fmt.Sprintf("%d-%d", ms, seq), ms, seq, nil
	}

	parts := strings.Split(entryID, "-")
	if len(parts) != 2 {
		return "", 0, 0, fmt.Errorf("-ERR Invalid stream ID format\r\n")
	}

	msPart := parts[0]
	seqPart := parts[1]

	if msPart == "*" {
		ms = time.Now().UnixMilli()
	} else {
		var err error
		ms, err = strconv.ParseInt(msPart, 10, 64)
		if err != nil {
			return "", 0, 0, fmt.Errorf("-ERR Invalid milliseconds time in stream ID\r\n")
		}
	}

	if seqPart == "*" {
		if lastID != "" {
			lastParts := strings.Split(lastID, "-")
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
		var err error
		seq, err = strconv.ParseUint(seqPart, 10, 64)
		if err != nil {
			return "", 0, 0, fmt.Errorf("-ERR Invalid sequence number in stream ID\r\n")
		}
	}

	if ms == 0 && seq == 0 {
		return "", 0, 0, fmt.Errorf("-ERR The ID specified in XADD must be greater than 0-0\r\n")
	}

	return fmt.Sprintf("%d-%d", ms, seq), ms, seq, nil
}

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

	entryId, ms, seq, err := ResolveStreamID(entry_id, stream.LastID)
	if err != nil {
		conn.Write([]byte(err.Error()))
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

	// Build the field-value pairs
	for i := 0; i < len(fields); i += 2 {
		pair := []string{fields[i], fields[i+1]}
		stream.Values[entryId] = strings.Join(pair, " ")
	}

	stream.LastID = entryId
	store.StreamStore.Entry[stream_name] = stream
	store.StreamStore.Mu.Unlock()

	conn.Write([]byte("+" + entryId + "\r\n"))

}
