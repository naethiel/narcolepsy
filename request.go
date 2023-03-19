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

// readRequests reads from lines and returns a list of parsed Request objects
// It also applies env variables to request text, replacing env keys with their corresponding values
// before building each request.
func (s Service) readRequests() ([]Request, error) {
	lines, err := readLines(s.path)
	if err != nil {
		return nil, fmt.Errorf("reading lines from file: %w", err)
	}

	rawDumps := getRawRequests(s.env, lines)

	parsed := make([]Request, 0, len(rawDumps))

	for _, d := range rawDumps {
		s.logger.Debug("request dump found", "dump", d)
		// apply env variables to raw request string
		// before parsing it into an actual http Request object
		d = applyEnvVars(s.env, d)

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

// getRawRequests walks through lines and splits it into
// a slice of strings, each string being the content separated between two ### separator lines
// it does no other parsing
func getRawRequests(env Environment, lines []string) []string {
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

// requestReader is a struct that can be used to parse a raw http request string
// and build a raw request struct representation of it, to be used as parameters to build an actual http.Request for example
type requestReader struct {
	err        error
	inputLines []string
	line       int
	state      string
	output     struct {
		name    string
		method  string
		uri     string
		proto   string
		headers map[string]string
		body    strings.Builder
	}
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
	r.line++

	switch r.state {
	case STATE_START:
		if len(l) == 0 || isComment(l) {
			return
		}

		if isSep(l) {
			r.output.name = strings.TrimPrefix(l, "### ")
			return
		}

		r.readRequestLine(l)
		if len(r.output.name) == 0 {
			r.output.name = l
		}
		r.state = STATE_HEADERS
	case STATE_HEADERS:
		if isComment(l) {
			return
		}

		if len(l) == 0 {
			r.state = STATE_BODY
			return
		}

		r.readHeaderLine(l)
	case STATE_BODY:
		if isComment(l) {
			return
		}

		r.output.body.WriteString(l)
	}
	return
}

func (r *requestReader) back() {
	if r.line > 0 {
		r.line--
	}
}

func (r *requestReader) readHeaderLine(l string) {
	if r.output.headers == nil {
		r.output.headers = make(map[string]string)
	}

	key, value, ok := strings.Cut(l, ":")

	if !ok {
		r.back()
		r.err = fmt.Errorf("malformed header line: %s on line %d of request %s", l, r.line, r.output.name)
		return
	}

	r.output.headers[key] = value

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
		r.back()
		r.err = fmt.Errorf("malformed request line: %s on line %d of request %s", l, r.line, r.output.name)
		return
	}

	// add default method and proto as necessary
	if len(method) == 0 {
		method = DEFAULT_METHOD
	}
	if len(proto) == 0 {
		proto = DEFAULT_PROTOCOL
	}

	r.output.method = method
	r.output.uri = uri
	r.output.proto = proto
}

// readRequest takes a raw string containing an http request definition
// and builds a *http.Request matching the specs found in raw
// and returns it
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
		return nil, r.output.name, fmt.Errorf("reading request: %w", r.err)
	}

	req, err := http.NewRequest(r.output.method, r.output.uri, strings.NewReader(r.output.body.String()))
	if err != nil {
		return nil, r.output.name, fmt.Errorf("instantiating request: %w", err)
	}

	for key, val := range r.output.headers {
		req.Header.Add(key, val)
	}

	return req, r.output.name, nil
}

// applyEnvVars will read from env and replace all variables in s
// with the corresponding value found in env.
// In s, variables are defined by surrounding {{ }}
// It returns a new string with variables replaced by their values from env.
// If a variable has no corresponding key in env, it is ignored.
func applyEnvVars(env Environment, s string) string {
	// if there is no replacement to do, just fast exit
	re := regexp.MustCompile(`\{\{(.*?)\}\}`)
	hasReplacements := re.MatchString(s)
	if !hasReplacements {
		return s
	}

	newStr := s

	// replace all keys in env with their respective values in current string
	for key, repl := range env {
		pattern := "{{" + key + "}}"
		newStr = strings.ReplaceAll(newStr, pattern, repl)
	}

	return newStr
}

// isComment returns wether line is a comment line (defined by either a // or a # prefix)
func isComment(line string) bool {
	return strings.HasPrefix(line, COMMENT_PREFIX_SLASH) || strings.HasPrefix(line, COMMENT_PREFIX_HASH)
}

// isSep returns wether line is a separator line (defined by a ### prefix)
func isSep(line string) bool {
	return strings.HasPrefix(line, SEPARATOR_PREFIX)
}
