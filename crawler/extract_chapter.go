package crawler

import (
	"fmt"
	"go-novel/models"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

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
	case strings.Contains(url, "69shu.me"):
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
