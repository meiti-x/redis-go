package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

var _ = net.Listen
var _ = os.Exit

func handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		msg := strings.TrimSpace(scanner.Text())
		if strings.ToUpper(msg) == "PING" {
			conn.Write([]byte("+PONG\r\n"))
		}

		echo_message := strings.Split(strings.ToLower(msg), "echo")[1]
		echo_message = strings.TrimSpace(echo_message) + "\r\n"
		conn.Write([]byte(echo_message))
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
