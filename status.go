package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type StatusChecker struct {
	mu     sync.RWMutex
	cache  map[int64]statusCacheEntry
	client *http.Client
}

type statusCacheEntry struct {
	online    bool
	checkedAt time.Time
}

var checker = &StatusChecker{
	cache: make(map[int64]statusCacheEntry),
	client: &http.Client{
		Timeout: 2500 * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: (&net.Dialer{
				Timeout: 2 * time.Second,
			}).DialContext,
		},
	},
}

// CheckApp verifica se o app está online (com cache de 30 segundos).
func (c *StatusChecker) CheckApp(a App) bool {
	c.mu.RLock()
	entry, ok := c.cache[a.ID]
	c.mu.RUnlock()

	if ok && time.Since(entry.checkedAt) < 30*time.Second {
		return entry.online
	}

	online := c.ping(a.URL)

	c.mu.Lock()
	c.cache[a.ID] = statusCacheEntry{
		online:    online,
		checkedAt: time.Now(),
	}
	c.mu.Unlock()

	return online
}

func (c *StatusChecker) ping(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}

	// 1. Tentar primeiro HTTP HEAD/GET rápido
	req, err := http.NewRequest("GET", rawURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "SuperLauncher-HealthCheck")
		resp, err := c.client.Do(req)
		if err == nil {
			resp.Body.Close()
			// Qualquer resposta HTTP (mesmo 401 Unauthorized, 403, 500, etc.) significa que o servidor está vivo
			return true
		}
	}

	// 2. Se HTTP falhar (por exemplo, timeout ou erro de protocolo), tenta TCP connect direto na porta
	host := u.Host
	if !hasPort(host) {
		if u.Scheme == "https" {
			host = net.JoinHostPort(host, "443")
		} else {
			host = net.JoinHostPort(host, "80")
		}
	}

	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err == nil {
		conn.Close()
		return true
	}

	return false
}

func hasPort(host string) bool {
	_, _, err := net.SplitHostPort(host)
	return err == nil
}
