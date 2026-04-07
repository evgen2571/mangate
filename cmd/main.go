package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {

	// --------- do not touch UNDER NO CIRCUMSTANCES(actually you can but js why would you do that) ---------

	baseURL := "https://api.mangadex.org/manga"

	// --------- forming a request by constructing a link to suit our needs ---------

	title := "re zero"
	limit := "5"

	parameters := url.Values{}
	parameters.Set("title", title)
	parameters.Set("limit", limit)

	requestURL := baseURL + "?" + parameters.Encode()

	fmt.Println("\n FULL URL: ", requestURL, "\n")

	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	check(err)

	// --------- creating a client & then sending a GET response to it ---------

	client := &http.Client{Timeout: 10 * time.Second}

	response, err := client.Do(request)
	check(err)

	defer response.Body.Close() // just in case

	fmt.Println("RESULT RESPONSE: ", response, "\n")

	// --------- decoding the response ig(MUST BE FIXED OR DONE USING JSON DECODER) ---------

	body, err := io.ReadAll(response.Body) // <- temporary decision
	check(err)

	fmt.Println("RESULT: ", string(body), "\n")

}
