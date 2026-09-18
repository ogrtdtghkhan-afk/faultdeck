package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ogrtdtghkhan-afk/faultdeck/internal/deck"
	"github.com/ogrtdtghkhan-afk/faultdeck/internal/web"
)

var version = deck.Version

type options struct {
	proxyPort, uiPort, demoPort                               int
	listen, target, scenarioFile, dataDir, uiOrigin, proxyURL string
	showVersion, healthcheck                                  bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FaultDeck:", err)
		os.Exit(1)
	}
}

func parseOptions(args []string) (options, error) {
	var o options
	f := flag.NewFlagSet("faultdeck", flag.ContinueOnError)
	f.IntVar(&o.proxyPort, "port", 7332, "Proxy port")
	f.IntVar(&o.uiPort, "ui-port", 7331, "Control panel port")
	f.IntVar(&o.demoPort, "demo-port", 7333, "Built-in demo API port (always internal)")
	f.StringVar(&o.listen, "listen", "127.0.0.1", "IP address for control and proxy listeners; remote binding requires authentication")
	f.StringVar(&o.target, "target", "", "Upstream HTTP(S) URL; overrides saved target only when supplied")
	f.StringVar(&o.scenarioFile, "scenario", "", "Import a scenario JSON file at startup")
	f.StringVar(&o.dataDir, "data-dir", "./data", "Persistent workspace directory; empty string disables persistence")
	f.StringVar(&o.uiOrigin, "ui-origin", "", "Public control origin, e.g. https://faultdeck.example.com")
	f.StringVar(&o.proxyURL, "proxy-url", "", "Public proxy origin shown to clients")
	f.BoolVar(&o.showVersion, "version", false, "Print version")
	f.BoolVar(&o.healthcheck, "healthcheck", false, "Check the local control listener and exit")
	if err := f.Parse(args); err != nil {
		return o, err
	}
	if f.NArg() != 0 {
		return o, fmt.Errorf("unexpected arguments; use --help for usage")
	}
	return o, nil
}

func publicOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return "", fmt.Errorf("public URL must be an http(s) origin without credentials, path, query or fragment")
	}
	if p := u.Port(); p != "" {
		if _, err := net.LookupPort("tcp", p); err != nil {
			return "", fmt.Errorf("invalid public URL port")
		}
	}
	return u.Scheme + "://" + u.Host, nil
}

func (o options) config(env func(string) string) (deck.Config, error) {
	c := deck.Config{Version: version, DataDir: o.dataDir}
	seen := map[int]bool{}
	for _, port := range []int{o.uiPort, o.proxyPort, o.demoPort} {
		if port < 1 || port > 65535 || seen[port] {
			return c, fmt.Errorf("ports must be distinct integers between 1 and 65535")
		}
		seen[port] = true
	}
	bind := net.ParseIP(o.listen)
	if bind == nil {
		return c, fmt.Errorf("--listen must be an IP address, such as 127.0.0.1 or 0.0.0.0")
	}
	loopHost := "127.0.0.1"
	if bind.To4() == nil {
		loopHost = "::1"
	}
	internalHost := o.listen
	if bind.IsUnspecified() {
		internalHost = loopHost
	}
	urlFor := func(host string, port int) string { return "http://" + net.JoinHostPort(host, fmt.Sprint(port)) }
	c.InternalAdminURL = urlFor(internalHost, o.uiPort)
	c.InternalProxyURL = urlFor(internalHost, o.proxyPort)
	c.AdminURL, c.ProxyURL = c.InternalAdminURL, c.InternalProxyURL
	c.DemoURL = urlFor(loopHost, o.demoPort)
	var err error
	if o.uiOrigin != "" {
		c.AdminURL, err = publicOrigin(o.uiOrigin)
		if err != nil {
			return c, fmt.Errorf("--ui-origin: %w", err)
		}
	}
	if o.proxyURL != "" {
		c.ProxyURL, err = publicOrigin(o.proxyURL)
		if err != nil {
			return c, fmt.Errorf("--proxy-url: %w", err)
		}
	}
	c.AdminUser = env("FAULTDECK_ADMIN_USER")
	if c.AdminUser == "" {
		c.AdminUser = "admin"
	}
	if strings.ContainsAny(c.AdminUser, ":\r\n") {
		return c, fmt.Errorf("FAULTDECK_ADMIN_USER cannot contain a colon or newline")
	}
	c.AdminPassword = env("FAULTDECK_ADMIN_PASSWORD")
	c.ProxyToken = env("FAULTDECK_PROXY_TOKEN")
	if c.AdminPassword != "" && (len(c.AdminPassword) < 12 || strings.ContainsAny(c.AdminPassword, "\r\n")) {
		return c, fmt.Errorf("FAULTDECK_ADMIN_PASSWORD must contain at least 12 bytes and no newline")
	}
	admin, _ := url.Parse(c.AdminURL)
	adminIP := net.ParseIP(admin.Hostname())
	publicAdmin := !strings.EqualFold(admin.Hostname(), "localhost") && (adminIP == nil || !adminIP.IsLoopback())
	if (!bind.IsLoopback() || publicAdmin) && c.AdminPassword == "" {
		return c, fmt.Errorf("remote access requires FAULTDECK_ADMIN_PASSWORD (at least 12 bytes)")
	}
	if c.ProxyToken == "" {
		c.ProxyToken = c.AdminPassword
	}
	if c.ProxyToken != "" && (len(c.ProxyToken) < 12 || strings.ContainsAny(c.ProxyToken, "\r\n")) {
		return c, fmt.Errorf("FAULTDECK_PROXY_TOKEN must contain at least 12 bytes and no newline")
	}
	if o.target != "" && o.scenarioFile != "" {
		return c, fmt.Errorf("use either --target or --scenario (the scenario already contains its target)")
	}
	c.Upstream = c.DemoURL
	if o.target != "" {
		c.Upstream = o.target
	}
	return c, nil
}

func checkHealth(controlURL string) error {
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(controlURL + "/healthz")
	if err != nil {
		return fmt.Errorf("control listener is not healthy")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %d", response.StatusCode)
	}
	return nil
}

func loadScenario(path string) (deck.Scenario, error) {
	var scenario deck.Scenario
	file, err := os.Open(path)
	if err != nil {
		return scenario, fmt.Errorf("open scenario: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 1<<20 {
		return scenario, fmt.Errorf("scenario file must be at most 1 MiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, (1<<20)+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&scenario); err != nil {
		return scenario, fmt.Errorf("parse scenario: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return scenario, fmt.Errorf("scenario must contain exactly one JSON value")
	}
	return scenario, nil
}

func run() error {
	o, err := parseOptions(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if o.showVersion {
		fmt.Println("FaultDeck", version)
		return nil
	}
	config, err := o.config(os.Getenv)
	if err != nil {
		return err
	}
	if o.healthcheck {
		return checkHealth(config.InternalAdminURL)
	}
	d, err := deck.New(config)
	if err != nil {
		return err
	}
	defer d.Close()
	if o.target != "" {
		if _, err := d.Configure(&o.target, nil); err != nil {
			return err
		}
	}
	if o.scenarioFile != "" {
		scenario, err := loadScenario(o.scenarioFile)
		if err != nil {
			return err
		}
		if err = d.Import(scenario); err != nil {
			return fmt.Errorf("load scenario: %w", err)
		}
	}
	handlers := []http.Handler{d.AdminHandler(web.Handler()), d.ProxyHandler(), deck.DemoHandler()}
	demo, _ := url.Parse(config.DemoURL)
	addresses := []string{net.JoinHostPort(o.listen, fmt.Sprint(o.uiPort)), net.JoinHostPort(o.listen, fmt.Sprint(o.proxyPort)), demo.Host}
	listeners := make([]net.Listener, 0, len(addresses))
	for _, address := range addresses {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			for _, open := range listeners {
				open.Close()
			}
			return fmt.Errorf("listen on %s: %w (choose another port with --help)", address, err)
		}
		listeners = append(listeners, listener)
	}
	servers := make([]*http.Server, len(addresses))
	serverErrors := make(chan error, len(addresses))
	for i, handler := range handlers {
		servers[i] = &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
		go func(s *http.Server, listener net.Listener) {
			if err := s.Serve(listener); err != nil && err != http.ErrServerClosed {
				serverErrors <- err
			}
		}(servers[i], listeners[i])
	}
	auth := "local access without authentication"
	if config.AdminPassword != "" {
		auth = "admin login and proxy token required"
	}
	persistence := "disabled (in-memory workspace)"
	if config.DataDir != "" {
		persistence = config.DataDir
	}
	fmt.Printf("\n  FAULTDECK  v%s\n\n  Control panel  %s\n  Proxy          %s\n  Upstream       %s\n  Workspace      %s\n  Access         %s\n\n  Press Ctrl+C to stop.\n\n", version, config.AdminURL, config.ProxyURL, d.State().Upstream, persistence, auth)
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
