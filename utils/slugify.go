package utils

import (
	"regexp"
	"strings"
)

func Slugify(title string) string {

	title = strings.ToLower(title)

	title = strings.ReplaceAll(title, " ", "_")

	re := regexp.MustCompile(`[^a-z0-9_]+`)
	title = re.ReplaceAllString(title, "")
	return title
}
