package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dinkisstyle-chat/internal/codetestbed"
	"github.com/pkg/browser"
)

func main() {
	address := flag.String("addr", "127.0.0.1:31987", "loopback address for the testbed UI")
	noOpen := flag.Bool("no-open", false, "do not open the browser automatically")
	flag.Parse()

	host, _, err := net.SplitHostPort(*address)
	if err != nil || !isLoopback(host) {
		log.Fatal("-addr must be a loopback host and port, for example 127.0.0.1:31987")
	}
	server := &http.Server{
		Addr:              *address,
		Handler:           codetestbed.NewServer().Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-shutdown
		_ = server.Close()
	}()

	pageURL := "http://" + *address
	fmt.Printf("Local LLM Code Testbed: %s\n", pageURL)
	fmt.Println("Press Ctrl+C to stop.")
	if !*noOpen {
		go func() {
			time.Sleep(250 * time.Millisecond)
			_ = browser.OpenURL(pageURL)
		}()
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
