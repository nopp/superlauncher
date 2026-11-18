package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // útil para serviços locais https auto-assinados
	},
}

var (
	linkRegex = regexp.MustCompile(`(?i)<link[^>]+(?:rel=["']?(?:icon|shortcut\s+icon|apple-touch-icon)[^"'>]*)?[^>]*>`)
	hrefRegex = regexp.MustCompile(`(?i)href=["']([^"']+)["']`)
	relRegex  = regexp.MustCompile(`(?i)rel=["']([^"']+)["']`)
)

// FetchFavicon tenta descobrir o ícone de uma URL web.
// Ele analisa a página HTML em busca de tags <link rel="icon">, apple-touch-icon,
// e se não encontrar, tenta obter diretamente o /favicon.ico.
func FetchFavicon(pageURL string) (string, error) {
	u, err := url.Parse(pageURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("URL inválida")
	}

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (SuperLauncher Favicon Finder)")

	resp, err := httpClient.Do(req)
	if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 400 {
		defer resp.Body.Close()

		// Ler os primeiros 128KB do HTML
		lr := io.LimitReader(resp.Body, 128*1024)
		htmlBytes, _ := io.ReadAll(lr)
		html := string(htmlBytes)

		// Procurar por links de favicon/apple-touch-icon
		var bestHref string
		for _, tag := range linkRegex.FindAllString(html, -1) {
			relMatch := relRegex.FindStringSubmatch(tag)
			hrefMatch := hrefRegex.FindStringSubmatch(tag)

			if len(relMatch) > 1 && len(hrefMatch) > 1 {
				rel := strings.ToLower(relMatch[1])
				href := strings.TrimSpace(hrefMatch[1])

				if strings.Contains(rel, "apple-touch-icon") {
					bestHref = href // apple-touch-icon costuma ter resolução melhor
					break
				}
				if strings.Contains(rel, "icon") {
					bestHref = href
				}
			}
		}

		if bestHref != "" {
			resolved := resolveURL(resp.Request.URL, bestHref)
			if checkImageExists(resolved) {
				return resolved, nil
			}
		}
	}

	// Fallback 1: tentar origin/favicon.ico
	defaultFavicon := fmt.Sprintf("%s://%s/favicon.ico", u.Scheme, u.Host)
	if checkImageExists(defaultFavicon) {
		return defaultFavicon, nil
	}

	// Fallback 2: Se for domínio externo na internet pública (não IP privado/localhost),
	// podemos usar a API de favicons do Google.
	host := u.Hostname()
	if isPublicHostname(host) {
		googleFavicon := fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=128", url.QueryEscape(host))
		return googleFavicon, nil
	}

	return "", errors.New("não foi possível encontrar um ícone automaticamente")
}

func resolveURL(base *url.URL, target string) string {
	t, err := url.Parse(target)
	if err != nil {
		return target
	}
	return base.ResolveReference(t).String()
}

func checkImageExists(rawURL string) bool {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (SuperLauncher Favicon Finder)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func isPublicHostname(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "10.") || strings.HasSuffix(host, ".local") {
		return false
	}
	return strings.Contains(host, ".")
}
