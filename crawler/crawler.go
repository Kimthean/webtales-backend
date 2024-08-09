package crawler

import (
	"crypto/tls"
	"fmt"
	"go-novel/models"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"golang.org/x/exp/rand"
)

type Crawler struct{}

func NewCrawler() *Crawler {
	return &Crawler{}
}

func (c *Crawler) newCollector() *colly.Collector {
	collector := colly.NewCollector(
		colly.UserAgent(RandomUserAgent()),
	)

	collector.SetRequestTimeout(30 * time.Second)
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
		DomainGlob:  "*lightnovelworld.co*",
		RandomDelay: 4 * time.Second,
	})

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{
		Transport: transport,
	}

	collector.WithTransport(httpClient.Transport)

	collector.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
		r.Headers.Set("Referer", "https://www.google.com/")
	})

	return collector
}

func RandomUserAgent() string {
	userAgents := []string{
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
	}
	randIndex := rand.Intn(len(userAgents))
	return userAgents[randIndex]
}

func (c *Crawler) CrawlNovel(url string) (*models.Novel, error) {
	log.Printf("Crawling novel from %s", url)

	var novel *models.Novel

	if strings.Contains(url, "9999txt.cc") {
		novel = &models.Novel{URL: &url}
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
			chapter, err := c.extractChapters(chapterListURL)
			if err != nil {
				log.Printf("Error extracting chapters: %s", err)
			}
			novel.Chapters = chapter
		})

		err := collector.Visit(url)
		if err != nil {
			return nil, fmt.Errorf("visiting novel page: %w", err)
		}

	} else if strings.Contains(url, "uukanshu.cc/") {
		novel = &models.Novel{URL: &url}
		collector := c.newCollector()

		log.Println("Crawling novel from uukanshu.cc")

		collector.OnHTML(".thumbnail", func(e *colly.HTMLElement) {
			imageURL := e.Attr("src")
			tempImageURL := imageURL
			novel.Thumbnail = &tempImageURL
			log.Printf("Thumbnail: %s", imageURL)
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
	} else if strings.Contains(url, "www.wuxiabox.com/") {
		novel = &models.Novel{URL: &url}
		collector := c.newCollector()

		log.Println("Crawling novel from wuxiabox.com")

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
			detailedSummary := e.ChildText(".summary .content")
			novel.Description = &detailedSummary

		})

		err := collector.Visit(url)
		if err != nil {
			return nil, fmt.Errorf("visiting novel page: %w", err)
		}
		chapter, err := c.extractChapters(url)
		if err != nil {
			log.Printf("Error extracting chapters: %s", err)
		}
		novel.Chapters = chapter

	} else if strings.Contains(url, "lightnovelworld.co") {
		novel = &models.Novel{URL: &url}
		collector := c.newCollector()

		collector.OnHTML(".novel-title", func(e *colly.HTMLElement) {
			title := strings.TrimSpace(e.Text)
			fmt.Println("Title:", title)
			novel.Title = &title
		})

		collector.OnHTML(".alternative-title", func(e *colly.HTMLElement) {
			altTitle := strings.TrimSpace(e.Text)
			fmt.Println("Alternative Title:", altTitle)
			novel.RawTitle = &altTitle
		})

		collector.OnHTML(".property-item span[itemprop='author']", func(e *colly.HTMLElement) {
			author := strings.TrimSpace(e.Text)
			fmt.Println("Author:", author)
			novel.Author = &author
		})

		collector.OnHTML(".fixed-img img", func(e *colly.HTMLElement) {
			coverImageURL := e.Attr("src")
			if strings.HasPrefix(coverImageURL, "data:image") {
				coverImageURL = e.Attr("data-src")
			}
			fmt.Println("Cover Image URL:", coverImageURL)
			novel.Thumbnail = &coverImageURL
		})

		collector.OnHTML(".summary .content", func(e *colly.HTMLElement) {
			description := ""
			e.ForEach("p", func(_ int, el *colly.HTMLElement) {
				description += el.Text + "\n\n"
			})
			novel.Description = &description
		})

		collector.OnHTML("a.chapter-latest-container", func(e *colly.HTMLElement) {
			chapterListURL := e.Request.AbsoluteURL(e.Attr("href"))
			chapter, err := c.extractChapters(chapterListURL)
			if err != nil {
				log.Printf("Error extracting chapters: %s", err)
			}
			novel.Chapters = chapter
		})

		err := collector.Visit(url)
		if err != nil {
			return nil, fmt.Errorf("visiting novel page: %w", err)
		}

	}

	return novel, nil
}

func (c *Crawler) extractChapters(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter
	collector := c.newCollector()

	chapterCounter := 0

	if strings.Contains(url, "9999txt.cc") {
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
	} else if strings.Contains(url, "wuxiabox.com") {
		baseURL := url
		visitedPages := make(map[string]bool)

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
		}

	} else if strings.Contains(url, "lightnovelworld.co/") {

		c := c.newCollector()

		c.OnHTML(".chapter-list li", func(e *colly.HTMLElement) {
			chapterURL := e.Request.AbsoluteURL(e.ChildAttr("a", "href"))
			chapterTitle := strings.TrimSpace(e.ChildText("strong.chapter-title"))
			chapterCounter++
			chapters = append(chapters, models.Chapter{
				Number:          chapterCounter,
				TranslatedTitle: &chapterTitle,
				URL:             chapterURL,
			})
		})

		c.OnHTML(".pagination li.PagedList-skipToNext a", func(e *colly.HTMLElement) {
			nextPageURL := e.Attr("href")
			if nextPageURL != "" {
				fmt.Println("Found next page:", nextPageURL)
				e.Request.Visit(nextPageURL)
			}
		})

		c.Visit(url)
	}

	return chapters, nil
}

func (c *Crawler) CrawlChapter(chapterURL string, chapterTitle string, chapterNumber int) (*models.Chapter, error) {
	chapter := &models.Chapter{
		Number: chapterNumber,
		Title:  chapterTitle,
		URL:    chapterURL,
	}

	var contentBuilder strings.Builder
	err := c.crawlChapterPage(chapterURL, &contentBuilder)
	if err != nil {
		return nil, err
	}

	content := contentBuilder.String()

	if strings.Contains(chapterURL, "wuxiabox") || strings.Contains(chapterURL, "lightnovelworld") {
		chapter.TranslatedContent = &content
	} else {
		chapter.Content = &content
	}

	return chapter, nil
}

func (c *Crawler) crawlChapterPage(pageURL string, contentBuilder *strings.Builder) error {

	collector := c.newCollector()
	var paragraphs []string
	var nextPageURL string
	var nextPageText string

	if strings.Contains(pageURL, "9999txt.cc") {
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

		// Append the content of the current page, skipping the last paragraph if it's the "continue reading" text
		for i, text := range paragraphs {
			trimmedText := strings.TrimSpace(text)
			// Normalize spaces and punctuation for comparison
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

		// Check if the next page button is not "下一章" (Next Chapter)
		if nextPageURL != "" && nextPageURL != pageURL && nextPageText != "下一章" && !strings.Contains(nextPageURL, "javascript:void(0);") {
			err = c.crawlChapterPage(nextPageURL, contentBuilder)
			if err != nil {
				return err
			}
		}
	} else if strings.Contains(pageURL, "uukanshu.cc") {

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
			log.Printf("Failed to crawl page: %s, error: %v", pageURL, err)
			return err
		}
	} else if strings.Contains(pageURL, "wuxiabox.com") {

		collector.OnRequest(func(r *colly.Request) {
			time.Sleep(4 * time.Second)
		})

		collector.OnHTML(".chapter-content", func(e *colly.HTMLElement) {
			// Function to process text content
			processContent := func(text string) {
				text = strings.TrimSpace(text)
				if text != "" {
					contentBuilder.WriteString(text + "\n\n")
				}
			}

			// Process <p> tags
			e.ForEach("p", func(_ int, el *colly.HTMLElement) {
				processContent(el.Text)
			})

			// Process direct text nodes
			e.DOM.Contents().Each(func(_ int, s *goquery.Selection) {
				if goquery.NodeName(s) == "#text" {
					processContent(s.Text())
				}
			})

			// Remove any unwanted elements
			content := contentBuilder.String()
			content = strings.ReplaceAll(content, "&ZeroWidthSpace;", "")

			contentBuilder.Reset()
			contentBuilder.WriteString(content)
		})

		err := collector.Visit(pageURL)
		if err != nil {
			log.Printf("Failed to crawl page: %s, error: %v", pageURL, err)
			return err
		}

	} else if strings.Contains(pageURL, "lightnovelworld.co") {
		collector.OnHTML(".chapter-content p", func(e *colly.HTMLElement) {
			content := e.Text + "\n\n"
			contentBuilder.WriteString(content)
		})

		err := collector.Visit(pageURL)
		if err != nil {
			log.Printf("Failed to crawl page: %s, error: %v", pageURL, err)
			return err
		}
	}

	return nil
}
