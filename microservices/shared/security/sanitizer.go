package security

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Sanitizer provides HTML sanitization for user input.
type Sanitizer struct {
	// htmlTagPattern matches any HTML tag
	htmlTagPattern *regexp.Regexp
	// scriptPattern matches script tags and their content
	scriptPattern *regexp.Regexp
	// stylePattern matches style tags and their content
	stylePattern *regexp.Regexp
	// eventHandlerPattern matches HTML event handlers (onclick, onerror, etc.)
	eventHandlerPattern *regexp.Regexp
}

func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		// Match actual HTML tags: < followed by letter or /, then any chars until >
		htmlTagPattern:      regexp.MustCompile(`</?[a-zA-Z][^>]*>`),
		scriptPattern:       regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`),
		stylePattern:        regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`),
		eventHandlerPattern: regexp.MustCompile(`(?i)\son\w+\s*=\s*[^>\s]*`),
	}
}

// SanitizeString removes all HTML tags and attributes from a string.
// It uses multiple layers of protection:
// 1. Remove script tags and their content
// 2. Remove style tags and their content
// 3. Remove event handlers (onclick, onerror, etc.)
// 4. Strip all HTML tags
func (s *Sanitizer) SanitizeString(input string) string {
	if input == "" {
		return input
	}

	// Remove script tags and their content
	clean := s.scriptPattern.ReplaceAllString(input, "")

	// Remove style tags and their content
	clean = s.stylePattern.ReplaceAllString(clean, "")

	// Remove event handlers
	clean = s.eventHandlerPattern.ReplaceAllString(clean, "")

	// Strip all HTML tags
	clean = s.htmlTagPattern.ReplaceAllString(clean, "")

	return strings.TrimSpace(clean)
}

// SanitizeMap recursively sanitizes all string values in a map.
func (s *Sanitizer) SanitizeMap(m map[string]interface{}) {
	for key, val := range m {
		switch v := val.(type) {
		case string:
			m[key] = s.SanitizeString(v)
		case map[string]interface{}:
			s.SanitizeMap(v)
		case []interface{}:
			s.SanitizeSlice(v)
		}
	}
}

// SanitizeSlice recursively sanitizes all string values in a slice.
func (s *Sanitizer) SanitizeSlice(arr []interface{}) {
	for i, val := range arr {
		switch v := val.(type) {
		case string:
			arr[i] = s.SanitizeString(v)
		case map[string]interface{}:
			s.SanitizeMap(v)
		case []interface{}:
			s.SanitizeSlice(v)
		}
	}
}

// SanitizeStruct recursively sanitizes all string fields in a struct.
// It marshals the struct to JSON, sanitizes all string values, then unmarshals back.
// This is a simple catch-all approach. For performance-critical paths,
// sanitize individual fields explicitly.
func (s *Sanitizer) SanitizeStruct(v interface{}) error {
	// Marshal struct to JSON
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// Unmarshal to a map so we can walk all values
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// Recursively sanitize all string values
	s.SanitizeMap(m)

	// Marshal back to JSON and unmarshal into the original struct
	cleanData, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(cleanData, v)
}

// TruncateAndSanitize limits string length and sanitizes HTML.
// Useful for fields with known max lengths (e.g., description, bio).
func (s *Sanitizer) TruncateAndSanitize(input string, maxLen int) string {
	clean := s.SanitizeString(input)
	clean = strings.TrimSpace(clean)
	if len(clean) > maxLen {
		clean = clean[:maxLen]
	}
	return clean
}
