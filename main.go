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
	"strings"
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
	apiRequests       chan *apiRequest
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.StringVar(&addr, "addr", "127.0.0.1:8080", "airtabler address")
	baseID := flag.String("baseid", os.Getenv("AIRTABLE_BASE_ID"), "your airtable base id")
	flag.StringVar(&apiKey, "apikey", os.Getenv("AIRTABLE_API_KEY"), "your airtable api key")
	flag.Uint64Var(&requestsPerSecond, "rate", 5, "requests per second")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "airtable request timeout")
	flag.Parse()

	apiRequests = make(chan *apiRequest)
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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	http.HandleFunc("/", handler)

	server := &http.Server{Addr: addr}

	go func() {
		log.Println("Running on " + addr)
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Println("Server closed, waiting for requests to finish")
			} else {
				die(err)
			}
		}
	}()

	signal := <-stop
	log.Printf("Received signal %s, shutting down the server...", signal)
	server.Shutdown(context.Background())
	log.Println("Server gracefully stopped")
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

func proxiedURL(incomingURL *url.URL) *url.URL {
	var builder strings.Builder

	builder.WriteString(baseURL)
	builder.WriteString(incomingURL.Path)

	query := incomingURL.Query().Encode()
	if query != "" {
		builder.WriteString("?")
		builder.WriteString(query)
	}

	reqURL, _ := url.Parse(builder.String())

	return reqURL
}

func proxiedHeader(incomingHeaders http.Header) (ret http.Header) {
	ret = make(http.Header)
	copyHeader(ret, incomingHeaders)
	ret.Add("Authorization", "Bearer "+apiKey)
	return ret
}

func createProxiedRequest(req *http.Request) *http.Request {
	return &http.Request{
		Method: req.Method,
		URL:    proxiedURL(req.URL),
		Header: proxiedHeader(req.Header),
	}
}

func handler(w http.ResponseWriter, req *http.Request) {
	done := make(chan Empty, 1)
	apiRequests <- &apiRequest{
		req:  createProxiedRequest(req),
		w:    w,
		done: done,
	}
	<-done
}
