package utils

import (
	"regexp"
	"strings"
)

func Slugify(title string) string {

	title = strings.TrimSpace(title)

	title = strings.ToLower(title)

	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, "_")

	re := regexp.MustCompile(`[^a-z0-9_]+`)
	title = re.ReplaceAllString(title, "")

	title = strings.Trim(title, "_")

	return title
}
