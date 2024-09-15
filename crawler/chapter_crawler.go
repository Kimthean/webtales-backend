// File: chapter_crawlers.go

package crawler

import (
	"fmt"
	"go-novel/models"
	"go-novel/utils"
	"log"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
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
	collector := c.newCollector()
	var paragraphs []string
	var nextPageURL string
	var nextPageText string

	collector.OnHTML("#content", func(e *colly.HTMLElement) {
		e.ForEach("p", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text != "" {
				paragraphs = append(paragraphs, text)
			}
		})
	})

	collector.OnHTML(".bottem2 a[rel='next']", func(e *colly.HTMLElement) {
		nextPageURL = e.Request.AbsoluteURL(e.Attr("href"))
		nextPageText = strings.TrimSpace(e.Text)
	})

	err := collector.Visit(pageURL)
	if err != nil {
		return fmt.Errorf("visiting chapter page: %w", err)
	}

	for i, text := range paragraphs {
		trimmedText := strings.TrimSpace(text)
		normalizedText := strings.ReplaceAll(trimmedText, " ", "")
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

	if nextPageURL != "" && nextPageURL != pageURL && nextPageText != "下一章" && !strings.Contains(nextPageURL, "javascript:void(0);") {
		err = c.crawl9999txtChapterContent(nextPageURL, contentBuilder)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Crawler) crawlUukanshuChapterContent(pageURL string, contentBuilder *strings.Builder) error {
	collector := c.newCollector()

	collector.OnHTML(".book.read", func(e *colly.HTMLElement) {
		contentSelection := e.DOM.Find("p.readcotent")

		contentHtml, err := contentSelection.Html()
		if err != nil {
			log.Printf("Failed to extract HTML content: %v", err)
			return
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
	})

	err := collector.Visit(pageURL)
	if err != nil {
		return fmt.Errorf("visiting chapter page: %w", err)
	}

	return nil
}

func (c *Crawler) crawlWuxiaboxChapterContent(pageURL string, contentBuilder *strings.Builder) error {
	collector := c.newCollector()

	collector.OnRequest(func(r *colly.Request) {
		time.Sleep(4 * time.Second)
	})

	collector.OnHTML(".chapter-content", func(e *colly.HTMLElement) {
		processContent := func(text string) {
			text = strings.TrimSpace(text)
			if text != "" {
				contentBuilder.WriteString(text + "\n\n")
			}
		}

		e.ForEach("p", func(_ int, el *colly.HTMLElement) {
			processContent(el.Text)
		})

		e.DOM.Contents().Each(func(_ int, s *goquery.Selection) {
			if goquery.NodeName(s) == "#text" {
				processContent(s.Text())
			}
		})

		content := contentBuilder.String()
		content = strings.ReplaceAll(content, "&ZeroWidthSpace;", "")

		contentBuilder.Reset()
		contentBuilder.WriteString(content)
	})

	err := collector.Visit(pageURL)
	if err != nil {
		return fmt.Errorf("visiting chapter page: %w", err)
	}

	return nil
}

func (c *Crawler) crawlLightNovelWorldChapterContent(pageURL string, contentBuilder *strings.Builder) error {
	collector := c.newCollector()

	collector.OnHTML(".chapter-content p", func(e *colly.HTMLElement) {
		content := e.Text + "\n\n"
		contentBuilder.WriteString(content)
	})

	err := collector.Visit(pageURL)
	if err != nil {
		return fmt.Errorf("visiting chapter page: %w", err)
	}

	return nil
}

func (c *Crawler) crawl69shuChapterContent(pageURL string, contentBuilder *strings.Builder) error {
	collector := c.newCollector()

	collector.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept-Charset", "utf-8")
	})
	collector.OnResponse(func(r *colly.Response) {
		utf8Body, err := convertToUTF8(r.Body, r.Headers.Get("Content-Type"))
		if err != nil {
			log.Printf("Error converting response body to UTF-8: %s", err)
			return
		}
		r.Body = utf8Body
	})

	collector.OnHTML(".mybox", func(e *colly.HTMLElement) {
		// First, try to find content within <p> tags
		paragraphs := e.ChildTexts("p")
		if len(paragraphs) > 0 {
			for _, text := range paragraphs {
				text = strings.TrimSpace(text)
				if text != "" {
					contentBuilder.WriteString(text + "\n\n")
				}
			}
		} else {
			// If no <p> tags, extract content directly from .txtnav
			e.ForEach(".txtnav", func(_ int, el *colly.HTMLElement) {
				content := el.Text
				lines := strings.Split(content, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && !strings.Contains(line, "本章完") && !strings.Contains(line, "章节列表") {
						contentBuilder.WriteString(line + "\n\n")
					}
				}
			})
		}
	})

	err := collector.Visit(pageURL)
	if err != nil {
		return fmt.Errorf("visiting chapter page: %w", err)
	}

	return nil
}
