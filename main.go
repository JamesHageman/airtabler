package main

import (
	"flag"
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

var (
	requestsPerSecond uint64
	timeout           time.Duration
	apiKey            string
	baseURL           string
	apiRequests       chan *apiRequest
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	baseID := flag.String("baseid", os.Getenv("AIRTABLE_BASE_ID"), "your airtable base id")
	flag.StringVar(&apiKey, "apikey", os.Getenv("AIRTABLE_API_KEY"), "your airtable api key")
	flag.Uint64Var(&requestsPerSecond, "rate", 5, "requests per second")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "airtable request timeout")
	flag.Parse()

	apiRequests = make(chan *apiRequest, requestsPerSecond*2)
	baseURL = "https://api.airtable.com/v0/" + *baseID

	if apiKey == "" {
		log.Fatal("api key missing")
	}

	if *baseID == "" {
		log.Fatal("Base id missing")
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
	client := &http.Client{Timeout: timeout}

	for {
		timer := time.After(1 * time.Second)
		for i := uint64(0); i < requestsPerSecond; i++ {
			apiReq := <-apiRequests
			go handleAPIRequest(apiReq, client)
		}
		<-timer
	}
}

func handleAPIRequest(apiReq *apiRequest, client *http.Client) {
	w := apiReq.w
	log.Println(apiReq.req.URL)
	res, err := client.Do(apiReq.req)
	if err != nil {
		http.Error(w, res.Status, res.StatusCode)
		log.Println(err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode == 429 {
		log.Println("429 - Retrying ", apiReq.req.URL)
		time.Sleep(1 * time.Second)
		apiRequests <- apiReq
		return
	}

	copyHeader(w.Header(), res.Header)

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
