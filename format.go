package main

import (
	"fmt"
	"net/http"
)

func formatResponse(res *http.Response) {
	fmt.Printf("request status: %s", res.Status)
}
