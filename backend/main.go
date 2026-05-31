package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type App struct {
	db                                       *sql.DB
	adminHash, staticDir, uploadDir, baseURL string
}
type Category struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Count int    `json:"count"`
}
type Post struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Slug          string    `json:"slug"`
	Excerpt       string    `json:"excerpt"`
	Content       string    `json:"content"`
	FeaturedImage string    `json:"featured_image"`
	PublishedAt   time.Time `json:"published_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Status        string    `json:"status"`
	Category      *Category `json:"category,omitempty"`
	CategoryID    *int64    `json:"category_id,omitempty"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9а-яё]+`)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL missing")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	app := &App{db: db, adminHash: os.Getenv("ADMIN_PASSWORD_SHA256"), staticDir: first(os.Getenv("STATIC_DIR"), "./dist"), uploadDir: first(os.Getenv("UPLOAD_DIR"), "./uploads"), baseURL: first(os.Getenv("BASE_URL"), "https://haifa.news")}
	if err := app.migrate(context.Background()); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/categories", app.categories)
	mux.HandleFunc("/api/posts", app.posts)
	mux.HandleFunc("/api/posts/", app.postBySlug)
	mux.HandleFunc("/api/admin/login", app.login)
	mux.HandleFunc("/api/admin/posts", app.adminPosts)
	mux.HandleFunc("/api/admin/posts/", app.adminPostID)
	mux.HandleFunc("/api/admin/upload", app.uploadImage)
	mux.HandleFunc("/sitemap.xml", app.sitemap)
	mux.HandleFunc("/sitemap_index.xml", app.sitemapIndex)
	mux.HandleFunc("/robots.txt", app.robots)
	mux.HandleFunc("/assets/", app.static)
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(app.uploadDir))))
	mux.HandleFunc("/", app.page)
	addr := first(os.Getenv("ADDR"), "127.0.0.1:8088")
	log.Println("Haifa.News Go backend on", addr)
	log.Fatal(http.ListenAndServe(addr, securityHeaders(mux)))
}
func first(v, d string) string {
	if v != "" {
		return v
	}
	return d
}
func (a *App) migrate(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS categories(id BIGSERIAL PRIMARY KEY, wp_id BIGINT UNIQUE, name TEXT NOT NULL, slug TEXT NOT NULL UNIQUE, count INT DEFAULT 0, created_at TIMESTAMPTZ DEFAULT now());CREATE TABLE IF NOT EXISTS posts(id BIGSERIAL PRIMARY KEY, wp_id BIGINT UNIQUE, category_id BIGINT REFERENCES categories(id) ON DELETE SET NULL, title TEXT NOT NULL, slug TEXT NOT NULL UNIQUE, excerpt TEXT DEFAULT '', content TEXT DEFAULT '', featured_image TEXT DEFAULT '', status TEXT NOT NULL DEFAULT 'publish', published_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), created_at TIMESTAMPTZ DEFAULT now());CREATE INDEX IF NOT EXISTS idx_posts_pub ON posts(status,published_at DESC);CREATE INDEX IF NOT EXISTS idx_posts_cat ON posts(category_id,published_at DESC);CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY, expires_at TIMESTAMPTZ NOT NULL);`)
	return err
}

func (a *App) categories(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT c.id,c.name,c.slug,COUNT(p.id) FROM categories c LEFT JOIN posts p ON p.category_id=c.id AND p.status='publish' GROUP BY c.id ORDER BY COUNT(p.id) DESC,c.name`)
	if err != nil {
		httpError(w, err)
		return
	}
	defer rows.Close()
	out := make([]Category, 0)
	for rows.Next() {
		var c Category
		rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Count)
		out = append(out, c)
	}
	jsonOut(w, out)
}
func (a *App) posts(w http.ResponseWriter, r *http.Request) {
	limit := clamp(qint(r, "limit", 24), 1, 100)
	cat := r.URL.Query().Get("category")
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	args := []interface{}{}
	where := `p.status='publish'`
	if cat != "" {
		args = append(args, cat)
		where += fmt.Sprintf(" AND c.slug=$%d", len(args))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		where += fmt.Sprintf(" AND (p.title ILIKE $%d OR p.excerpt ILIKE $%d OR p.content ILIKE $%d)", len(args), len(args), len(args))
	}
	args = append(args, limit)
	rows, err := a.db.Query(`SELECT p.id,p.title,p.slug,p.excerpt,p.content,p.featured_image,p.published_at,p.updated_at,p.status,c.id,c.name,c.slug FROM posts p LEFT JOIN categories c ON c.id=p.category_id WHERE `+where+fmt.Sprintf(` ORDER BY p.published_at DESC LIMIT $%d`, len(args)), args...)
	if err != nil {
		httpError(w, err)
		return
	}
	defer rows.Close()
	out, err := scanPosts(rows)
	if err != nil {
		httpError(w, err)
		return
	}
	jsonOut(w, out)
}
func (a *App) postBySlug(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/posts/")
	p, err := a.getPost(slug, false)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	jsonOut(w, p)
}
func (a *App) getPost(slug string, anyStatus bool) (Post, error) {
	where := `p.slug=$1`
	if !anyStatus {
		where += ` AND p.status='publish'`
	}
	row := a.db.QueryRow(`SELECT p.id,p.title,p.slug,p.excerpt,p.content,p.featured_image,p.published_at,p.updated_at,p.status,c.id,c.name,c.slug FROM posts p LEFT JOIN categories c ON c.id=p.category_id WHERE `+where, slug)
	rows := &singleRow{row: row}
	out, err := scanPosts(rows)
	if err != nil || len(out) == 0 {
		return Post{}, errors.New("not found")
	}
	return out[0], nil
}

type singleRow struct {
	row  *sql.Row
	done bool
}

func (s *singleRow) Next() bool {
	if s.done {
		return false
	}
	s.done = true
	return true
}
func (s *singleRow) Close() error                   { return nil }
func (s *singleRow) Scan(dest ...interface{}) error { return s.row.Scan(dest...) }

type scanner interface {
	Next() bool
	Scan(...interface{}) error
	Close() error
}

func scanPosts(rows scanner) ([]Post, error) {
	out := make([]Post, 0)
	for rows.Next() {
		var p Post
		var cid sql.NullInt64
		var cn, cs sql.NullString
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Excerpt, &p.Content, &p.FeaturedImage, &p.PublishedAt, &p.UpdatedAt, &p.Status, &cid, &cn, &cs); err != nil {
			return nil, err
		}
		if cid.Valid {
			id := cid.Int64
			p.CategoryID = &id
			p.Category = &Category{ID: cid.Int64, Name: cn.String, Slug: cs.String}
		}
		out = append(out, p)
	}
	return out, nil
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	sum := sha256.Sum256([]byte(in.Password))
	if a.adminHash == "" || hex.EncodeToString(sum[:]) != a.adminHash {
		http.Error(w, "bad login", 401)
		return
	}
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	if _, err := a.db.Exec(`INSERT INTO sessions(token,expires_at) VALUES($1,now()+interval '12 hours')`, token); err != nil {
		httpError(w, err)
		return
	}
	jsonOut(w, map[string]string{"token": token})
}
func (a *App) auth(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(h, "Bearer ")
	var ok bool
	_ = a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sessions WHERE token=$1 AND expires_at>now())`, token).Scan(&ok)
	return ok
}
func (a *App) adminPosts(w http.ResponseWriter, r *http.Request) {
	if !a.auth(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Method == "GET" {
		limit := clamp(qint(r, "limit", 50), 1, 200)
		rows, err := a.db.Query(`SELECT p.id,p.title,p.slug,p.excerpt,p.content,p.featured_image,p.published_at,p.updated_at,p.status,c.id,c.name,c.slug FROM posts p LEFT JOIN categories c ON c.id=p.category_id ORDER BY p.published_at DESC LIMIT $1`, limit)
		if err != nil {
			httpError(w, err)
			return
		}
		defer rows.Close()
		out, err := scanPosts(rows)
		if err != nil {
			httpError(w, err)
			return
		}
		jsonOut(w, out)
		return
	}
	if r.Method == "POST" {
		a.savePost(w, r, false)
		return
	}
	if r.Method == "PUT" {
		a.savePost(w, r, true)
		return
	}
	http.Error(w, "method", 405)
}
func (a *App) adminPostID(w http.ResponseWriter, r *http.Request) {
	if !a.auth(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Method != "DELETE" {
		http.Error(w, "method", 405)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/posts/")
	_, err := a.db.Exec(`DELETE FROM posts WHERE id=$1`, id)
	if err != nil {
		httpError(w, err)
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (a *App) uploadImage(w http.ResponseWriter, r *http.Request) {
	if !a.auth(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		httpError(w, err)
		return
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		httpError(w, err)
		return
	}
	defer f.Close()
	ext := strings.ToLower(filepath.Ext(h.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}
	if !allowed[ext] {
		http.Error(w, "unsupported image type", 400)
		return
	}
	b := make([]byte, 8)
	rand.Read(b)
	sub := time.Now().Format("2006/01")
	dir := filepath.Join(a.uploadDir, "admin", sub)
	if err := os.MkdirAll(dir, 0755); err != nil {
		httpError(w, err)
		return
	}
	name := hex.EncodeToString(b) + ext
	out, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		httpError(w, err)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, f); err != nil {
		httpError(w, err)
		return
	}
	jsonOut(w, map[string]string{"url": "/uploads/admin/" + sub + "/" + name})
}
func (a *App) savePost(w http.ResponseWriter, r *http.Request, update bool) {
	var p Post
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpError(w, err)
		return
	}
	if p.Slug == "" {
		p.Slug = slugify(p.Title)
	}
	if p.PublishedAt.IsZero() {
		p.PublishedAt = time.Now()
	}
	if update && p.ID > 0 {
		_, err := a.db.Exec(`UPDATE posts SET title=$1,slug=$2,excerpt=$3,content=$4,featured_image=$5,category_id=$6,status=$7,updated_at=now() WHERE id=$8`, p.Title, p.Slug, p.Excerpt, p.Content, p.FeaturedImage, p.CategoryID, p.Status, p.ID)
		if err != nil {
			httpError(w, err)
			return
		}
		jsonOut(w, map[string]bool{"ok": true})
		return
	}
	_, err := a.db.Exec(`INSERT INTO posts(title,slug,excerpt,content,featured_image,category_id,status,published_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now())`, p.Title, p.Slug, p.Excerpt, p.Content, p.FeaturedImage, p.CategoryID, first(p.Status, "publish"), p.PublishedAt)
	if err != nil {
		httpError(w, err)
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}

func (a *App) page(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/wp-") || strings.HasPrefix(r.URL.Path, "/wp/") {
		http.Error(w, "Page removed", http.StatusGone)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/admin") {
		a.index(w, r)
		return
	}
	if r.URL.Path == "/" {
		a.renderHome(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/article/") {
		slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/article/"), "/")
		p, err := a.getPost(slug, false)
		if err == nil {
			a.renderArticle(w, r, p)
			return
		}
	}
	if strings.HasPrefix(r.URL.Path, "/category/") {
		a.index(w, r)
		return
	}
	a.index(w, r)
}
func (a *App) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	http.ServeFile(w, r, filepath.Join(a.staticDir, "index.html"))
}
func (a *App) static(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	http.StripPrefix("/", http.FileServer(http.Dir(a.staticDir))).ServeHTTP(w, r)
}
func (a *App) renderHome(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT p.id,p.title,p.slug,p.excerpt,p.content,p.featured_image,p.published_at,p.updated_at,p.status,c.id,c.name,c.slug FROM posts p LEFT JOIN categories c ON c.id=p.category_id WHERE p.status='publish' ORDER BY p.published_at DESC LIMIT 12`)
	if err != nil {
		httpError(w, err)
		return
	}
	defer rows.Close()
	posts, _ := scanPosts(rows)
	data := map[string]interface{}{"Title": "Haifa.News — новости Хайфы", "Desc": "Новости Хайфы, Израиля и севера: город, безопасность, бизнес, культура.", "Posts": posts, "Base": a.baseURL, "Canonical": a.baseURL + "/"}
	templates.ExecuteTemplate(w, "home", data)
}
func (a *App) renderArticle(w http.ResponseWriter, r *http.Request, p Post) {
	data := map[string]interface{}{"Title": p.Title + " — Haifa.News", "Desc": p.Excerpt, "Post": p, "Base": a.baseURL, "Canonical": a.baseURL + "/article/" + p.Slug}
	templates.ExecuteTemplate(w, "article", data)
}
func (a *App) sitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	rows, err := a.db.Query(`SELECT slug,updated_at FROM posts WHERE status='publish' ORDER BY updated_at DESC LIMIT 50000`)
	if err != nil {
		httpError(w, err)
		return
	}
	defer rows.Close()
	io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>`+a.baseURL+`/</loc></url>`)
	cats, _ := a.db.Query(`SELECT slug FROM categories ORDER BY name`)
	if cats != nil {
		defer cats.Close()
		for cats.Next() {
			var slug string
			cats.Scan(&slug)
			fmt.Fprintf(w, "<url><loc>%s/category/%s</loc></url>", a.baseURL, template.URLQueryEscaper(slug))
		}
	}
	for rows.Next() {
		var slug string
		var upd time.Time
		rows.Scan(&slug, &upd)
		fmt.Fprintf(w, "<url><loc>%s/article/%s</loc><lastmod>%s</lastmod></url>", a.baseURL, template.URLQueryEscaper(slug), upd.Format("2006-01-02"))
	}
	io.WriteString(w, "</urlset>")
}
func (a *App) sitemapIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprintf(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?><sitemapindex xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\"><sitemap><loc>%s/sitemap.xml</loc></sitemap></sitemapindex>", a.baseURL)
}
func (a *App) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", a.baseURL)
}

var templates = template.Must(template.New("t").Funcs(template.FuncMap{"safe": func(s string) template.HTML { return template.HTML(s) }}).Parse(`{{define "head"}}<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><meta name="description" content="{{.Desc}}"><meta name="robots" content="index, follow, max-image-preview:large"><link rel="canonical" href="{{.Canonical}}"><link rel="stylesheet" href="/assets/index.css?v=20260525v8"></head><body><div id="app">{{end}}{{define "nav"}}<div class="ticker"><span><b>LIVE</b> Haifa.News · Хайфа · Север Израиля · город · безопасность · общество</span></div><header class="head"><div class="wrap bar"><a class="brand" href="/"><span>H</span><strong>Haifa.News</strong><small>independent city desk</small></a><nav><a href="/">Главная</a><a href="/category/новости-израиля">Израиль</a><a href="/category/новости-хайфы">Хайфа</a><a href="/about">Редакция</a></nav><div class="search">Поиск</div></div></header>{{end}}{{define "home"}}{{template "head" .}}{{template "nav" .}}<main><section class="wrap hero"><div class="hero-copy"><p class="kicker">Новости Хайфы</p><h1>Главные события Хайфы и севера Израиля.</h1><p>Городские новости, безопасность, транспорт, общество, бизнес и культура — всё важное для жителей Хайфы на русском языке.</p></div><div class="radar"><span>חיפה</span><i></i><em>HAIFA NEWSROOM</em></div></section><section class="wrap layout"><div><h2>Лента новостей</h2><div class="grid">{{range .Posts}}<a class="card" href="/article/{{.Slug}}">{{if .FeaturedImage}}<img src="{{.FeaturedImage}}" alt="{{.Title}}">{{else}}<div class="ph">HAIFA.NEWS<span>חיפה</span></div>{{end}}<div><p class="meta">{{if .Category}}{{.Category.Name}}{{end}} · {{.PublishedAt.Format "02.01.2006"}}</p><h3>{{.Title}}</h3><p>{{.Excerpt}}</p><b>читать →</b></div></a>{{end}}</div></div><aside><h3>Редакция</h3><p>Сообщить новость или связаться с редакцией.</p><a href="mailto:newsroom@haifa.news">newsroom@haifa.news</a></aside></section></main><footer><div class="wrap">Haifa.News · Новости Хайфы и севера Израиля</div></footer></div><script type="module" src="/assets/index.js?v=20260525v8"></script></body></html>{{end}}{{define "article"}}{{template "head" .}}{{template "nav" .}}<main class="wrap article-page"><article class="article"><p class="kicker">{{if .Post.Category}}{{.Post.Category.Name}}{{end}} · {{.Post.PublishedAt.Format "02.01.2006"}}</p><h1>{{.Post.Title}}</h1><p class="lead">{{.Post.Excerpt}}</p>{{if .Post.FeaturedImage}}<img class="hero-img" src="{{.Post.FeaturedImage}}" alt="{{.Post.Title}}">{{end}}<script type="application/ld+json">{"@context":"https://schema.org","@type":"NewsArticle","headline":{{printf "%q" .Post.Title}},"description":{{printf "%q" .Post.Excerpt}},"datePublished":"{{.Post.PublishedAt.Format "2006-01-02T15:04:05Z07:00"}}","dateModified":"{{.Post.UpdatedAt.Format "2006-01-02T15:04:05Z07:00"}}","mainEntityOfPage":"{{.Canonical}}","publisher":{"@type":"Organization","name":"Haifa.News"}}</script><div class="body">{{safe .Post.Content}}</div></article><aside><a href="/">← Главная</a><a href="/admin">Редакция</a></aside></main><footer><div class="wrap">Haifa.News · Новости Хайфы и севера Израиля</div></footer></div><script type="module" src="/assets/index.js?v=20260525v8"></script></body></html>{{end}}`))

func qint(r *http.Request, k string, d int) int {
	v, _ := strconv.Atoi(r.URL.Query().Get(k))
	if v == 0 {
		return d
	}
	return v
}
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
func jsonOut(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
func httpError(w http.ResponseWriter, err error) { log.Println(err); http.Error(w, err.Error(), 500) }
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Host, "www.haifa.news") {
			target := "https://haifa.news" + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://static.cloudflareinsights.com https://challenges.cloudflare.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com data:; img-src 'self' data: https:; connect-src 'self'; frame-src https://challenges.cloudflare.com https://us06web.zoom.us; object-src 'none'; base-uri 'self'; frame-ancestors 'self'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		next.ServeHTTP(w, r)
	})
}
