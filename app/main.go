package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	resp "github.com/codecrafters-io/redis-starter-go/app/pkg"
)

var store = struct {
	sync.RWMutex
	m map[string]string
}{m: make(map[string]string)}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

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
			// Allows other readers not any write
			store.Lock()
			store.m[args[0]] = args[1]
			store.Unlock()

			conn.Write([]byte("+OK\r\n"))
		case "GET":
			store.RLock()
			value, isExist := store.m[args[0]]
			store.RUnlock()

			if !isExist {
				conn.Write([]byte("null bulk string\r\n"))
				return
			}
			conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)))
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
