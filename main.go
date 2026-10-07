package main

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/yuin/goldmark"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// Post represents a blog post with metadata
type Post struct {
	Slug    string
	Title   string
	Date    time.Time
	DateStr string
}

// PostFrontmatter represents the YAML frontmatter in posts
type PostFrontmatter struct {
	Title string `yaml:"title"`
	Date  string `yaml:"date"`
}

// PageData holds data for HTML templates
type PageData struct {
	Title   string
	Lang    string
	Content template.HTML
}

// Cached templates for performance
var (
	tmpl        *template.Template
	contactTmpl *template.Template
)

// contentSecurityPolicy only allows scripts and styles from this site, plus
// Google Fonts for the Sarabun font
const contentSecurityPolicy = "default-src 'self'; " +
	"style-src 'self' https://fonts.googleapis.com; " +
	"font-src https://fonts.gstatic.com; " +
	"img-src 'self' data:; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

// Slug validation regex - only allow alphanumeric, hyphens, and underscores
var validSlugRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func init() {
	var err error
	tmpl, err = template.ParseFiles("templates/base.html")
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}
	contactTmpl, err = template.ParseFiles("templates/contact.html")
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}
}

func main() {
	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "3030"
	}

	mux := http.NewServeMux()

	// Serve static files (CSS, JS)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Serve images
	mux.Handle("GET /images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))

	// Homepage - list all posts
	mux.HandleFunc("GET /{$}", HomeHandler)

	// Contact page
	mux.HandleFunc("GET /contact", ContactHandler)

	// Individual post
	mux.HandleFunc("GET /posts/{slug}", PostHandler(&FileReader{}))

	// Configure server with timeouts for production
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown handling
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down gracefully...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	log.Printf("Blog running at http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

type SlugReader interface {
	Read(slug string) (string, error)
}

type FileReader struct{}

func (fr *FileReader) Read(slug string) (string, error) {
	f, err := os.Open(filepath.Join("posts", slug+".md"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ParseFrontmatter extracts YAML frontmatter from markdown content
func ParseFrontmatter(content string) (PostFrontmatter, string) {
	var fm PostFrontmatter

	// Check if content starts with ---
	if !strings.HasPrefix(content, "---") {
		return fm, content
	}

	// Find the closing ---
	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return fm, content
	}

	// Parse YAML
	if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
		log.Printf("Warning: Failed to parse frontmatter: %v", err)
	}

	// Return the content after frontmatter
	return fm, strings.TrimSpace(parts[1])
}

// setSecurityHeaders adds security headers to all responses
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

// toTitleCase converts a string to title case
func toTitleCase(s string) string {
	caser := cases.Title(language.English)
	return caser.String(s)
}

// titleFromSlug builds a display title from a slug, dropping any language prefix
func titleFromSlug(slug string) string {
	if strings.HasPrefix(slug, "th-") || strings.HasPrefix(slug, "en-") {
		slug = slug[3:]
	}
	return toTitleCase(strings.ReplaceAll(slug, "-", " "))
}

// getLang returns the requested language from the query param or cookie,
// defaulting to "th"
func getLang(r *http.Request) string {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		if cookie, err := r.Cookie("lang"); err == nil {
			lang = cookie.Value
		}
	}
	if lang != "en" && lang != "th" {
		lang = "th"
	}
	return lang
}

// setLangCookie remembers the language of the page being shown
func setLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   31536000, // 1 year
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// IsValidSlug checks if a slug contains only valid characters
func IsValidSlug(slug string) bool {
	return validSlugRegex.MatchString(slug)
}

// ContactHandler displays the contact/about page
func ContactHandler(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)

	lang := getLang(r)
	setLangCookie(w, lang)

	var content bytes.Buffer
	if err := contactTmpl.Execute(&content, lang); err != nil {
		log.Printf("Error rendering contact page: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}

	renderPage(w, lang, "Contact", template.HTML(content.String()))
}

// HomeHandler lists all blog posts
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)

	lang := getLang(r)
	setLangCookie(w, lang)

	files, err := os.ReadDir("posts")
	if err != nil {
		log.Printf("Error reading posts directory: %v", err)
		http.Error(w, "Could not read posts", http.StatusInternalServerError)
		return
	}

	var posts []Post
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".md") {
			slug := strings.TrimSuffix(f.Name(), ".md")

			// Filter by language prefix (th- or en-)
			// Posts without prefix are shown in all languages
			hasLangPrefix := strings.HasPrefix(slug, "th-") || strings.HasPrefix(slug, "en-")
			if hasLangPrefix {
				expectedPrefix := lang + "-"
				if !strings.HasPrefix(slug, expectedPrefix) {
					continue // Skip posts for other languages
				}
			}

			// Read the post to get frontmatter
			content, err := os.ReadFile(filepath.Join("posts", f.Name()))
			if err != nil {
				log.Printf("Error reading post %s: %v", f.Name(), err)
				continue
			}

			fm, _ := ParseFrontmatter(string(content))

			post := Post{
				Slug: slug,
			}

			// Use frontmatter title or generate from slug
			if fm.Title != "" {
				post.Title = fm.Title
			} else {
				post.Title = titleFromSlug(slug)
			}

			// Parse date from frontmatter or use file modification time
			if fm.Date != "" {
				if t, err := time.Parse("2006-01-02", fm.Date); err == nil {
					post.Date = t
					post.DateStr = t.Format("Jan 2, 2006")
				}
			}
			if post.DateStr == "" {
				info, err := f.Info()
				if err == nil && info != nil {
					post.Date = info.ModTime()
					post.DateStr = info.ModTime().Format("Jan 2, 2006")
				}
			}

			posts = append(posts, post)
		}
	}

	// Sort posts by date (newest first)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	// Translated content based on language
	var welcomeTitle, welcomeText, postsHeading string
	if lang == "th" {
		welcomeTitle = "ยินดีต้อนรับสู่ K-Blog"
		welcomeText = "สวัสดีครับ!! ผมคือคนที่ชอบสร้างสรรค์และเรียนรู้สิ่งต่างๆ นี่คือพื้นที่ส่วนตัวของผมซึ่งเอาไว้สำหรับแชร์ความคิด สิ่งที่ได้เรียนรู้ หรือโปรเจกต์ที่กำลังทำอยู่"
		postsHeading = "บทความ"
	} else {
		welcomeTitle = "Welcome to K-Blog"
		welcomeText = "Hi!! I'm someone who likes to create and learn new things. This is my personal space where I can share ideas or projects I'm currently working on."
		postsHeading = "Posts"
	}

	// Build post list HTML
	var content bytes.Buffer
	content.WriteString("<h1>" + template.HTMLEscapeString(welcomeTitle) + "</h1>\n")
	content.WriteString("<p class=\"about-me\">" + template.HTMLEscapeString(welcomeText) + "</p>\n")
	content.WriteString("<h2 class=\"posts-heading\">" + template.HTMLEscapeString(postsHeading) + "</h2>\n")
	content.WriteString("<ul class=\"post-list\">\n")
	for _, post := range posts {
		content.WriteString("<li>")
		content.WriteString("<a href=\"/posts/" + template.HTMLEscapeString(post.Slug) + "\">" + template.HTMLEscapeString(post.Title) + "</a>")
		content.WriteString("<span class=\"post-date\">" + template.HTMLEscapeString(post.DateStr) + "</span>")
		content.WriteString("</li>\n")
	}
	content.WriteString("</ul>\n")

	renderPage(w, lang, "Home", template.HTML(content.String()))
}

// PostHandler handles individual blog posts
func PostHandler(sl SlugReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)

		slug := r.PathValue("slug")

		// Validate slug to prevent path traversal
		if !IsValidSlug(slug) {
			http.Error(w, "Invalid post slug", http.StatusBadRequest)
			return
		}

		// A th-/en- prefix fixes the post's language; other posts follow
		// the reader's choice
		lang := getLang(r)
		if strings.HasPrefix(slug, "th-") || strings.HasPrefix(slug, "en-") {
			lang = slug[:2]
		}

		postMarkdown, err := sl.Read(slug)
		if err != nil {
			http.Error(w, "Post not found", http.StatusNotFound)
			return
		}

		// Parse frontmatter and get content
		fm, markdownContent := ParseFrontmatter(postMarkdown)

		// Convert markdown to HTML
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(markdownContent), &buf); err != nil {
			log.Printf("Error rendering post %s: %v", slug, err)
			http.Error(w, "Error rendering post", http.StatusInternalServerError)
			return
		}

		// Use frontmatter title or generate from slug
		title := fm.Title
		if title == "" {
			title = titleFromSlug(slug)
		}

		// Build post HTML with date
		var postHTML bytes.Buffer
		postHTML.WriteString("<article>\n")
		postHTML.WriteString("<div class=\"post-header\">\n")
		postHTML.WriteString("<h1>" + template.HTMLEscapeString(title) + "</h1>\n")
		if fm.Date != "" {
			if t, err := time.Parse("2006-01-02", fm.Date); err == nil {
				postHTML.WriteString("<span class=\"post-meta\">" + t.Format("Jan 2, 2006") + "</span>\n")
			}
		}
		postHTML.WriteString("</div>\n")
		postHTML.WriteString(buf.String())
		postHTML.WriteString("</article>")

		setLangCookie(w, lang)
		renderPage(w, lang, title, template.HTML(postHTML.String()))
	}
}

// renderPage renders the base template with content
func renderPage(w http.ResponseWriter, lang, title string, content template.HTML) {
	data := PageData{
		Title:   title,
		Lang:    lang,
		Content: content,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}
