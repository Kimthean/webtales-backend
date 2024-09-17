package utils

import "strings"

func IsEnglishSource(url string) bool {
	return strings.Contains(url, "wuxiabox.com") ||
		strings.Contains(url, "lightnovelworld.co") ||
		strings.Contains(url, "wuxiaspot.com") || strings.Contains(url, "fanmtl.com") || strings.Contains(url, "1stkissnovel")
}
