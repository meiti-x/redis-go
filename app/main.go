package main

import (
	"bufio"
	"fmt"
	"net"
	"os"

	resp "github.com/codecrafters-io/redis-starter-go/app/pkg"
)

func handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		// Read the raw command from the connection
		response, err := resp.Parse(reader)
		if err != nil {
			conn.Write([]byte("-ERR " + err.Error() + "\r\n"))
			return
		}

		// Write the response back to the client
		_, err = conn.Write([]byte(response))
		if err != nil {
			return
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
