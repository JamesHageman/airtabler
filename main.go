package main

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type apiRequest struct {
	req  *http.Request
	w    http.ResponseWriter
	done chan struct{}
}

const (
	requestsPerSecond = 5
)

var (
	apiKey  = os.Getenv("AIRTABLE_API_KEY")
	baseURL = "https://api.airtable.com/v0/appMDlUpKSJNcvCsm"
	// Hop-by-hop headers. These are removed when sent to the backend.
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
	hopHeaders = []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te", // canonicalized version of "TE"
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	apiRequests = make(chan *apiRequest, 5)
)

func init() {
	if apiKey == "" {
		log.Fatal("AIRTABLE_API_KEY missing")
	}
}

func die(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func main() {
	go apiRequestLoop()

	http.HandleFunc("/", handler)

	addr := "127.0.0.1:8080"
	log.Println("Running on " + addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func apiRequestLoop() {
	client := &http.Client{}
	ticker := time.Tick(time.Second / requestsPerSecond)

	for apiReq := range apiRequests {
		go handleAPIRequest(apiReq, client)
		<-ticker
	}
}

func handleAPIRequest(apiReq *apiRequest, client *http.Client) {
	w := apiReq.w
	res, err := client.Do(apiReq.req)
	if err != nil {
		http.Error(w, res.Status, res.StatusCode)
		log.Println(err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode == 429 {
		log.Println("Retrying ", apiReq.req.URL)
		time.Sleep(1 * time.Second)
		apiRequests <- apiReq
		return
	}

	copyHeader(w.Header(), res.Header)

	log.Println(res.StatusCode, apiReq.req.URL)
	io.Copy(w, res.Body)
	apiReq.done <- struct{}{}
}

func createProxiedRequest(req *http.Request) *http.Request {
	q := req.URL.Query()
	q.Add("api_key", apiKey)
	reqURL, _ := url.Parse(baseURL + req.URL.Path + "?" + q.Encode())

	return &http.Request{
		Method: req.Method,
		URL:    reqURL,
	}
}

func handler(w http.ResponseWriter, req *http.Request) {
	done := make(chan struct{})
	apiRequests <- &apiRequest{
		req:  createProxiedRequest(req),
		w:    w,
		done: done,
	}
	<-done
}
