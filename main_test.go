package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockSlugReader implements SlugReader for testing
type MockSlugReader struct {
	content map[string]string
}

func (m *MockSlugReader) Read(slug string) (string, error) {
	if content, ok := m.content[slug]; ok {
		return content, nil
	}
	return "", os.ErrNotExist
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		expectedTitle   string
		expectedDate    string
		expectedContent string
	}{
		{
			name: "valid frontmatter",
			content: `---
title: Test Post
date: 2026-01-15
---

This is the content.`,
			expectedTitle:   "Test Post",
			expectedDate:    "2026-01-15",
			expectedContent: "This is the content.",
		},
		{
			name:            "no frontmatter",
			content:         "Just content without frontmatter.",
			expectedTitle:   "",
			expectedDate:    "",
			expectedContent: "Just content without frontmatter.",
		},
		{
			name: "only title",
			content: `---
title: Only Title
---

Content here.`,
			expectedTitle:   "Only Title",
			expectedDate:    "",
			expectedContent: "Content here.",
		},
		{
			name: "incomplete frontmatter missing closing",
			content: `---
title: Incomplete
This should not parse`,
			expectedTitle:   "",
			expectedDate:    "",
			expectedContent: "---\ntitle: Incomplete\nThis should not parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, content := ParseFrontmatter(tt.content)

			if fm.Title != tt.expectedTitle {
				t.Errorf("expected title %q, got %q", tt.expectedTitle, fm.Title)
			}
			if fm.Date != tt.expectedDate {
				t.Errorf("expected date %q, got %q", tt.expectedDate, fm.Date)
			}
			if strings.TrimSpace(content) != tt.expectedContent {
				t.Errorf("expected content %q, got %q", tt.expectedContent, strings.TrimSpace(content))
			}
		})
	}
}

func TestIsValidSlug(t *testing.T) {
	tests := []struct {
		slug  string
		valid bool
	}{
		{"valid-slug", true},
		{"valid_slug", true},
		{"ValidSlug123", true},
		{"th-getting-started", true},
		{"en-my-post", true},
		{"../etc/passwd", false},
		{"path/traversal", false},
		{"slug with spaces", false},
		{"", false},
		{"slug<script>", false},
		{"slug&param=value", false},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			result := IsValidSlug(tt.slug)
			if result != tt.valid {
				t.Errorf("IsValidSlug(%q) = %v, want %v", tt.slug, result, tt.valid)
			}
		})
	}
}

func TestPostHandler_ValidPost(t *testing.T) {
	mockReader := &MockSlugReader{
		content: map[string]string{
			"test-post": `---
title: Test Post
date: 2026-01-15
---

# Hello World

This is a test post.`,
		},
	}

	handler := PostHandler(mockReader)

	req := httptest.NewRequest("GET", "/posts/test-post", nil)
	req.SetPathValue("slug", "test-post")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Check security headers
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
}

func TestPostHandler_InvalidSlug(t *testing.T) {
	mockReader := &MockSlugReader{content: map[string]string{}}

	handler := PostHandler(mockReader)

	// Test path traversal attempt
	req := httptest.NewRequest("GET", "/posts/../etc/passwd", nil)
	req.SetPathValue("slug", "../etc/passwd")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid slug, got %d", w.Code)
	}
}

func TestPostHandler_NotFound(t *testing.T) {
	mockReader := &MockSlugReader{content: map[string]string{}}

	handler := PostHandler(mockReader)

	req := httptest.NewRequest("GET", "/posts/nonexistent", nil)
	req.SetPathValue("slug", "nonexistent")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHomeHandler_SecurityHeaders(t *testing.T) {
	// Create a temporary posts directory for testing
	tmpDir := t.TempDir()
	postsDir := filepath.Join(tmpDir, "posts")
	os.MkdirAll(postsDir, 0755)

	// Save original working directory and change to temp
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create templates directory
	os.MkdirAll("templates", 0755)
	os.WriteFile("templates/base.html", []byte(`<!DOCTYPE html><html><head><title>{{.Title}}</title></head><body>{{.Content}}</body></html>`), 0644)

	// Re-initialize template for test
	testTmpl, _ := tmpl.ParseFiles("templates/base.html")
	_ = testTmpl

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	HomeHandler(w, req)

	// Check security headers are set
	if w.Header().Get("X-XSS-Protection") != "1; mode=block" {
		t.Error("missing X-XSS-Protection header")
	}
}

func TestContactHandler_LanguageSwitch(t *testing.T) {
	tests := []struct {
		lang         string
		expectedText string
	}{
		{"th", "ธีรภัทร ยาใจ"},
		{"en", "Teerapat Yajai"},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/contact?lang="+tt.lang, nil)
			w := httptest.NewRecorder()

			ContactHandler(w, req)

			body := w.Body.String()
			if !strings.Contains(body, tt.expectedText) {
				t.Errorf("expected body to contain %q for lang=%s", tt.expectedText, tt.lang)
			}
		})
	}
}
