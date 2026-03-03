package sitemap

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"
)

// URL represents a single URL in the sitemap
type URL struct {
	Loc        string     `xml:"loc"`
	LastMod    *time.Time `xml:"lastmod,omitempty"`
	ChangeFreq string     `xml:"changefreq,omitempty"`
	Priority   float32    `xml:"priority,omitempty"`
}

// Sitemap represents the sitemap XML structure
type Sitemap struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr"`
	URLs    []URL    `xml:"url"`
}

// Config holds sitemap configuration
type Config struct {
	BaseURL     string
	StaticURLs  []string
	DynamicURLs func() ([]string, error)
}

// DefaultConfig returns the default sitemap configuration
func DefaultConfig() *Config {
	return &Config{
		BaseURL: "https://goth.local",
		StaticURLs: []string{
			"/",
			"/login",
			"/register",
			"/forgot-password",
			"/dashboard",
			"/profile",
			"/settings",
		},
	}
}

// Generate generates the sitemap XML
func Generate(cfg *Config) ([]byte, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	now := time.Now()
	urls := make([]URL, 0)

	// Add static URLs
	for _, path := range cfg.StaticURLs {
		url := URL{
			Loc:        fmt.Sprintf("%s%s", cfg.BaseURL, path),
			LastMod:    &now,
			ChangeFreq: "weekly",
			Priority:   0.5,
		}

		// Set priority for important pages
		switch path {
		case "/":
			url.Priority = 1.0
			url.ChangeFreq = "daily"
		case "/login", "/register":
			url.Priority = 0.8
		case "/dashboard", "/profile", "/settings":
			url.Priority = 0.6
		}

		urls = append(urls, url)
	}

	// Add dynamic URLs if provided
	if cfg.DynamicURLs != nil {
		dynamicURLs, err := cfg.DynamicURLs()
		if err != nil {
			return nil, fmt.Errorf("failed to get dynamic URLs: %w", err)
		}

		for _, path := range dynamicURLs {
			urls = append(urls, URL{
				Loc:        fmt.Sprintf("%s%s", cfg.BaseURL, path),
				LastMod:    &now,
				ChangeFreq: "weekly",
				Priority:   0.5,
			})
		}
	}

	sitemap := Sitemap{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	return xml.MarshalIndent(sitemap, "", "  ")
}

// Handler creates an HTTP handler for serving the sitemap
func Handler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

		data, err := Generate(cfg)
		if err != nil {
			http.Error(w, "Failed to generate sitemap", http.StatusInternalServerError)
			return
		}

		w.Write(data)
	}
}

// GenerateRobotsTxt generates a robots.txt file
func GenerateRobotsTxt(cfg *Config) string {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	robots := `# robots.txt for ` + cfg.BaseURL + `
User-agent: *
Allow: /
Disallow: /admin/
Disallow: /api/
Disallow: /auth/
Disallow: /checkout/
Disallow: /webhook/

# Sitemap
Sitemap: ` + cfg.BaseURL + `/sitemap.xml

# Crawl-delay (optional)
Crawl-delay: 1

# Google specific
User-agent: Googlebot
Allow: /
Allow: /public/

# Bing specific
User-agent: Bingbot
Allow: /
Allow: /public/

# Block bad bots
User-agent: AhrefsBot
Disallow: /

User-agent: SemrushBot
Disallow: /

User-agent: MJ12bot
Disallow: /
`

	return robots
}

// RobotsHandler creates an HTTP handler for serving robots.txt
func RobotsHandler(cfg *Config) http.HandlerFunc {
	robotsTxt := GenerateRobotsTxt(cfg)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours
		w.Write([]byte(robotsTxt))
	}
}
