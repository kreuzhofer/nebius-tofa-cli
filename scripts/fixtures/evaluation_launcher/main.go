// A test executable using the real launcher and adapter with synthetic credentials.
// The production CLI deliberately has no endpoint override.
package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("tofa evaluation-fixture")
		return
	}
	endpoint := os.Getenv("FIXTURE_ENDPOINT")
	target, err := url.Parse(endpoint)
	if err != nil || target.Scheme != "http" || target.Hostname() != "127.0.0.1" || target.Port() == "" || target.Path != "" || target.RawQuery != "" || target.User != nil || target.Fragment != "" {
		panic("fixture requires a loopback provider")
	}
	if err := run(endpoint); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(endpoint string) error {
	dir, err := os.MkdirTemp("", "tofa-synthetic-store-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	store := tofa.Store{Dir: dir}
	if err := store.Login("synthetic-project", "synthetic-fixture-token", "file"); err != nil {
		return err
	}
	app := tofa.App{Dir: dir, Endpoint: endpoint, HTTP: &http.Client{Transport: &http.Transport{Proxy: nil}}, Version: "evaluation-fixture"}
	return app.Run(os.Args[1:])
}
