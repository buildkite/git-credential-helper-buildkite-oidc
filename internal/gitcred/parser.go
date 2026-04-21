package gitcred

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Request struct {
	Protocol  string
	Authority string
	Path      string
}

func ParseRequest(reader io.Reader) (Request, error) {
	scanner := bufio.NewScanner(reader)
	request := Request{}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Request{}, fmt.Errorf("invalid credential line %q", line)
		}
		switch key {
		case "protocol":
			request.Protocol = value
		case "host":
			request.Authority = value
		case "path":
			request.Path = value
		}
	}

	if err := scanner.Err(); err != nil {
		return Request{}, err
	}

	return request, nil
}
