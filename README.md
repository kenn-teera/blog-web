# LearnArai Blog

A minimal, fast, and beautiful blog built with Go.

## Features

- 🌙 **Dark Mode** - Toggle between light and dark themes
- 📱 **Responsive Design** - Works great on mobile and desktop
- 📅 **Post Dates** - Frontmatter support for titles and dates
- 🖼️ **Image Support** - Easily add images to your posts
- ⚡ **Fast** - Lightweight Go server with no JavaScript frameworks

## Tech Stack

- **Backend**: Go (Golang)
- **Router**: net/http (Go 1.22+)
- **Markdown**: [goldmark](https://github.com/yuin/goldmark)
- **Frontmatter**: [yaml.v3](https://gopkg.in/yaml.v3)
- **Styling**: Vanilla CSS with CSS Variables

## Project Structure

```
blog-web/
├── main.go              # Go server
├── posts/               # Markdown blog posts
├── images/              # Post images
├── static/
│   └── style.css        # Styling
└── templates/
    └── base.html        # HTML template
```

## Getting Started

```bash
# Run the server
go run main.go

# Open in browser
open http://localhost:3030
```

## Creating Posts

Create a new `.md` file in the `posts/` folder with frontmatter:

```markdown
---
title: Your Post Title
date: 2026-01-15
---

Your content here...

![Image](/images/your-image.jpg)
```

## License

MIT
