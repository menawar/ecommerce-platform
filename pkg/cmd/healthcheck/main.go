// Command healthcheck is a minimal, statically-linked binary that replaces
// wget/curl in scratch containers. Docker Compose healthchecks need a process
// inside the container that can hit the /healthz endpoint; scratch has no shell
// and no wget, so we compile our own.
//
// Usage (in compose):  test: ["CMD", "/healthcheck", "http://localhost:2112/healthz"]
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: healthcheck <url>")
		os.Exit(1)
	}

	// Short timeout — healthchecks should be fast. If the service can't
	// respond in 3 seconds it's unhealthy.
	client := &http.Client{Timeout: 3 * time.Second}

	resp, err := client.Get(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unhealthy: status %d\n", resp.StatusCode)
		os.Exit(1)
	}
}
