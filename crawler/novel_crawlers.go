package crawler

import (
	"fmt"
	"go-novel/models"
	"log"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

func (c *Crawler) CrawlNovel(url string) (*models.Novel, error) {
	log.Printf("Crawling novel from %s", url)

	var novel *models.Novel
	var err error

	switch {
	case strings.Contains(url, "9999txt.cc"):
		novel, err = c.crawl9999txt(url)
	case strings.Contains(url, "uukanshu.cc"):
		novel, err = c.crawlUukanshu(url)
	case strings.Contains(url, "wuxiabox.com"):
		novel, err = c.crawlWuxiabox(url)
	case strings.Contains(url, "wuxiaspot.com"):
		novel, err = c.crawlWuxiaspot(url)
	case strings.Contains(url, "fanmtl.com"):
		novel, err = c.crawlWuxiabox(url)
	case strings.Contains(url, "lightnovelworld.co"):
		novel, err = c.crawlLightNovelWorld(url)
	case strings.Contains(url, "69shu.me"):
		novel, err = c.crawl69Shu(url)
	default:
		return nil, fmt.Errorf("unsupported URL: %s", url)
	}

	if err != nil {
		return nil, fmt.Errorf("crawling novel: %w", err)
	}

	return novel, nil
}

func (c *Crawler) crawl9999txt(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}
	collector := c.newCollector()

	collector.OnHTML("#info h1", func(e *colly.HTMLElement) {
		title := e.Text
		novel.Title = &title
	})

	collector.OnHTML("#fmimg img", func(e *colly.HTMLElement) {
		imageURL := e.Attr("data-original")
		novel.Thumbnail = &imageURL
	})

	collector.OnHTML("#info > p:first-of-type a", func(e *colly.HTMLElement) {
		author := e.Text
		novel.Author = &author
	})

	collector.OnHTML("#intro", func(e *colly.HTMLElement) {
		introDescription := e.Text
		novel.Description = &introDescription
	})

	collector.OnHTML(".readbtn .chapterlist", func(e *colly.HTMLElement) {
		chapterListURL := e.Request.AbsoluteURL(e.Attr("href"))
		chapters, err := c.extractChapters(chapterListURL)
		if err != nil {
			log.Printf("Error extracting chapters: %s", err)
		}
		novel.Chapters = chapters
	})

	err := collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("visiting novel page: %w", err)
	}

	return novel, nil
}

func (c *Crawler) crawlUukanshu(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}
	collector := c.newCollector()

	collector.OnHTML(".thumbnail", func(e *colly.HTMLElement) {
		imageURL := e.Request.AbsoluteURL(e.Attr("src"))
		novel.Thumbnail = &imageURL
	})

	collector.OnHTML(".booktitle", func(e *colly.HTMLElement) {
		bookTitle := e.Text
		novel.Title = &bookTitle
	})

	collector.OnHTML(".bookintro", func(e *colly.HTMLElement) {
		bookIntro := e.Text
		novel.Description = &bookIntro
	})

	collector.OnHTML(".booktag", func(e *colly.HTMLElement) {
		authorWithPrefix := e.ChildText("a.red")
		author := strings.Replace(authorWithPrefix, "作者：", "", -1)
		novel.Author = &author
	})

	var chapters []models.Chapter
	chapterCounter := 0
	collector.OnHTML(".book.chapterlist dd a", func(e *colly.HTMLElement) {
		chapterURL := e.Request.AbsoluteURL(e.Attr("href"))
		chapterTitle := e.Text

		chapterCounter++

		chapters = append(chapters, models.Chapter{
			URL:    chapterURL,
			Title:  chapterTitle,
			Number: chapterCounter,
		})
	})

	err := collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("visiting novel page: %w", err)
	}
	novel.Chapters = chapters

	return novel, nil
}

func (c *Crawler) crawlWuxiabox(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}
	collector := c.newCollector()

	collector.OnHTML(".novel-header", func(e *colly.HTMLElement) {
		title := e.ChildText(".novel-title")
		novel.Title = &title

		altTitle := e.ChildText(".alternative-title")
		novel.RawTitle = &altTitle

		author := e.ChildText(".author span[itemprop='author']")
		novel.Author = &author

		imgSrc := e.ChildAttr(".fixed-img img", "src")
		if imgSrc == "/static/picture/placeholder-158.jpg" {
			imgSrc = e.ChildAttr(".fixed-img img", "data-src")
		}
		imageURL := e.Request.AbsoluteURL(imgSrc)
		novel.Thumbnail = &imageURL
	})

	collector.OnHTML("#info", func(e *colly.HTMLElement) {
		var paragraphs []string

		paragraphElements := e.DOM.Find(".summary .content p")
		if paragraphElements.Length() > 1 {
			paragraphElements.Each(func(_ int, s *goquery.Selection) {
				trimmedText := strings.TrimSpace(s.Text())
				if trimmedText != "" {
					paragraphs = append(paragraphs, trimmedText)
				}
			})
		} else {
			content := e.ChildText(".summary .content p")
			content = strings.ReplaceAll(content, "<br>", "\n")
			content = strings.ReplaceAll(content, "<br/>", "\n")
			for _, paragraph := range strings.Split(content, "\n") {
				trimmedParagraph := strings.TrimSpace(paragraph)
				if trimmedParagraph != "" {
					paragraphs = append(paragraphs, trimmedParagraph)
				}
			}
		}

		detailedSummary := strings.Join(paragraphs, "\n\n")
		novel.Description = &detailedSummary
	})

	err := collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("visiting novel page: %w", err)
	}

	chapters, err := c.extractChapters(url)
	if err != nil {
		log.Printf("Error extracting chapters: %s", err)
	}
	novel.Chapters = chapters

	return novel, nil
}

func (c *Crawler) crawlWuxiaspot(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}
	collector := c.newCollector()

	collector.OnHTML(".novel-header", func(e *colly.HTMLElement) {
		title := e.ChildText(".novel-title")
		novel.Title = &title

		altTitle := e.ChildText(".alternative-title")
		novel.RawTitle = &altTitle

		author := e.ChildText("span[itemprop='author'] a")
		novel.Author = &author

		imgSrc := e.ChildAttr(".fixed-img img", "src")
		if imgSrc == "/static/picture/placeholder-158.jpg" {
			imgSrc = e.ChildAttr(".fixed-img img", "data-src")
		}
		imageURL := e.Request.AbsoluteURL(imgSrc)
		novel.Thumbnail = &imageURL
	})

	collector.OnHTML("#info", func(e *colly.HTMLElement) {
		var paragraphs []string

		paragraphElements := e.DOM.Find(".summary .content p")
		if paragraphElements.Length() > 1 {
			paragraphElements.Each(func(_ int, s *goquery.Selection) {
				trimmedText := strings.TrimSpace(s.Text())
				if trimmedText != "" {
					paragraphs = append(paragraphs, trimmedText)
				}
			})
		} else {
			content := e.ChildText(".summary .content p")
			content = strings.ReplaceAll(content, "<br>", "\n\n")
			content = strings.ReplaceAll(content, "<br/>", "\n\n")
			for _, paragraph := range strings.Split(content, "\n\n") {
				trimmedParagraph := strings.TrimSpace(paragraph)
				if trimmedParagraph != "" {
					paragraphs = append(paragraphs, trimmedParagraph)
				}
			}
		}

		detailedSummary := strings.Join(paragraphs, "\n\n\n")
		novel.Description = &detailedSummary
	})

	err := collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("visiting novel page: %w", err)
	}

	chapters, err := c.extractChapters(url)
	if err != nil {
		log.Printf("Error extracting chapters: %s", err)
	}
	novel.Chapters = chapters

	return novel, nil
}

func (c *Crawler) crawlLightNovelWorld(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}
	collector := c.newCollector()

	collector.OnHTML(".novel-title", func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.Text)
		novel.Title = &title
	})

	collector.OnHTML(".alternative-title", func(e *colly.HTMLElement) {
		altTitle := strings.TrimSpace(e.Text)
		novel.RawTitle = &altTitle
	})

	collector.OnHTML(".property-item span[itemprop='author']", func(e *colly.HTMLElement) {
		author := strings.TrimSpace(e.Text)
		novel.Author = &author
	})

	collector.OnHTML(".fixed-img img", func(e *colly.HTMLElement) {
		coverImageURL := e.Request.AbsoluteURL(e.Attr("src"))
		if strings.HasPrefix(coverImageURL, "data:image") {
			coverImageURL = e.Attr("data-src")
		}
		novel.Thumbnail = &coverImageURL
	})

	collector.OnHTML(".summary .content", func(e *colly.HTMLElement) {
		description := ""
		e.ForEach("p", func(_ int, el *colly.HTMLElement) {
			description += el.Text + "\n\n\n"
		})
		novel.Description = &description
	})

	collector.OnHTML("a.chapter-latest-container", func(e *colly.HTMLElement) {
		chapterListURL := e.Request.AbsoluteURL(e.Attr("href"))
		chapters, err := c.extractChapters(chapterListURL)
		if err != nil {
			log.Printf("Error extracting chapters: %s", err)
		}
		novel.Chapters = chapters
	})

	err := collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("visiting novel page: %w", err)
	}

	return novel, nil
}

func (c *Crawler) crawl69Shu(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}
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

	collector.OnHTML(".bookbox", func(e *colly.HTMLElement) {
		title := e.ChildText("h1 a")
		novel.Title = &title
		log.Printf("Title: %s", title)

		author := e.ChildText("p:contains('作者：') a")
		novel.Author = &author
		log.Printf("Author: %s", author)

		coverImageURL := e.Request.AbsoluteURL(e.ChildAttr(".bookimg2 img", "src"))
		novel.Thumbnail = &coverImageURL
	})
	collector.OnHTML(".jianjie-popup .content", func(e *colly.HTMLElement) {
		description := ""

		e.ForEach("p", func(_ int, el *colly.HTMLElement) {
			description += el.Text + "\n\n\n"
		})

		if description == "" {
			e.ForEach("p", func(_ int, el *colly.HTMLElement) {
				description = strings.ReplaceAll(el.Text, "<br>", "\n\n\n")
			})
		}

		novel.Description = &description
		log.Printf("Description: %s", description)
	})

	collector.OnHTML(".addbtn a.btn[href*='/book/']", func(e *colly.HTMLElement) {
		chapterPageURL := e.Request.AbsoluteURL(e.Attr("href"))
		chapters, err := c.extractChapters(chapterPageURL)
		if err != nil {
			log.Printf("Error extracting chapters: %s", err)
		}
		novel.Chapters = chapters
	})

	err := collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("visiting novel page: %w", err)
	}

	return novel, nil
}
