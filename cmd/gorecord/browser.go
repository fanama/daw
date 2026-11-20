package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const appProfileName = "gorecord"

// launchAppWindow opens url in a chromeless window of an installed Chromium
// browser. The dedicated profile keeps the app's IndexedDB data separate from
// the user's everyday browsing profile.
func launchAppWindow(url, browserPath string) (<-chan struct{}, error) {
	bin := browserPath
	if bin == "" {
		var err error
		bin, err = findChromium()
		if err != nil {
			return nil, err
		}
	}
	profile, err := appProfileDir()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin,
		"--app="+url,
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-mode",
	)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	return done, nil
}

func appProfileDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appProfileName, "browser")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func findChromium() (string, error) {
	for _, path := range chromiumCandidates() {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	for _, name := range []string{
		"google-chrome", "google-chrome-stable", "chromium",
		"chromium-browser", "microsoft-edge", "brave-browser",
	} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("no Chromium browser found; pass -browser or use -window=browser")
}

func chromiumCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		apps := []string{"Google Chrome", "Chromium", "Microsoft Edge", "Brave Browser"}
		bases := []string{"/Applications", filepath.Join(os.Getenv("HOME"), "Applications")}
		var paths []string
		for _, base := range bases {
			for _, app := range apps {
				paths = append(paths, filepath.Join(base, app+".app", "Contents", "MacOS", app))
			}
		}
		return paths
	case "windows":
		roots := []string{
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("LocalAppData"),
		}
		relatives := []string{
			filepath.Join("Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join("Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join("Chromium", "Application", "chrome.exe"),
			filepath.Join("BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		}
		var paths []string
		for _, root := range roots {
			if root == "" {
				continue
			}
			for _, rel := range relatives {
				paths = append(paths, filepath.Join(root, rel))
			}
		}
		return paths
	default:
		return nil
	}
}

// launchBrowser opens the URL with the platform's default browser.
func launchBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("open browser: %v", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}
