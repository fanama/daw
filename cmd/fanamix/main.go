// Command fanamix serves the browser build on localhost and opens it as a
// desktop app window (or in the default browser).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	appPath       = "/app.html"
	livePath      = "/__fanamix/events"
	liveGrace     = 2 * time.Second
	liveHeartbeat = 15 * time.Second
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address; port 0 picks a free port")
	dir := flag.String("dir", "", "directory holding the browser build (default: web/, or Resources/web in an app bundle)")
	window := flag.String("window", "app", "where to open the app: app, browser or none")
	browserPath := flag.String("browser", "", "Chromium browser binary for app mode")
	flag.Parse()

	root, err := resolveWebDir(*dir)
	if err != nil {
		log.Fatalf("browser build: %v; run \"make web\" first", err)
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", *addr, err)
	}
	url := appURL(listener.Addr())
	log.Printf("fanamix: %s", url)
	quit := make(chan struct{})
	var once sync.Once
	shutdown := func() {
		once.Do(func() { close(quit) })
	}
	server := &http.Server{Handler: &webServer{root: root, shutdown: shutdown}}
	go serve(server, listener)
	waitForExit(openWindow(*window, url, *browserPath), quit)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func serve(server *http.Server, listener net.Listener) {
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func waitForExit(done, quit <-chan struct{}) {
	if done == nil {
		<-quit
		return
	}
	select {
	case <-done:
	case <-quit:
	}
}

func openWindow(mode, baseURL, browserPath string) <-chan struct{} {
	url := baseURL
	if mode != "none" {
		// The site root is the welcome page; the app window and tab open the
		// recorder directly. The page keeps an SSE connection open only for this
		// launcher. A static deployment has no such endpoint and must not retry it.
		url = strings.TrimRight(url, "/") + appPath + "?desktop=1"
	}
	switch mode {
	case "none":
		return nil
	case "browser":
		launchBrowser(url)
		return nil
	case "app":
		done, err := launchAppWindow(url, browserPath)
		if err == nil {
			return done
		}
		log.Printf("app window: %v; opening the default browser instead", err)
		launchBrowser(url)
		return nil
	default:
		log.Fatalf("unknown -window %q (want app, browser or none)", mode)
		return nil
	}
}

// resolveWebDir finds the browser build in the working directory during
// development, or next to the executable inside a macOS app bundle.
func resolveWebDir(configured string) (string, error) {
	if configured != "" {
		if err := requireBuild(configured); err != nil {
			return "", err
		}
		return configured, nil
	}
	candidates := []string{"web"}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), "..", "Resources", "web"))
	}
	var problems []string
	for _, candidate := range candidates {
		if err := requireBuild(candidate); err == nil {
			return candidate, nil
		}
		problems = append(problems, candidate)
	}
	return "", fmt.Errorf("no complete build found in %s", strings.Join(problems, ", "))
}

// requireBuild checks the generated files before the server starts, so a
// missing wasm build fails with a build hint instead of a blank page.
func requireBuild(root string) error {
	for _, name := range []string{"index.html", "app.html", "fanamix.wasm", "wasm_exec.js"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			return fmt.Errorf("%s is missing from %s", name, root)
		}
	}
	return nil
}

// appURL turns the bound listener address into a browser URL.
func appURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String()
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

type webServer struct {
	root      string
	shutdown  func()
	mu        sync.Mutex
	live      int
	quitTimer *time.Timer
}

func (s *webServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == livePath {
		s.events(w, r)
		return
	}
	clean := path.Clean("/" + r.URL.Path)
	if clean == "/" {
		clean = "/index.html"
	}
	if strings.ContainsAny(clean, "\\") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if strings.HasSuffix(clean, ".wasm") {
		w.Header().Set("Content-Type", "application/wasm")
	}
	file := filepath.Join(s.root, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
	http.ServeFile(w, r, file)
}

// events keeps an SSE connection open for each app page. When the last one
// closes, the launcher shuts down; a reload reconnects within the grace period.
func (s *webServer) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.live++
	if s.quitTimer != nil {
		s.quitTimer.Stop()
		s.quitTimer = nil
	}
	s.mu.Unlock()
	defer s.streamClosed()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, "retry: 1000\n\n"); err != nil {
		return
	}
	flusher.Flush()
	ticker := time.NewTicker(liveHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *webServer) streamClosed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.live > 0 {
		s.live--
	}
	if s.live > 0 {
		return
	}
	if s.quitTimer != nil {
		s.quitTimer.Stop()
	}
	s.quitTimer = time.AfterFunc(liveGrace, s.shutdown)
}
