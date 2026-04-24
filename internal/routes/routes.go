package routes

import (
	"net/http"
	"strings"
)

// Route defines a URL pattern and its handler
type Route struct {
	Pattern string
	Handler http.Handler
}

var registry []Route

// Add registers a new route
func Add(pattern string, h http.Handler) {
	registry = append(registry, Route{Pattern: pattern, Handler: h})
}

// MatchResult contains the matched handler and the prefix that was matched
type MatchResult struct {
	Handler http.Handler
	Prefix  string
}

// Match finds a handler for the given path and returns the matched prefix
func Match(path string) *MatchResult {
	for _, r := range registry {
		if matches(path, r.Pattern) {
			return &MatchResult{Handler: r.Handler, Prefix: r.Pattern}
		}
	}
	return nil
}

// matches checks if path matches the pattern
func matches(path, pattern string) bool {
	// Exact match
	if path == pattern {
		return true
	}
	// Prefix match (pattern ends with /)
	if strings.HasSuffix(pattern, "/") && strings.HasPrefix(path, pattern) {
		return true
	}
	// Match without trailing slash
	if path == strings.TrimSuffix(pattern, "/") {
		return true
	}
	return false
}

// Reset clears all routes (useful for testing)
func Reset() {
	registry = nil
}
