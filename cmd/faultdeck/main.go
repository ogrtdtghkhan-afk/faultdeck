package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ogrtdtghkhan-afk/faultdeck/internal/deck"
	"github.com/ogrtdtghkhan-afk/faultdeck/internal/web"
)

var version = deck.Version

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FaultDeck:", err)
		os.Exit(1)
	}
}

func run() error {
	proxyPort := flag.Int("port", 7332, "Loopback proxy port")
	uiPort := flag.Int("ui-port", 7331, "Loopback control panel port")
	demoPort := flag.Int("demo-port", 7333, "Built-in demo API port")
	target := flag.String("target", "", "Upstream HTTP(S) URL; defaults to the built-in demo")
	scenarioFile := flag.String("scenario", "", "Load a scenario JSON file at startup")
	showVersion := flag.Bool("version", false, "Print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("FaultDeck", version)
		return nil
	}
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments; use --help for usage")
	}
	ports := []int{*uiPort, *proxyPort, *demoPort}
	seen := map[int]bool{}
	for _, port := range ports {
		if port < 1 || port > 65535 || seen[port] {
			return fmt.Errorf("ports must be distinct integers between 1 and 65535")
		}
		seen[port] = true
	}
	urlFor := func(port int) string { return fmt.Sprintf("http://127.0.0.1:%d", port) }
	upstream := *target
	if upstream == "" {
		upstream = urlFor(*demoPort)
	}
	d, err := deck.New(deck.Config{Version: version, Upstream: upstream, AdminURL: urlFor(*uiPort), ProxyURL: urlFor(*proxyPort), DemoURL: urlFor(*demoPort)})
	if err != nil {
		return err
	}
	defer d.Close()
	if *scenarioFile != "" {
		file, err := os.Open(*scenarioFile)
		if err != nil {
			return fmt.Errorf("open scenario: %w", err)
		}
		info, statErr := file.Stat()
		if statErr != nil || info.Size() > 1<<20 {
			file.Close()
			return fmt.Errorf("scenario file must be at most 1 MiB")
		}
		decoder := json.NewDecoder(file)
		decoder.DisallowUnknownFields()
		var scenario deck.Scenario
		err = decoder.Decode(&scenario)
		if err == nil {
			if trailing := decoder.Decode(new(any)); trailing != io.EOF {
				err = fmt.Errorf("scenario must contain exactly one JSON value")
			}
		}
		file.Close()
		if err != nil {
			return fmt.Errorf("parse scenario: %w", err)
		}
		if err = d.Import(scenario); err != nil {
			return fmt.Errorf("load scenario: %w", err)
		}
		if *target != "" {
			return fmt.Errorf("use either --target or --scenario (the scenario already contains its target)")
		}
	}
	handlers := []http.Handler{d.AdminHandler(web.Handler()), d.ProxyHandler(), deck.DemoHandler()}
	listeners := make([]net.Listener, 0, len(ports))
	for _, port := range ports {
		listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			for _, open := range listeners {
				open.Close()
			}
			return fmt.Errorf("listen on port %d: %w (choose another port with --help)", port, err)
		}
		listeners = append(listeners, listener)
	}
	servers := make([]*http.Server, len(ports))
	serverErrors := make(chan error, len(ports))
	for i, handler := range handlers {
		servers[i] = &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
		go func(s *http.Server, listener net.Listener) {
			if err := s.Serve(listener); err != nil && err != http.ErrServerClosed {
				serverErrors <- err
			}
		}(servers[i], listeners[i])
	}
	fmt.Printf("\n  FAULTDECK  v%s\n  Break it here. Ship it stronger.\n\n  Control panel  %s\n  Proxy          %s\n  Upstream       %s\n  Demo API       %s\n\n  Local only. Press Ctrl+C to stop.\n\n", version, urlFor(*uiPort), urlFor(*proxyPort), d.State().Upstream, urlFor(*demoPort))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
	case err = <-serverErrors:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, server := range servers {
		if e := server.Shutdown(shutdown); e != nil {
			server.Close()
		}
	}
	return err
}
