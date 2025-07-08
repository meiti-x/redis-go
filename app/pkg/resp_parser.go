package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func Parse(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return "", errors.New("empty command")
	}

	switch line[0] {
	case '*': // Array
		count, err := strconv.Atoi(line[1:])
		if err != nil {
			return "", fmt.Errorf("invalid array count: %v", err)
		}

		args := make([]string, 0, count)
		for i := 0; i < count; i++ {
			// Read the bulk string header
			bulkHeader, err := r.ReadString('\n')
			if err != nil {
				return "", fmt.Errorf("reading bulk header: %v", err)
			}

			bulkHeader = strings.TrimSpace(bulkHeader)
			if len(bulkHeader) == 0 || bulkHeader[0] != '$' {
				return "", errors.New("invalid bulk string header")
			}

			length, err := strconv.Atoi(bulkHeader[1:])
			if err != nil {
				return "", fmt.Errorf("invalid bulk length: %v", err)
			}

			// Read the bulk string content
			bulkContent := make([]byte, length)
			_, err = io.ReadFull(r, bulkContent)
			if err != nil {
				return "", fmt.Errorf("reading bulk content: %v", err)
			}

			// Read the trailing \r\n
			_, err = r.ReadString('\n')
			if err != nil {
				return "", fmt.Errorf("reading bulk terminator: %v", err)
			}

			args = append(args, string(bulkContent))
		}

		if len(args) == 0 {
			return "", errors.New("empty command array")
		}

		command := strings.ToUpper(args[0])
		switch command {
		case "PING":
			return "+PONG\r\n", nil
		case "ECHO":
			if len(args) < 2 {
				return "", errors.New("ECHO requires an argument")
			}
			return fmt.Sprintf("$%d\r\n%s\r\n", len(args[1]), args[1]), nil
		default:
			return "", fmt.Errorf("unknown command: %s", command)
		}

	default:
		// Simple string or inline command (for backward compatibility)
		command := strings.ToUpper(line)
		switch {
		case command == "PING":
			return "+PONG\r\n", nil
		case strings.HasPrefix(command, "ECHO "):
			echoMsg := strings.TrimSpace(line[5:])
			return fmt.Sprintf("$%d\r\n%s\r\n", len(echoMsg), echoMsg), nil
		default:
			return "", fmt.Errorf("unknown command: %s", line)
		}
	}
}
