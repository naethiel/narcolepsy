package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/AlecAivazis/survey/v2"
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

func initLogger(ctx *cli.Context) {
	// set log level properly
	verbose := ctx.Bool("verbose")
	var h log15.Handler
	if verbose {
		h = log15.LvlFilterHandler(log15.LvlDebug, log15.StdoutHandler)
	} else {
		h = log15.LvlFilterHandler(log15.LvlInfo, log15.StdoutHandler)
	}
	log15.Root().SetHandler(h)
}

func fetch(ctx *cli.Context) error {
	args := ctx.Args()
	path := args.First()
	// if no path arg is provided, try to read the -config flag
	if len(path) == 0 {
		path = ctx.String("file")
	}

	initLogger(ctx)

	log15.Debug("reading file", "file", path)

	lines, err := readLines(path)
	if err != nil {
		log15.Error("Reading lines from file", "file", path, "err", err)
		return fmt.Errorf("reading lines from file: %w", err)
	}

	log15.Debug("lines read", "lines", lines)

	requests, keys, err := getRequestsFromLines(lines)

	var answer string
	err = survey.AskOne(&survey.Select{
		Message: "Choose a request",
		Options: keys,
	}, &answer)

	log15.Debug("chosen request", "req", answer)

	var request *http.Request
	for _, r := range requests {
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

	formatResponse(res)
	return nil
}
