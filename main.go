package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/inconshreveable/log15"
	"github.com/urfave/cli/v2"
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

	// format lines by removing trailing whitespace
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}

	// join into a single string with line breaks
	text := strings.Join(lines, "\n")
	tokens, err := lex(text)
	if err != nil {
		log15.Error("lexing", "error", err)
		return fmt.Errorf("lexing file: %w", err)
	}

	log15.Debug("done lexing", "tokens", tokens)

	return nil
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
