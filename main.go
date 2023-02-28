package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/inconshreveable/log15"
	"github.com/urfave/cli/v2"
	"golang.org/x/exp/slices"
)

func main() {
	app := &cli.App{
		Name:   "Owl HTTP rest client",
		Usage:  "Send HTTP requests using .http or .rest files",
		Action: fetch,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "Set log level to 'DEBUG'",
				Value:   false,
				Aliases: []string{"v"},
			},
			&cli.StringFlag{
				Name:    "file",
				Usage:   "Specify path to http request file",
				Aliases: []string{"f"},
			},
		},
	}

	err := app.Run(os.Args)

	if err != nil {
		log.Fatal("could not run app")
	}
}

func fetch(ctx *cli.Context) error {
	args := ctx.Args()
	path := args.First()
	// if no path arg is provided, try to read the -config flag
	if len(path) == 0 {
		path = ctx.String("file")
	}

	// set log level properly
	verbose := ctx.Bool("verbose")
	var h log15.Handler
	if verbose {
		h = log15.LvlFilterHandler(log15.LvlDebug, log15.StdoutHandler)
	} else {
		h = log15.LvlFilterHandler(log15.LvlInfo, log15.StdoutHandler)
	}
	log15.Root().SetHandler(h)

	log15.Debug("reading file", "file", path)

	lines, err := readLines(path)
	if err != nil {
		return fmt.Errorf("reading lines from file: %w", err)
	}

	log15.Debug("lines read", "lines", lines)

	rawRequests := getRawRequests(lines)
	log15.Debug("raw requests parsed", "reqs", rawRequests, "len", len(rawRequests))

	var (
		parsed []parsedRequest
		keys   []string
	)

	// now parse each raw request to build parsed requests ready to shoot
	for _, r := range rawRequests {
		log15.Debug("parsing request", "key", r.Key)
		p, err := parseRequestPayload(r)

		if err != nil {
			log15.Error("failed parsing request", "key", r.Key, "err", err)
			return fmt.Errorf("parsing request %s: %w", r.Key, err)
		}

		parsed = append(parsed, p)
		keys = append(keys, p.Key)
	}

	var answer string
	err = survey.AskOne(&survey.Select{
		Message: "Choose a request",
		Options: keys,
	}, &answer)

	log15.Debug("chosen request", "req", answer)

	var request *http.Request
	for _, r := range parsed {
		if r.Key == answer {
			request = r.Definition
			break
		}
	}

	res, err := http.DefaultClient.Do(request.Clone(ctx.Context))
	if err != nil {
		log15.Error("sending request", "key", answer, "request", request, "err", err)
		return err
	}

	fmt.Printf("request status: %s", res.Status)

	return nil
}

func parseRequestPayload(raw rawRequest) (parsedRequest, error) {
	content := bufio.NewReader(strings.NewReader(raw.Definition))
	var err error
	out := parsedRequest{
		Key: raw.Key,
	}
	out.Definition, err = http.ReadRequest(content)
	// unset RequestURI since it should not be set for outgoing requests
	out.Definition.RequestURI = ""

	return out, err
}

func readLines(filePath string) ([]string, error) {
	// import file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading from file: %w", err)
	}

	defer file.Close()

	lines := []string{}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}

	return lines, nil
}

type rawRequest struct {
	Key        string
	Definition string
}

type parsedRequest struct {
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

func getRawRequests(lines []string) []rawRequest {
	var (
		builtRequests []rawRequest
		current       rawRequest
	)

	state := "START"

	for i := 0; i < len(lines); {

		rawLine := lines[i]
		l := strings.TrimSpace(rawLine)

		switch state {
		case "START":
			if len(l) == 0 || isComment(l) {
				// skip blank lines
				// skip comment lines
				i++
				continue
			}
			if isSep(l) {
				state = "SEPARATOR"
			}
		case "SEPARATOR":
			state = "BEFORE_REQUEST"
			current = rawRequest{}
			current.Key = strings.TrimPrefix(l, SEPARATOR_PREFIX)
			i++
		case "BEFORE_REQUEST":
			// as many blank lines and comments after a separator and before a request
			if len(l) == 0 || isComment(l) {
				i++
				continue
			}
			state = "REQUEST"
		case "REQUEST":
			if isSep(l) {
				// commit current then move on
				builtRequests = append(builtRequests, current)
				state = "SEPARATOR"
				continue
			}

			// skip
			if isComment(l) {
				i++
				continue
			}

			// add current
			current.Definition += rawLine + "\n"
			i++
		}
	}

	// commit last built request
	if len(current.Definition) > 0 {
		builtRequests = append(builtRequests, current)
	}

	// sanitize trailing space on each request definition
	for i, r := range builtRequests {
		builtRequests[i].Definition = formatRequestDefinition(r.Definition)
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
	case 2:
		if slices.Contains(allowedMethods, f[0]) {
			method, uri = f[0], f[1]
		}
	case 1:
		uri = f[0]
	case 3:
		method, uri, proto = f[0], f[1], f[2]
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

func isComment(line string) bool {
	return strings.HasPrefix(line, COMMENT_PREFIX_SLASH) || strings.HasPrefix(line, COMMENT_PREFIX_HASH)
}

func isSep(line string) bool {
	return strings.HasPrefix(line, SEPARATOR_PREFIX)
}

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
