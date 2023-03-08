package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"

	"github.com/alecthomas/chroma/quick"
)

func writeResponse(w io.Writer, res *http.Response) error {
	defer res.Body.Close()
	dump, err := httputil.DumpResponse(res, false)
	if err != nil {
		return fmt.Errorf("dumping response: %w", err)
	}

	rawBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}

	var body bytes.Buffer
	if res.Header.Get("Content-Type") == "application/json" {
		err = json.Indent(&body, rawBody, "", "\t")

		if err != nil {
			return fmt.Errorf("indenting json body: %w", err)
		}
	}

	err = quick.Highlight(w, string(dump), "HTTP", "terminal16m", "dracula")
	if err != nil {
		return fmt.Errorf("printing response: %w", err)
	}

	err = quick.Highlight(w, body.String(), "JSON", "terminal16m", "dracula")
	if err != nil {
		return fmt.Errorf("printing response to stdout: %w", err)
	}

	return nil
}
