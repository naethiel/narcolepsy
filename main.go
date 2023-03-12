package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/inconshreveable/log15"
	"github.com/urfave/cli/v2"
)

type Service struct {
	logger        log15.Logger
	configuration Configuration
	env           Environment
	filePath      string
	request       string
}

func main() {

	var s Service

	app := &cli.App{
		Name:  "Narcolepsy HTTP rest client",
		Usage: "Send HTTP requests using .http or .rest files",
		Action: func(ctx *cli.Context) error {
			return s.Fetch(ctx)
		},
		Authors: []*cli.Author{
			{
				Name:  "Nicolas Missika",
				Email: "n.missika@outlook.com",
			},
		},
		UsageText: "narc [options] [path/to/file]",
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
			&cli.StringFlag{
				Name:    "environment",
				Value:   "default",
				Usage:   "set the working environment",
				Aliases: []string{"e"},
			},
			&cli.StringFlag{
				Name:    "configuration",
				Value:   "narcolepsy.json",
				Aliases: []string{"c"},
				Usage:   "path to the configuration file",
			},
			&cli.StringFlag{
				Name:    "request",
				Value:   "",
				Aliases: []string{"r"},
				Usage:   "name (without \"###\") of the request to run in the given file",
			},
		},
	}

	err := app.Run(os.Args)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (s *Service) bootstrap(ctx *cli.Context) error {
	args := ctx.Args()
	filePath := args.First()
	// if no path arg is provided, try to read the -config flag
	if len(filePath) == 0 {
		filePath = ctx.String("file")
	}

	// set log level properly
	verbose := ctx.Bool("verbose")

	logger := log15.New()
	var h log15.Handler

	if verbose {
		h = log15.LvlFilterHandler(log15.LvlDebug, log15.StdoutHandler)
	} else {
		h = log15.LvlFilterHandler(log15.LvlInfo, log15.StdoutHandler)
	}

	logger.SetHandler(h)

	// read configuration
	cfg, err := LoadConfiguration(ctx.String("configuration"))
	if err != nil {
		return fmt.Errorf("reading configuration: %w", err)
	}

	env, err := cfg.Env(ctx.String("environment"))
	logger.Debug("environment set", "env", env)
	if err != nil {
		return fmt.Errorf("reading environment: %w", err)
	}

	// assign newly configured service to global var
	s.logger = logger
	s.configuration = cfg
	s.filePath = filePath
	s.request = ctx.String("request")
	s.env = env

	return nil
}

func (s *Service) Fetch(ctx *cli.Context) error {
	err := s.bootstrap(ctx)
	if err != nil {
		return fmt.Errorf("initializing service: %w", err)
	}

	lines, err := readLines(s.filePath)
	if err != nil {
		return fmt.Errorf("reading lines from file: %w", err)
	}

	requests, err := s.getRequestsFromLines(lines)
	if err != nil {
		return err
	}

	request, err := s.selectRequest(requests)
	if err != nil {
		return fmt.Errorf("selecting request to send: %w", err)
	}

	res, err := http.DefaultClient.Do(request.Clone(ctx.Context))
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}

	err = writeResponse(os.Stdout, res)
	if err != nil {
		return fmt.Errorf("writing response to stdout: %w", err)
	}

	s.logger.Debug("Successfully output request. Done")
	return nil
}

func (s Service) selectRequest(list []Request) (*http.Request, error) {
	if len(list) == 1 {
		return list[0].Definition, nil
	}

	var req *http.Request

	if s.request == "" {
		opts := make([]string, 0, len(list))
		for _, r := range list {
			opts = append(opts, r.Key)
		}
		err := survey.AskOne(&survey.Select{
			Message: "Select request",
			Options: opts,
		}, &s.request)

		if err != nil {
			return nil, fmt.Errorf("selecting request interactively: %w", err)
		}
		s.logger.Debug("chosen request", "req", s.request)
	}

	// pick the request targetted by s.request
	for _, r := range list {
		if r.Key == s.request {
			req = r.Definition
			break
		}
	}
	if req == nil {
		return nil, fmt.Errorf("no request defined for given string: %s", s.request)
	}

	return req, nil
}
