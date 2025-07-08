package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func parseCommand(line string) (string, []string, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", nil, errors.New("empty command")
	}

	parts := strings.Split(line, " ")
	if len(parts) == 0 {
		return "", nil, errors.New("invalid command format")
	}

	command := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	return command, args, nil
}

// Parse reads a RESP command from the provided bufio.Reader and returns the response as a string.
func Parse(r *bufio.Reader) (string, []string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", nil, err
	}

	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return "", nil, errors.New("empty command")
	}

	if line[0] == '*' {
		count, err := strconv.Atoi(line[1:])
		if err != nil {
			return "", nil, fmt.Errorf("invalid array count: %v", err)
		}

		args := make([]string, 0, count)
		for i := 0; i < count; i++ {

			bulkHeader, err := r.ReadString('\n')
			if err != nil {
				return "", nil, fmt.Errorf("reading bulk header: %v", err)
			}

			bulkHeader = strings.TrimSpace(bulkHeader)
			if len(bulkHeader) == 0 || bulkHeader[0] != '$' {
				return "", nil, errors.New("invalid bulk string header")
			}

			length, err := strconv.Atoi(bulkHeader[1:])
			if err != nil {
				return "", nil, fmt.Errorf("invalid bulk length: %v", err)
			}

			// Read the bulk string content
			bulkContent := make([]byte, length)
			_, err = io.ReadFull(r, bulkContent)
			if err != nil {
				return "", nil, fmt.Errorf("reading bulk content: %v", err)
			}

			// Read the trailing \r\n
			_, err = r.ReadString('\n')
			if err != nil {
				return "", nil, fmt.Errorf("reading bulk terminator: %v", err)
			}

			args = append(args, string(bulkContent))
			return args[0], args, nil

		}

		if len(args) == 0 {
			return "", nil, errors.New("empty command array")
		}

	}

	return parseCommand(line)
}
