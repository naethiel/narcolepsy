package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
)

type Request struct {
	Key        string
	Definition *http.Request
}

const (
	DEFAULT_METHOD       = "GET"
	DEFAULT_PROTOCOL     = "HTTP/1.1"
	COMMENT_PREFIX_SLASH = "// "
	COMMENT_PREFIX_HASH  = "# "
	SEPARATOR_PREFIX     = "### "
)

var allowedMethods = []string{
	"GET",
	"HEAD",
	"POST",
	"PUT",
	"DELETE",
	"CONNECT",
	"PATCH",
	"OPTIONS",
	"TRACE",
}

func (s Service) getRequestsFromLines(lines []string) ([]Request, error) {
	rawDumps := getRawRequests(&s.env, lines)

	parsed := make([]Request, 0, len(rawDumps))

	for _, d := range rawDumps {
		s.logger.Debug("request dump found", "dump", d)
		// apply env variables to raw request string
		// before parsing it into an actual http Request object
		d = applyEnvVars(&s.env, d)

		// parse the request dump into an http request obj
		req, key, err := readRequest(d)
		s.logger.Debug("parsed request", "key", key, "req", req)

		if err != nil {
			return nil, fmt.Errorf("parsing request: %w, dump: %s", err, d)
		}

		parsed = append(parsed, Request{Key: key, Definition: req})
	}

	return parsed, nil
}

func getRawRequests(env *Environment, lines []string) []string {
	rawRequests := []string{}
	builder := strings.Builder{}

	for _, l := range lines {

		if isComment(l) {
			continue
		}

		if isSep(l) {
			// push previously built string in acc
			built := builder.String()
			built = strings.TrimSpace(built)
			if len(built) > 0 {
				rawRequests = append(rawRequests, built)
			}

			// reset builder
			builder.Reset()
		}

		builder.WriteString(l + "\n")
	}

	// commit last built request
	if builder.Len() > 0 {
		built := builder.String()
		built = strings.TrimSpace(built)
		rawRequests = append(rawRequests, built)
	}

	return rawRequests
}

type requestReader struct {
	err        error
	inputLines []string
	line       int
	name       string
	method     string
	uri        string
	proto      string
	headers    map[string]string
	body       strings.Builder
	state      string
}

const (
	STATE_START   = "START"
	STATE_HEADERS = "HEADERS"
	STATE_BODY    = "BODY"
)

func (r *requestReader) ReadLine() {
	if r.line >= len(r.inputLines) {
		r.err = io.EOF
		return
	}

	l := strings.TrimSpace(r.inputLines[r.line])

	switch r.state {
	case STATE_START:
		if len(l) == 0 || isComment(l) {
			r.line++
			return
		}

		if isSep(l) {
			r.name = strings.TrimPrefix(l, "### ")
			r.line++
			return
		}

		r.readRequestLine(l)
		if len(r.name) == 0 {
			r.name = l
		}
		r.state = STATE_HEADERS
		r.line++
	case STATE_HEADERS:
		if isComment(l) {
			r.line++
			return
		}

		if len(l) == 0 {
			r.line++
			r.state = STATE_BODY
			return
		}

		r.readHeaderLine(l)
		r.line++
	case STATE_BODY:
		if isComment(l) {
			r.line++
			return
		}

		r.body.WriteString(l)
		r.line++
	}
}

func (r *requestReader) readHeaderLine(l string) {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}

	key, value, ok := strings.Cut(l, ":")

	if !ok {
		r.err = fmt.Errorf("malformed header line: %s on line %d of request %s", l, r.line, r.name)
		return
	}

	r.headers[key] = value

	return

}

func (r *requestReader) readRequestLine(l string) {
	fields := strings.Fields(l)
	var method, uri, proto string

	//if f has len 2 or 1, we have missing method and/or proto
	switch len(fields) {
	case 3:
		method, uri, proto = fields[0], fields[1], fields[2]
	case 2:
		if slices.Contains(allowedMethods, fields[0]) {
			method, uri = fields[0], fields[1]
		} else {
			uri, proto = fields[0], fields[1]
		}
	case 1:
		uri = fields[0]
	default:
		r.err = fmt.Errorf("malformed request line: %s on line %d of request %s", l, r.line, r.name)
		return
	}

	// add default method and proto as necessary
	if len(method) == 0 {
		method = DEFAULT_METHOD
	}
	if len(proto) == 0 {
		proto = DEFAULT_PROTOCOL
	}

	r.method = method
	r.uri = uri
	r.proto = proto
}

func readRequest(raw string) (*http.Request, string, error) {
	s := strings.TrimSpace(raw)

	r := requestReader{
		inputLines: strings.Split(s, "\n"),
		line:       0,
		state:      STATE_START,
	}

	for {
		r.ReadLine()
		if r.err != nil {
			break
		}
	}

	// eof is not an error we want to fail on
	if r.err == io.EOF {
		r.err = nil
	}

	if r.err != nil {
		return nil, r.name, fmt.Errorf("reading request: %w", r.err)
	}

	req, err := http.NewRequest(r.method, r.uri, strings.NewReader(r.body.String()))
	if err != nil {
		return nil, r.name, fmt.Errorf("instantiating request: %w", err)
	}

	for key, val := range r.headers {
		req.Header.Add(key, val)
	}

	return req, r.name, nil
}

func applyEnvVars(env *Environment, s string) string {
	// if there is no replacement to do, just fast exit
	re := regexp.MustCompile(`\{\{(.*?)\}\}`)
	hasReplacements := re.MatchString(s)
	if !hasReplacements {
		return s
	}

	newStr := s

	// replace all keys in env with their respective values in current string
	for key, repl := range *env {
		pattern := "{{" + key + "}}"
		newStr = strings.ReplaceAll(newStr, pattern, repl)
	}

	return newStr
}

func isComment(line string) bool {
	return strings.HasPrefix(line, COMMENT_PREFIX_SLASH) || strings.HasPrefix(line, COMMENT_PREFIX_HASH)
}

func isSep(line string) bool {
	return strings.HasPrefix(line, SEPARATOR_PREFIX)
}
