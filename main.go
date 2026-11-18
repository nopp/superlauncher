package main

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"hash/fnv"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := env("ADDR", ":9090")
	dataDir := env("DATA_DIR", "data")
	password := env("ADMIN_PASSWORD", "admin")
	if password == "admin" {
		log.Println("AVISO: usando a senha padrão 'admin'. Defina ADMIN_PASSWORD para trocá-la.")
	}

	uploadDir := filepath.Join(dataDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Fatal(err)
	}

	store, err := OpenStore(filepath.Join(dataDir, "superlauncher.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	secret := sha256.Sum256([]byte("superlauncher-session:" + password))
	s := &server{
		store:     store,
		tpl:       mustTemplates(),
		uploadDir: uploadDir,
		password:  password,
		secret:    secret[:],
	}

	log.Printf("Super Launcher rodando em http://localhost%s (admin em /admin)", addr)
	log.Fatal(http.ListenAndServe(addr, s.routes()))
}

func mustTemplates() map[string]*template.Template {
	funcs := template.FuncMap{"initial": initial, "color": color}
	pages := []string{"index.html", "login.html", "list.html", "form.html"}
	m := make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		m[p] = template.Must(template.New(p).Funcs(funcs).
			ParseFS(templatesFS, "templates/partials.html", "templates/"+p))
	}
	return m
}

// initial returns the first letter of the app name, used when there's no icon.
func initial(name string) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) == 0 {
		return "?"
	}
	return strings.ToUpper(string(r[0]))
}

// color returns a stable background color derived from the app name.
func color(name string) template.CSS {
	h := fnv.New32a()
	h.Write([]byte(name))
	return template.CSS(fmt.Sprintf("hsl(%d, 55%%, 45%%)", h.Sum32()%360))
}
