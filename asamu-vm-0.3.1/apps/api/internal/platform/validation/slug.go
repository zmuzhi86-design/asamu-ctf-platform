package validation

import (
	"regexp"
	"strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonSlug.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}
func In(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
