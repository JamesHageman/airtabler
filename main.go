package main

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
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
	http.HandleFunc("/", handler)

	addr := "127.0.0.1:8080"
	log.Println("Running on " + addr)
	log.Fatal(http.ListenAndServe(addr, nil))
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
	airtableReq := createProxiedRequest(req)
	client := &http.Client{}

	res, err := client.Do(airtableReq)
	if err != nil {
		http.Error(w, res.Status, res.StatusCode)
		log.Println(err)
		return
	}
	defer res.Body.Close()

	copyHeader(w.Header(), res.Header)

	log.Println(res.StatusCode, airtableReq.URL)
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
}
