package main

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var (
	apiKey  = os.Getenv("AIRTABLE_API_KEY")
	baseUrl = "https://api.airtable.com/v0/appMDlUpKSJNcvCsm"
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

func deleteHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

func main() {
	http.HandleFunc("/", handler)

	addr := "127.0.0.1:8080"
	log.Println("Running on " + addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handler(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	q.Add("api_key", apiKey)
	reqURL, _ := url.Parse(baseUrl + req.URL.Path + "?" + q.Encode())

	req.URL = reqURL
	req.RequestURI = ""

	deleteHopHeaders(req.Header)

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		http.Error(w, res.Status, res.StatusCode)
		log.Println(err)
		return
	}
	defer res.Body.Close()

	deleteHopHeaders(res.Header)
	copyHeader(w.Header(), res.Header)

	log.Println(res.StatusCode, req.URL)
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
}
