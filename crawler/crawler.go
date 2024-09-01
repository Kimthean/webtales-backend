package crawler

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"go-novel/models"
	"go-novel/utils"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"golang.org/x/exp/rand"
	"golang.org/x/net/html/charset"
)

type Crawler struct {
	userAgents []string
}

func NewCrawler() *Crawler {
	return &Crawler{
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.2 Safari/605.1.15",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3",
			"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Mobile Safari/537.36,gzip(gfe)",
			"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Linux; Android 13; SM-S901B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36",
			"Mozilla/5.0 (Linux; Android 13; SM-S901U) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36",
			"Mozilla/5.0 (Linux; Android 13; SM-S908U) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Mobile Safari/537.36",
			"Mozilla/5.0 (Linux; Android 13; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36",
			"Mozilla/5.0 (Linux; Android 12; moto g stylus 5G) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36v",
			"Mozilla/5.0 (Linux; Android 12; Redmi Note 9 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36",
		},
	}
}

func (c *Crawler) newCollector() *colly.Collector {
	collector := colly.NewCollector(
		colly.UserAgent(c.randomUserAgent()),
	)

	collector.SetRequestTimeout(30 * time.Second)
	c.setLimitRules(collector)
	c.configureTransport(collector)
	c.setRequestHeaders(collector)

	return collector
}

func (c *Crawler) setLimitRules(collector *colly.Collector) {
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 20,
		RandomDelay: 1 * time.Second,
	})
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*wuxiabox.com*",
		RandomDelay: 2 * time.Second,
	})
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*wuxiaspot.com*",
		RandomDelay: 2 * time.Second,
	})
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*fanmtl.com*",
		RandomDelay: 2 * time.Second,
	})
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*lightnovelworld.co*",
		RandomDelay: 4 * time.Second,
	})
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*69shuba.cx*",
		Parallelism: 2,
		RandomDelay: 2 * time.Second,
	})
}

func (c *Crawler) configureTransport(collector *colly.Collector) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	collector.WithTransport(transport)
}

func (c *Crawler) setRequestHeaders(collector *colly.Collector) {
	collector.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
		r.Headers.Set("Referer", "https://www.google.com/")
	})
}

func (c *Crawler) randomUserAgent() string {
	return c.userAgents[rand.Intn(len(c.userAgents))]
}

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
		novel, err = c.crawlWuxiaspot(url)
	case strings.Contains(url, "lightnovelworld.co"):
		novel, err = c.crawlLightNovelWorld(url)
	case strings.Contains(url, "69shuba.cx"):
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

func convertToUTF8(body []byte, contentType string) ([]byte, error) {
	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
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

func (c *Crawler) extractChapters(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter
	var err error

	switch {
	case strings.Contains(url, "9999txt.cc"):
		chapters, err = c.extract9999txtChapters(url)
	case strings.Contains(url, "wuxiabox.com"):
		chapters, err = c.extractWuxiaboxChapters(url)
	case strings.Contains(url, "wuxiaspot.com"):
		chapters, err = c.extractWuxiaboxChapters(url)
	case strings.Contains(url, "fanmtl.com"):
		chapters, err = c.extractWuxiaboxChapters(url)
	case strings.Contains(url, "lightnovelworld.co/"):
		chapters, err = c.extractLightNovelWorldChapters(url)
	case strings.Contains(url, "69shuba.cx"):
		chapters, err = c.extract69shuChapter(url)
	default:
		return nil, fmt.Errorf("unsupported URL for chapter extraction: %s", url)
	}

	if err != nil {
		return nil, fmt.Errorf("extracting chapters: %w", err)
	}

	return chapters, nil
}

func (c *Crawler) extract9999txtChapters(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter
	collector := c.newCollector()
	chapterCounter := 0

	collector.OnHTML("div#list a[rel='chapter']", func(e *colly.HTMLElement) {
		chapterURL := e.Request.AbsoluteURL(e.Attr("href"))
		chapterTitle := e.DOM.Find("dd").Text()

		chapterCounter++

		chapters = append(chapters, models.Chapter{
			Number: chapterCounter,
			Title:  chapterTitle,
			URL:    chapterURL,
		})
	})

	err := collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("visiting chapter list: %w", err)
	}

	return chapters, nil
}

func (c *Crawler) extractWuxiaboxChapters(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter
	baseURL := url
	visitedPages := make(map[string]bool)
	chapterCounter := 0

	collector := c.newCollector()

	collector.OnRequest(func(r *colly.Request) {
		time.Sleep(2 * time.Second)
	})

	collector.OnHTML("#chpagedlist", func(e *colly.HTMLElement) {
		currentURL := e.Request.URL.String()

		log.Printf("Extracting chapters from page: %s", currentURL)

		e.ForEach(".chapter-list li", func(_ int, el *colly.HTMLElement) {
			chapterCounter++
			chapterURL := el.ChildAttr("a", "href")
			chapterTitle := el.ChildText(".chapter-title")
			chapters = append(chapters, models.Chapter{
				Number:          chapterCounter,
				TranslatedTitle: &chapterTitle,
				URL:             e.Request.AbsoluteURL(chapterURL),
			})
		})

		// Check for next page
		e.ForEach(".pagination li:not(.active) a[data-ajax='true']", func(_ int, el *colly.HTMLElement) {
			nextPageURL := el.Attr("href")
			if nextPageURL != "" {
				absoluteNextPageURL := e.Request.AbsoluteURL(nextPageURL)
				// Skip the first page, any already visited pages, and the "page=0" link
				if absoluteNextPageURL != baseURL &&
					!visitedPages[absoluteNextPageURL] &&
					!strings.Contains(absoluteNextPageURL, "page=0") {
					log.Printf("Found new page: %s. Queuing visit...", absoluteNextPageURL)
					collector.Visit(absoluteNextPageURL)
				}
			}
		})

		if visitedPages[currentURL] {
			log.Printf("Skipping already visited page: %s", currentURL)
			return
		}
		visitedPages[currentURL] = true
	})

	visitedPages[baseURL] = true

	err := collector.Visit(baseURL)
	if err != nil {
		log.Printf("Error visiting initial URL: %s", err)
		return nil, fmt.Errorf("visiting initial URL: %w", err)
	}

	return chapters, nil
}

func (c *Crawler) extractLightNovelWorldChapters(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter
	chapterCounter := 0

	collector := c.newCollector()

	collector.OnHTML(".chapter-list li", func(e *colly.HTMLElement) {
		chapterURL := e.Request.AbsoluteURL(e.ChildAttr("a", "href"))
		chapterTitle := strings.TrimSpace(e.ChildText("strong.chapter-title"))
		chapterCounter++
		chapters = append(chapters, models.Chapter{
			Number:          chapterCounter,
			TranslatedTitle: &chapterTitle,
			URL:             chapterURL,
		})
	})

	collector.OnHTML(".pagination li.PagedList-skipToNext a", func(e *colly.HTMLElement) {
		nextPageURL := e.Attr("href")
		if nextPageURL != "" {
			fmt.Println("Found next page:", nextPageURL)
			e.Request.Visit(nextPageURL)
		}
	})

	err := collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("visiting chapter list: %w", err)
	}

	return chapters, nil
}

func (c *Crawler) extract69shuChapter(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter

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

	collector.OnHTML("#catalog ul li a", func(a *colly.HTMLElement) {
		chapterURL := a.Attr("href")
		chapterTitle := a.Text
		chapterNumberStr := a.DOM.Parent().AttrOr("data-num", "")
		if chapterNumberStr == "" {
			log.Printf("Skipping element without data-num attribute")
			return
		}

		chapterNumber, err := strconv.Atoi(chapterNumberStr)
		if err != nil {
			log.Printf("Error parsing chapter number: %s", err)
			return
		}

		chapters = append(chapters, models.Chapter{
			URL:    chapterURL,
			Title:  chapterTitle,
			Number: chapterNumber,
		})
	})
	err := collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("visiting chapter list: %w", err)
	}

	sort.Slice(chapters, func(i, j int) bool {
		return chapters[i].Number > chapters[j].Number
	})

	return chapters, nil
}

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
	case strings.Contains(pageURL, "69shuba.cx"):
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

	collector.OnHTML(".mybox p", func(e *colly.HTMLElement) {
		text := strings.TrimSpace(e.Text)
		if text != "" {
			contentBuilder.WriteString(text + "\n\n")
		}
	})

	err := collector.Visit(pageURL)
	if err != nil {
		return fmt.Errorf("visiting chapter page: %w", err)
	}

	return nil
}
