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
		p, err := parseRequest(r)

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
	return nil
}

func parseRequest(raw rawRequest) (parsedRequest, error) {
	content := bufio.NewReader(strings.NewReader(raw.Definition))
	var err error
	out := parsedRequest{
		Key: raw.Key,
	}
	out.Definition, err = http.ReadRequest(content)

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

func getRawRequests(lines []string) []rawRequest {
	var (
		builtRequests []rawRequest
		current       rawRequest
	)

	for i := 0; i < len(lines); i++ {
		l := lines[i]

		if strings.HasPrefix(l, "// ") || strings.HasPrefix(l, "# ") {
			// skip comment lines
			continue
		}

		if strings.HasPrefix(l, "### ") {
			// separator line means new request

			// append previously built request to accumulated requests if there is one
			if len(current.Definition) != 0 {
				builtRequests = append(builtRequests, current)
			}

			// spawn a clean new one
			current = rawRequest{}
			current.Key = strings.TrimPrefix(l, "### ")
			continue
		}

		current.Definition += l + "\n"
	}

	// push last built request in list
	if len(current.Definition) != 0 {
		builtRequests = append(builtRequests, current)
	}

	// sanitize trailing space on each request definition
	for i, r := range builtRequests {
		builtRequests[i].Definition = formatRawReq(r.Definition)
	}

	return builtRequests
}

func formatRawReq(raw string) string {
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

	if len(method) == 0 {
		method = "GET"
	}
	if len(proto) == 0 {
		proto = "HTTP/1.1"
	}

	newReqLine := fmt.Sprintf("%s %s %s", method, uri, proto)

	newDef := fmt.Sprintf("%s\n%s\n", newReqLine, rest)

	return newDef
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