package crawler

import (
	"fmt"
	"go-novel/models"
	"go-novel/utils"
	"strings"
	"time"
)

func (c *Crawler) CrawlChapter(chapterURL string, chapterTitle string, chapterNumber int) (*models.Chapter, error) {
	chapter := &models.Chapter{
		Number: chapterNumber,
		Title:  chapterTitle,
		URL:    chapterURL,
	}

	content, err := c.crawlChapterContent(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("crawling chapter content: %w", err)
	}

	if utils.IsEnglishSource(chapterURL) {
		chapter.TranslatedContent = &content
	} else {
		chapter.Content = &content
	}

	return chapter, nil
}

func (c *Crawler) crawlChapterContent(pageURL string) (string, error) {
	var contentBuilder strings.Builder
	var err error

	switch {
	case strings.Contains(pageURL, "9999txt.cc"):
		err = c.crawl9999txtChapterContent(pageURL, &contentBuilder)
	case strings.Contains(pageURL, "uukanshu.cc"):
		err = c.crawlUukanshuChapterContent(pageURL, &contentBuilder)
	case strings.Contains(pageURL, "wuxiabox.com"):
		err = c.crawlWuxiaboxChapterContent(pageURL, &contentBuilder)
	case strings.Contains(pageURL, "wuxiaspot.com"):
		err = c.crawlWuxiaboxChapterContent(pageURL, &contentBuilder)
	case strings.Contains(pageURL, "fanmtl.com"):
		err = c.crawlWuxiaboxChapterContent(pageURL, &contentBuilder)
	case strings.Contains(pageURL, "lightnovelworld.co"):
		err = c.crawlLightNovelWorldChapterContent(pageURL, &contentBuilder)
	case strings.Contains(pageURL, "69shu.me"):
		err = c.crawl69shuChapterContent(pageURL, &contentBuilder)
	default:
		return "", fmt.Errorf("unsupported URL for chapter content: %s", pageURL)
	}

	if err != nil {
		return "", fmt.Errorf("crawling chapter content: %w", err)
	}

	return contentBuilder.String(), nil
}

func (c *Crawler) crawl9999txtChapterContent(pageURL string, contentBuilder *strings.Builder) error {
	page, err := c.newPage()
	if err != nil {
		return fmt.Errorf("creating new page: %w", err)
	}
	defer page.Close()

	err = page.Navigate(pageURL)
	if err != nil {
		return fmt.Errorf("navigating to URL: %w", err)
	}

	page.MustWaitLoad()

	contentElement, err := page.Element("#content")
	if err != nil {
		return fmt.Errorf("finding content element: %w", err)
	}

	paragraphs, err := contentElement.Elements("p")
	if err != nil {
		return fmt.Errorf("finding paragraph elements: %w", err)
	}

	for i, p := range paragraphs {
		text := strings.TrimSpace(p.MustText())
		if text != "" {
			normalizedText := strings.ReplaceAll(text, " ", "")
			normalizedText = strings.ReplaceAll(normalizedText, "，", "")
			normalizedText = strings.ReplaceAll(normalizedText, "。", "")
			if i == len(paragraphs)-1 && normalizedText == "本章未完点击下一页继续阅读" {
				continue
			}
			if i > 0 {
				contentBuilder.WriteString("\n\n")
			}
			contentBuilder.WriteString(text)
		}
	}

	nextPageElement, err := page.Element(".bottem2 a[rel='next']")
	if err == nil && nextPageElement != nil {
		nextPageURL := page.MustInfo().URL + *nextPageElement.MustAttribute("href")
		nextPageText := strings.TrimSpace(nextPageElement.MustText())
		if nextPageURL != pageURL && nextPageText != "下一章" && !strings.Contains(nextPageURL, "javascript:void(0);") {
			err = c.crawl9999txtChapterContent(nextPageURL, contentBuilder)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Crawler) crawlUukanshuChapterContent(pageURL string, contentBuilder *strings.Builder) error {
	page, err := c.newPage()
	if err != nil {
		return fmt.Errorf("creating new page: %w", err)
	}
	defer page.Close()

	err = page.Navigate(pageURL)
	if err != nil {
		return fmt.Errorf("navigating to URL: %w", err)
	}

	page.MustWaitLoad()

	contentElement, err := page.Element(".book.read p.readcotent")
	if err != nil {
		return fmt.Errorf("finding content element: %w", err)
	}

	contentHtml, err := contentElement.HTML()
	if err != nil {
		return fmt.Errorf("extracting HTML content: %w", err)
	}

	contentWithLineBreaks := strings.ReplaceAll(contentHtml, "<br/>", "\n")
	lines := strings.Split(contentWithLineBreaks, "\n")
	for i, line := range lines {
		line = strings.ReplaceAll(line, "\u00A0", " ")
		line = strings.TrimSpace(line)
		lines[i] = line
	}
	sanitizedContent := strings.Join(lines, "\n")

	contentBuilder.WriteString(sanitizedContent)

	return nil
}

func (c *Crawler) crawlWuxiaboxChapterContent(pageURL string, contentBuilder *strings.Builder) error {
	page, err := c.newPage()
	if err != nil {
		return fmt.Errorf("creating new page: %w", err)
	}
	defer page.Close()

	time.Sleep(4 * time.Second)

	err = page.Navigate(pageURL)
	if err != nil {
		return fmt.Errorf("navigating to URL: %w", err)
	}

	page.MustWaitLoad()

	contentElement, err := page.Element(".chapter-content")
	if err != nil {
		return fmt.Errorf("finding content element: %w", err)
	}

	paragraphs, err := contentElement.Elements("p")
	if err != nil {
		return fmt.Errorf("finding paragraph elements: %w", err)
	}

	for _, p := range paragraphs {
		text := strings.TrimSpace(p.MustText())
		if text != "" {
			contentBuilder.WriteString(text + "\n\n")
		}
	}

	content := contentBuilder.String()
	content = strings.ReplaceAll(content, "&ZeroWidthSpace;", "")

	contentBuilder.Reset()
	contentBuilder.WriteString(content)

	return nil
}

func (c *Crawler) crawlLightNovelWorldChapterContent(pageURL string, contentBuilder *strings.Builder) error {
	page, err := c.newPage()
	if err != nil {
		return fmt.Errorf("creating new page: %w", err)
	}
	defer page.Close()

	err = page.Navigate(pageURL)
	if err != nil {
		return fmt.Errorf("navigating to URL: %w", err)
	}

	page.MustWaitLoad()

	paragraphs, err := page.Elements(".chapter-content p")
	if err != nil {
		return fmt.Errorf("finding paragraph elements: %w", err)
	}

	for _, p := range paragraphs {
		text := strings.TrimSpace(p.MustText())
		if text != "" {
			contentBuilder.WriteString(text + "\n\n")
		}
	}

	return nil
}

func (c *Crawler) crawl69shuChapterContent(pageURL string, contentBuilder *strings.Builder) error {
	page, err := c.newPage()
	if err != nil {
		return fmt.Errorf("creating new page: %w", err)
	}
	defer page.Close()

	err = page.Navigate(pageURL)
	if err != nil {
		return fmt.Errorf("navigating to URL: %w", err)
	}

	page.MustWaitLoad()

	contentElement, err := page.Element("#content")
	if err != nil {
		return fmt.Errorf("finding content element: %w", err)
	}

	content := contentElement.MustText()
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			contentBuilder.WriteString(trimmedLine + "\n\n")
		}
	}

	return nil
}
