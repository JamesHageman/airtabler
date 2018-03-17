package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"
)

type Empty = struct{}

type apiRequest struct {
	req  *http.Request
	w    http.ResponseWriter
	done chan Empty
}

var (
	empty             = Empty{}
	requestsPerSecond uint64
	timeout           time.Duration
	apiKey            string
	baseURL           string
	addr              string
	apiRequests       chan apiRequest
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.StringVar(&addr, "addr", "127.0.0.1:8080", "airtabler address")
	baseID := flag.String("baseid", os.Getenv("AIRTABLE_BASE_ID"), "your airtable base id")
	flag.StringVar(&apiKey, "apikey", os.Getenv("AIRTABLE_API_KEY"), "your airtable api key")
	flag.Uint64Var(&requestsPerSecond, "rate", 5, "requests per second")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "airtable request timeout")
	flag.Parse()

	apiRequests = make(chan apiRequest)
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
	doneRequestLoop := make(chan Empty, 1)

	go func() {
		apiRequestLoop()
		doneRequestLoop <- empty
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	http.HandleFunc("/", handler)

	server := &http.Server{Addr: addr}

	go func() {
		log.Println("Running on " + addr)
		if err := server.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	signal := <-stop
	log.Printf("Received signal %s, shutting down the server...", signal)

	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatal("Failed to shutdown gracefully: ", err)
	}

	close(apiRequests)
	<-doneRequestLoop

	log.Println("Server gracefully stopped")
}

func apiRequestLoop() {
	client := &http.Client{Timeout: timeout}

	for apiReq := range apiRequests {
		go handleAPIRequest(apiReq, client)
		time.Sleep(time.Second / time.Duration(requestsPerSecond))
	}
}

func handleAPIRequest(apiReq apiRequest, client *http.Client) {
	defer func() { apiReq.done <- empty }()
	w := apiReq.w
	log.Println(apiReq.req.URL)
	res, err := client.Do(apiReq.req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		log.Println(err)
		return
	}
	defer res.Body.Close()

	copyHeader(w.Header(), res.Header)

	io.Copy(w, res.Body)
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
	done := make(chan Empty, 1)
	apiRequests <- apiRequest{
		req:  createProxiedRequest(req),
		w:    w,
		done: done,
	}
	<-done
}
