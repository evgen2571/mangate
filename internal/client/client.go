package client

import (
	"io"
	"log"
	"net/http"
	"time"
)

var Client = http.Client{
	Timeout: 30 * time.Second,
}

func NewRequest(url string, body io.Reader) *http.Request {
	req, err := http.NewRequest(http.MethodGet, url, body)
	if err != nil {
		log.Fatal("failed to create request")
	}

	return req
}

func DoRequest(req *http.Request) (*http.Response, error) {
	return Client.Do(req)
}
