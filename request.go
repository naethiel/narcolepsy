package main

import (
	"bufio"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
)

type RequestDump struct {
	Key   string
	Value string
}

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

func (s Service) getRequestsFromLines(lines []string) ([]Request, []string, error) {
	rawDumps := getReqDumps(&s.env, lines)
	var (
		parsed []Request
		keys   []string
	)

	for _, d := range rawDumps {
		p, err := parseRequest(d)
		s.logger.Debug("parsed request", "key", p.Key, "def", p.Definition)

		if err != nil {
			return nil, nil, fmt.Errorf("parsing request %s: %w", d.Key, err)
		}

		parsed = append(parsed, p)
		keys = append(keys, p.Key)
	}

	return parsed, keys, nil
}

func getReqDumps(env *Environment, lines []string) []RequestDump {
	const (
		STATE_START          = "START"
		STATE_SEPARATOR      = "SEPARATOR"
		STATE_BEFORE_REQUEST = "BEFORE_REQUEST"
		STATE_REQUEST        = "REQUEST"
	)

	var (
		builtRequests []RequestDump
		current       RequestDump
	)

	state := STATE_START

	for i := 0; i < len(lines); {
		rawLine := lines[i]
		l := strings.TrimSpace(rawLine)
		l = applyEnvVars(env, l)

		switch state {
		case STATE_START:
			if len(l) == 0 || isComment(l) {
				// skip blank lines
				// skip comment lines
				i++
				continue
			}
			if isSep(l) {
				state = STATE_SEPARATOR
			} else {
				current = RequestDump{
					Key: l}
				state = STATE_REQUEST
			}
		case STATE_SEPARATOR:
			state = STATE_BEFORE_REQUEST
			current = RequestDump{}
			current.Key = strings.TrimPrefix(l, SEPARATOR_PREFIX)
			i++
		case STATE_BEFORE_REQUEST:
			// as many blank lines and comments after a separator and before a request
			if len(l) == 0 || isComment(l) {
				i++
				continue
			}
			state = STATE_REQUEST
		case STATE_REQUEST:
			if isSep(l) {
				// commit current then move on
				builtRequests = append(builtRequests, current)
				state = STATE_SEPARATOR
				continue
			}

			// skip
			if isComment(l) {
				i++
				continue
			}

			// add current
			current.Value += rawLine + "\n"
			i++
		}
	}

	// commit last built request
	if len(current.Value) > 0 {
		builtRequests = append(builtRequests, current)
	}

	// sanitize trailing space on each request definition
	for i, r := range builtRequests {
		builtRequests[i].Value = formatRequestDefinition(r.Value)
	}

	return builtRequests
}

func formatRequestDefinition(raw string) string {
	s := strings.TrimSpace(raw)

	requestLine, rest, _ := strings.Cut(s, "\n")

	f := strings.Fields(requestLine)
	var method, uri, proto string

	//if f has len 2 or 1, we have missing method and/or proto
	switch len(f) {
	case 3:
		method, uri, proto = f[0], f[1], f[2]
	case 2:
		if slices.Contains(allowedMethods, f[0]) {
			method, uri = f[0], f[1]
		} else {
			uri, proto = f[0], f[1]
		}
	case 1:
		uri = f[0]
	}

	// add default method and proto as necessary
	if len(method) == 0 {
		method = DEFAULT_METHOD
	}
	if len(proto) == 0 {
		proto = DEFAULT_PROTOCOL
	}

	newReqLine := fmt.Sprintf("%s %s %s", method, uri, proto)

	newDef := fmt.Sprintf("%s\n%s\n", newReqLine, rest)

	return newDef
}

func parseRequest(raw RequestDump) (Request, error) {
	content := bufio.NewReader(strings.NewReader(raw.Value))
	var err error
	out := Request{
		Key:        raw.Key,
		Definition: nil,
	}

	out.Definition, err = http.ReadRequest(content)
	if err != nil {
		return Request{}, err
	}
	// unset RequestURI since it should not be set for outgoing requests
	out.Definition.RequestURI = ""

	return out, err
}

func applyEnvVars(env *Environment, s string) string {
	// if there is no replacement to do, just fast exit
	re := regexp.MustCompile(`\{(.*?)\}`)
	hasReplacements := re.MatchString(s)
	if !hasReplacements {
		return s
	}

	newStr := s

	// replace all keys in env with their respective values in current string
	for key, repl := range *env {
		pattern := "{" + key + "}"
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
