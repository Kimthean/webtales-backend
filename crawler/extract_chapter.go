package crawler

import (
	"fmt"
	"go-novel/models"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
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
	chapterCounter := 0

	page, err := c.newPage()
	if err != nil {
		return nil, fmt.Errorf("creating new page: %w", err)
	}
	defer page.Close()

	err = page.Navigate(url)
	if err != nil {
		return nil, fmt.Errorf("navigating to URL: %w", err)
	}

	page.MustWaitLoad()

	chapterElements, err := page.Elements("div#list a[rel='chapter']")
	if err != nil {
		return nil, fmt.Errorf("finding chapter elements: %w", err)
	}

	for _, el := range chapterElements {
		chapterURL := page.MustInfo().URL + *el.MustAttribute("href")
		chapterTitle := el.MustElement("dd").MustText()

		chapterCounter++

		chapters = append(chapters, models.Chapter{
			Number: chapterCounter,
			Title:  chapterTitle,
			URL:    chapterURL,
		})
	}

	return chapters, nil
}

func (c *Crawler) extractWuxiaboxChapters(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter
	baseURL := url
	visitedPages := make(map[string]bool)
	chapterCounter := 0

	for {
		page, err := c.newPage()
		if err != nil {
			return nil, fmt.Errorf("creating new page: %w", err)
		}

		err = page.Navigate(baseURL)
		if err != nil {
			page.Close()
			return nil, fmt.Errorf("navigating to URL: %w", err)
		}

		page.MustWaitLoad()

		log.Printf("Extracting chapters from page: %s", baseURL)

		chapterElements, err := page.Elements(".chapter-list li")
		if err != nil {
			page.Close()
			return nil, fmt.Errorf("finding chapter elements: %w", err)
		}

		for _, el := range chapterElements {
			chapterCounter++

			chapterURL := page.MustInfo().URL + *el.MustElement("a").MustAttribute("href")
			chapterTitle := el.MustElement(".chapter-title").MustText()
			chapters = append(chapters, models.Chapter{
				Number:          chapterCounter,
				TranslatedTitle: &chapterTitle,
				URL:             chapterURL,
			})
		}

		nextPageElement, err := page.Element(".pagination li:not(.active) a[data-ajax='true']")
		if err != nil || nextPageElement == nil {
			page.Close()
			break
		}

		nextPageURL := *nextPageElement.MustAttribute("href")
		absoluteNextPageURL := page.MustInfo().URL + nextPageURL

		if absoluteNextPageURL == baseURL || visitedPages[absoluteNextPageURL] || strings.Contains(absoluteNextPageURL, "page=0") {
			page.Close()
			break
		}

		baseURL = absoluteNextPageURL
		visitedPages[baseURL] = true
		page.Close()
		time.Sleep(2 * time.Second)
	}

	return chapters, nil
}

func (c *Crawler) extractLightNovelWorldChapters(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter
	chapterCounter := 0

	for {
		page, err := c.newPage()
		if err != nil {
			return nil, fmt.Errorf("creating new page: %w", err)
		}

		err = page.Navigate(url)
		if err != nil {
			page.Close()
			return nil, fmt.Errorf("navigating to URL: %w", err)
		}

		page.MustWaitLoad()

		chapterElements, err := page.Elements(".chapter-list li")
		if err != nil {
			page.Close()
			return nil, fmt.Errorf("finding chapter elements: %w", err)
		}

		for _, el := range chapterElements {
			chapterURL := page.MustInfo().URL + *el.MustElement("a").MustAttribute("href")
			chapterTitle := strings.TrimSpace(el.MustElement("strong.chapter-title").MustText())
			chapterCounter++
			chapters = append(chapters, models.Chapter{
				Number:          chapterCounter,
				TranslatedTitle: &chapterTitle,
				URL:             chapterURL,
			})
		}

		nextPageElement, err := page.Element(".pagination li.PagedList-skipToNext a")
		if err != nil || nextPageElement == nil {
			page.Close()
			break
		}

		url = *nextPageElement.MustAttribute("href")
		fmt.Println("Found next page:", url)
		page.Close()
		time.Sleep(2 * time.Second)
	}

	return chapters, nil
}

func (c *Crawler) extract69shuChapter(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter

	page, err := c.newPage()
	if err != nil {
		return nil, fmt.Errorf("creating new page: %w", err)
	}
	defer page.Close()

	err = page.Navigate(url)
	if err != nil {
		return nil, fmt.Errorf("navigating to URL: %w", err)
	}

	page.MustWaitLoad()

	chapterElements, err := page.Elements("#catalog ul li a")
	if err != nil {
		return nil, fmt.Errorf("finding chapter elements: %w", err)
	}

	for _, el := range chapterElements {
		chapterURL := page.MustInfo().URL + *el.MustAttribute("href")
		chapterTitle := el.MustText()
		chapterNumberStr := el.MustParent().MustAttribute("data-num")

		chapterNumber, err := strconv.Atoi(*chapterNumberStr)
		if err != nil {
			log.Printf("Error parsing chapter number: %s", err)
			continue
		}

		chapters = append(chapters, models.Chapter{
			URL:    chapterURL,
			Title:  chapterTitle,
			Number: chapterNumber,
		})
	}

	sort.Slice(chapters, func(i, j int) bool {
		return chapters[i].Number < chapters[j].Number
	})

	return chapters, nil
}

func (c *Crawler) extract1stKissChapters(url string) ([]models.Chapter, error) {
	var chapters []models.Chapter

	page, err := c.newPage()
	if err != nil {
		return nil, fmt.Errorf("creating new page: %w", err)
	}
	defer page.Close()

	err = page.Navigate(url)
	if err != nil {
		return nil, fmt.Errorf("navigating to URL: %w", err)
	}

	page.MustWaitLoad()

	chapterElements, err := page.Elements("li.wp-manga-chapter")
	if err != nil {
		return nil, fmt.Errorf("finding chapter elements: %w", err)
	}

	for i, el := range chapterElements {
		link, err := el.Element("a")
		if err != nil {
			continue
		}
		chapterURL := page.MustInfo().URL + *link.MustAttribute("href")
		chapterTitle := strings.TrimSpace(link.MustText())
		chapters = append(chapters, models.Chapter{
			URL:    chapterURL,
			Title:  chapterTitle,
			Number: len(chapterElements) - i, // Reverse the order
		})
	}

	return chapters, nil
}
