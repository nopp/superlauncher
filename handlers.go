package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cookieName = "sl_session"
	sessionTTL = 7 * 24 * time.Hour
	maxUpload  = 5 << 20 // 5 MB
)

type server struct {
	store     *Store
	tpl       map[string]*template.Template
	uploadDir string
	password  string
	secret    []byte
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	static, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /uploads/", s.serveUpload)

	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /api/status", s.appStatusAPI)

	mux.HandleFunc("GET /admin/login", s.loginPage)
	mux.HandleFunc("POST /admin/login", s.login)
	mux.HandleFunc("POST /admin/logout", s.logout)

	mux.HandleFunc("GET /admin", s.auth(s.adminList))
	mux.HandleFunc("GET /admin/apps/new", s.auth(s.newForm))
	mux.HandleFunc("POST /admin/apps", s.auth(s.create))
	mux.HandleFunc("GET /admin/apps/{id}/edit", s.auth(s.editForm))
	mux.HandleFunc("POST /admin/apps/{id}", s.auth(s.update))
	mux.HandleFunc("POST /admin/apps/{id}/delete", s.auth(s.delete))
	mux.HandleFunc("GET /admin/fetch-icon", s.auth(s.fetchIconAPI))

	return mux
}

// ---------- rendering ----------

func (s *server) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := s.tpl[name].ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("template %s: %v", name, err)
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func serverError(w http.ResponseWriter, err error) {
	log.Println("erro:", err)
	http.Error(w, "erro interno", http.StatusInternalServerError)
}

// ---------- public ----------

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	apps, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "index.html", map[string]any{"Apps": apps})
}

func (s *server) appStatusAPI(w http.ResponseWriter, r *http.Request) {
	apps, err := s.store.List()
	if err != nil {
		http.Error(w, `{}`, http.StatusInternalServerError)
		return
	}

	results := make(map[string]bool, len(apps))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, app := range apps {
		wg.Add(1)
		go func(a App) {
			defer wg.Done()
			online := checker.CheckApp(a)
			mu.Lock()
			results[strconv.FormatInt(a.ID, 10)] = online
			mu.Unlock()
		}(app)
	}

	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(results)
}

func (s *server) serveUpload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/uploads/")
	if name == "" || strings.Contains(name, "/") || strings.HasPrefix(name, ".") {
		http.NotFound(w, r)
		return
	}
	// Uploaded SVGs must never execute scripts.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(s.uploadDir, name))
}

// ---------- auth ----------

func (s *server) sign(v string) string {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(v))
	return hex.EncodeToString(m.Sum(nil))
}

func (s *server) loggedIn(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	exp, sig, ok := strings.Cut(c.Value, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(s.sign(exp))) {
		return false
	}
	n, err := strconv.ParseInt(exp, 10, 64)
	return err == nil && time.Now().Unix() < n
}

func (s *server) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.loggedIn(r) {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

func (s *server) loginPage(w http.ResponseWriter, r *http.Request) {
	if s.loggedIn(r) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	s.render(w, "login.html", map[string]any{})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	pw := r.FormValue("password")
	if subtle.ConstantTimeCompare([]byte(pw), []byte(s.password)) != 1 {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusUnauthorized)
		s.render(w, "login.html", map[string]any{"Error": "Senha incorreta."})
		return
	}
	exp := strconv.FormatInt(time.Now().Add(sessionTTL).Unix(), 10)
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: exp + "." + s.sign(exp), Path: "/",
		MaxAge: int(sessionTTL.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ---------- admin CRUD ----------

var flashMessages = map[string]string{
	"created": "App cadastrado com sucesso.",
	"updated": "App atualizado com sucesso.",
	"deleted": "App excluído.",
}

func (s *server) adminList(w http.ResponseWriter, r *http.Request) {
	apps, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	s.render(w, "list.html", map[string]any{"Apps": apps, "Msg": flashMessages[r.URL.Query().Get("msg")]})
}

func (s *server) fetchIconAPI(w http.ResponseWriter, r *http.Request) {
	targetURL := normalizeURL(r.URL.Query().Get("url"))
	if !validURL(targetURL) {
		http.Error(w, `{"error":"URL inválida"}`, http.StatusBadRequest)
		return
	}
	iconURL, err := FetchFavicon(targetURL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":%q}`, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"icon_url":%q}`, iconURL)
}

type formData struct {
	App     App
	IsNew   bool
	Error   string
	IconURL string
}

func (s *server) renderForm(w http.ResponseWriter, d formData) {
	if d.IconURL == "" && d.App.Icon != "" && !isUpload(d.App.Icon) {
		d.IconURL = d.App.Icon
	}
	s.render(w, "form.html", d)
}

func (s *server) newForm(w http.ResponseWriter, r *http.Request) {
	s.renderForm(w, formData{App: App{NewTab: true, Position: s.store.NextPosition()}, IsNew: true})
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	var a App
	if _, err := s.fill(w, r, &a); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.renderForm(w, formData{App: a, IsNew: true, Error: err.Error(), IconURL: r.FormValue("icon_url")})
		return
	}
	if err := s.store.Create(&a); err != nil {
		s.removeUpload(a.Icon)
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/admin?msg=created", http.StatusSeeOther)
}

func (s *server) loadApp(w http.ResponseWriter, r *http.Request) (App, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return App{}, false
	}
	a, err := s.store.Get(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return App{}, false
	}
	if err != nil {
		serverError(w, err)
		return App{}, false
	}
	return a, true
}

func (s *server) editForm(w http.ResponseWriter, r *http.Request) {
	if a, ok := s.loadApp(w, r); ok {
		s.renderForm(w, formData{App: a})
	}
}

func (s *server) update(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadApp(w, r)
	if !ok {
		return
	}
	oldIcon, err := s.fill(w, r, &a)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.renderForm(w, formData{App: a, Error: err.Error(), IconURL: r.FormValue("icon_url")})
		return
	}
	if err := s.store.Update(&a); err != nil {
		serverError(w, err)
		return
	}
	s.removeUpload(oldIcon)
	http.Redirect(w, r, "/admin?msg=updated", http.StatusSeeOther)
}

func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadApp(w, r)
	if !ok {
		return
	}
	if err := s.store.Delete(a.ID); err != nil {
		serverError(w, err)
		return
	}
	s.removeUpload(a.Icon)
	http.Redirect(w, r, "/admin?msg=deleted", http.StatusSeeOther)
}

// fill reads the form into a. It returns the path of a previously uploaded
// icon that was replaced/removed (to be deleted after a successful save).
func (s *server) fill(w http.ResponseWriter, r *http.Request, a *App) (string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+(1<<20))
	if err := r.ParseMultipartForm(maxUpload); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		return "", errors.New("Formulário inválido ou ícone maior que 5 MB.")
	}

	a.Name = strings.TrimSpace(r.FormValue("name"))
	a.URL = normalizeURL(r.FormValue("url"))
	a.Description = strings.TrimSpace(r.FormValue("description"))
	a.Position, _ = strconv.Atoi(r.FormValue("position"))
	a.NewTab = r.FormValue("new_tab") != ""

	if a.Name == "" {
		return "", errors.New("Informe o nome do app.")
	}
	if !validURL(a.URL) {
		return "", errors.New("Informe uma URL válida (ex.: http://192.168.0.10:8080).")
	}
	iconURL := strings.TrimSpace(r.FormValue("icon_url"))
	if iconURL != "" && !validURL(iconURL) {
		return "", errors.New("A URL do ícone precisa começar com http:// ou https://.")
	}

	old := a.Icon
	file, hdr, ferr := r.FormFile("icon_file")
	switch {
	case ferr == nil:
		defer file.Close()
		p, err := s.saveUpload(file, hdr)
		if err != nil {
			return "", err
		}
		a.Icon = p
	case r.FormValue("remove_icon") != "":
		a.Icon = ""
	case iconURL != "":
		a.Icon = iconURL
	case isUpload(old):
		// keep the current uploaded icon
	default:
		a.Icon = ""
	}
	if old != a.Icon && isUpload(old) {
		return old, nil
	}
	return "", nil
}

var allowedExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".ico": true,
}

func (s *server) saveUpload(f multipart.File, h *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(h.Filename))
	if !allowedExt[ext] {
		return "", errors.New("Formato de ícone não suportado (use png, jpg, gif, webp, svg ou ico).")
	}
	b := make([]byte, 12)
	rand.Read(b)
	name := hex.EncodeToString(b) + ext
	dst, err := os.Create(filepath.Join(s.uploadDir, name))
	if err != nil {
		return "", fmt.Errorf("não foi possível salvar o ícone: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, f); err != nil {
		os.Remove(dst.Name())
		return "", fmt.Errorf("não foi possível salvar o ícone: %w", err)
	}
	return "/uploads/" + name, nil
}

func isUpload(p string) bool { return strings.HasPrefix(p, "/uploads/") }

func (s *server) removeUpload(p string) {
	if isUpload(p) {
		os.Remove(filepath.Join(s.uploadDir, filepath.Base(p)))
	}
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw != "" && !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	return raw
}

func validURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
