// container-healthcheck probes a service on loopback from its distroless image.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: healthcheck ENV_VAR DEFAULT_PORT PATH")
		os.Exit(2)
	}
	port := os.Getenv(os.Args[1])
	if port == "" {
		port = os.Args[2]
	}
	if strings.Contains(port, ":") {
		_, resolvedPort, err := net.SplitHostPort(port)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		port = resolvedPort
	}
	client := http.Client{Timeout: 3 * time.Second}
	res, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + os.Args[3])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, res.Status)
		os.Exit(1)
	}
}
