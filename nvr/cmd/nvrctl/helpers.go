package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var scanner = bufio.NewScanner(os.Stdin)

func tokenPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "nvrctl", "token")
}

func loadToken() string {
	data, err := os.ReadFile(tokenPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveToken(token string) error {
	p := tokenPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(token), 0600)
}

func prompt(label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}
	scanner.Scan()
	v := strings.TrimSpace(scanner.Text())
	if v == "" {
		return defaultVal
	}
	return v
}

func promptSecret(label string) string {
	fmt.Printf("  %s: ", label)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func probe(host, port string) string {
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return "FAIL"
	}
	conn.Close()
	return "OK"
}

func probeHTTP(host string, port int) string {
	url := fmt.Sprintf("http://%s:%d/", host, port)
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Get(url)
	if err != nil {
		return "FAIL"
	}
	resp.Body.Close()
	return fmt.Sprintf("%d", resp.StatusCode)
}

func yn(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
