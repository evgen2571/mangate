package client

import (
	"io"
	"log"
	"net/http"
	"time"
)

var client = http.Client{
	Timeout: time.Second * 5,
}

func NewRequest(url string, body io.Reader) *http.Request {
	req, err := http.NewRequest(http.MethodGet, url, body)
	if err != nil {
		log.Fatal("Failed to create request")
	}

	return req
}

func DoRequest(req *http.Request) (*http.Response, error) {
	return client.Do(req)
}
