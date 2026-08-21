package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func main() {
	listen := flag.String("listen", ":443", "HTTPS listen address")
	targetValue := flag.String("target", "", "fixed HTTP upstream origin")
	certificate := flag.String("cert", "", "TLS certificate file")
	privateKey := flag.String("key", "", "TLS private key file")
	flag.Parse()

	target, err := url.Parse(*targetValue)
	if err != nil || target.Scheme != "http" || target.Host == "" || target.Path != "" || target.RawQuery != "" || target.Fragment != "" || target.User != nil {
		log.Fatal("target must be a fixed HTTP origin")
	}
	if *certificate == "" || *privateKey == "" {
		log.Fatal("cert and key are required")
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorLog = log.Default()
	proxy.ErrorHandler = func(response http.ResponseWriter, _ *http.Request, err error) {
		log.Printf("upstream request failed: %v", err)
		http.Error(response, "upstream unavailable", http.StatusBadGateway)
	}

	server := &http.Server{
		Addr:              *listen,
		Handler:           proxy,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    32 << 10,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
			NextProtos: []string{"http/1.1"},
		},
	}
	log.Fatal(server.ListenAndServeTLS(*certificate, *privateKey))
}
