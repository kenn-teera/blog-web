package main

import (
	"bytes"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
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
	Content template.HTML
}

func main() {
	mux := http.NewServeMux()

	// Serve static files (CSS, JS)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Serve images
	mux.Handle("GET /images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))

	// Homepage - list all posts
	mux.HandleFunc("GET /", HomeHandler)

	// Individual post
	mux.HandleFunc("GET /posts/{slug}", PostHandler(&FileReader{}))

	log.Println("🚀 Blog running at http://localhost:3030")
	err := http.ListenAndServe(":3030", mux)
	if err != nil {
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

// parseFrontmatter extracts YAML frontmatter from markdown content
func parseFrontmatter(content string) (PostFrontmatter, string) {
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
	yaml.Unmarshal([]byte(parts[0]), &fm)

	// Return the content after frontmatter
	return fm, strings.TrimSpace(parts[1])
}

// HomeHandler lists all blog posts
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	// Get language from query param or cookie, default to "th"
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		if cookie, err := r.Cookie("lang"); err == nil {
			lang = cookie.Value
		}
	}
	if lang != "en" && lang != "th" {
		lang = "th"
	}

	// Set language cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   31536000, // 1 year
		HttpOnly: false,
	})

	files, err := os.ReadDir("posts")
	if err != nil {
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
				continue
			}

			fm, _ := parseFrontmatter(string(content))

			post := Post{
				Slug: slug,
			}

			// Use frontmatter title or generate from slug
			if fm.Title != "" {
				post.Title = fm.Title
			} else {
				// Remove language prefix for display
				displaySlug := slug
				if strings.HasPrefix(slug, "th-") || strings.HasPrefix(slug, "en-") {
					displaySlug = slug[3:]
				}
				post.Title = strings.Title(strings.ReplaceAll(displaySlug, "-", " "))
			}

			// Parse date from frontmatter or use file modification time
			if fm.Date != "" {
				if t, err := time.Parse("2006-01-02", fm.Date); err == nil {
					post.Date = t
					post.DateStr = t.Format("Jan 2, 2006")
				}
			}
			if post.DateStr == "" {
				info, _ := f.Info()
				if info != nil {
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
		welcomeTitle = "ยินดีต้อนรับสู่บล็อกของฉัน"
		welcomeText = "สวัสดี! ฉันเป็นนักพัฒนาที่ชอบสร้างสิ่งต่างๆ ด้วย Go นี่คือพื้นที่ส่วนตัวของฉันสำหรับแชร์ความคิด บทเรียน และโปรเจกต์"
		postsHeading = "บทความ"
	} else {
		welcomeTitle = "Welcome to My Blog"
		welcomeText = "Hi! I'm a developer who loves building things with Go. This is my personal space to share thoughts, tutorials, and projects."
		postsHeading = "Posts"
	}

	// Build post list HTML
	var content bytes.Buffer
	content.WriteString("<h1>" + welcomeTitle + "</h1>\n")
	content.WriteString("<p class=\"about-me\">" + welcomeText + "</p>\n")
	content.WriteString("<h2 class=\"posts-heading\">" + postsHeading + "</h2>\n")
	content.WriteString("<ul class=\"post-list\">\n")
	for _, post := range posts {
		content.WriteString("<li>")
		content.WriteString("<a href=\"/posts/" + post.Slug + "\">" + post.Title + "</a>")
		content.WriteString("<span class=\"post-date\">" + post.DateStr + "</span>")
		content.WriteString("</li>\n")
	}
	content.WriteString("</ul>\n")

	renderPage(w, "Home", template.HTML(content.String()))
}

// PostHandler handles individual blog posts
func PostHandler(sl SlugReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		postMarkdown, err := sl.Read(slug)
		if err != nil {
			http.Error(w, "Post not found", http.StatusNotFound)
			return
		}

		// Parse frontmatter and get content
		fm, markdownContent := parseFrontmatter(postMarkdown)

		// Convert markdown to HTML
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(markdownContent), &buf); err != nil {
			http.Error(w, "Error rendering post", http.StatusInternalServerError)
			return
		}

		// Use frontmatter title or generate from slug
		title := fm.Title
		if title == "" {
			title = strings.Title(strings.ReplaceAll(slug, "-", " "))
		}

		// Build post HTML with date
		var postHTML bytes.Buffer
		postHTML.WriteString("<article>\n")
		postHTML.WriteString("<div class=\"post-header\">\n")
		postHTML.WriteString("<h1>" + title + "</h1>\n")
		if fm.Date != "" {
			if t, err := time.Parse("2006-01-02", fm.Date); err == nil {
				postHTML.WriteString("<span class=\"post-meta\">" + t.Format("Jan 2, 2006") + "</span>\n")
			}
		}
		postHTML.WriteString("</div>\n")
		postHTML.WriteString(buf.String())
		postHTML.WriteString("</article>")

		renderPage(w, title, template.HTML(postHTML.String()))
	}
}

// renderPage renders the base template with content
func renderPage(w http.ResponseWriter, title string, content template.HTML) {
	tmpl, err := template.ParseFiles("templates/base.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	data := PageData{
		Title:   title,
		Content: content,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}
