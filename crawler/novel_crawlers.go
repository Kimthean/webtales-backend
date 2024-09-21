package crawler

import (
	"fmt"
	"go-novel/models"
	"log"
	"strings"
)

func (c *Crawler) CrawlNovel(url string) (*models.Novel, error) {
	log.Printf("Crawling novel from %s", url)

	var novel *models.Novel
	var err error

	switch {
	case strings.Contains(url, "webtalesmtl.xyz"):
		err = c.testWebtales(url)
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
	case strings.Contains(url, "1stkissnovel.org"):
		novel, err = c.crawl1stKiss(url)
	default:
		return nil, fmt.Errorf("unsupported URL: %s", url)
	}

	if err != nil {
		return nil, fmt.Errorf("crawling novel: %w", err)
	}

	return novel, nil
}

func (c *Crawler) testWebtales(url string) error {
	page, err := c.newPage()
	if err != nil {
		log.Println("Error ")
	}
	defer page.MustClose()

	err = page.Navigate(url)
	if err != nil {
		log.Println("Error going to page")
	}
	log.Println("Waiting for page to load")
	page.MustWaitLoad()

	// Get the entire HTML content
	html, err := page.HTML()
	if err != nil {
		return fmt.Errorf("failed to get HTML: %w", err)
	}

	// Log the entire HTML content
	log.Println("Page HTML:")
	fmt.Println(html)

	return nil
}

func (c *Crawler) crawlWuxiabox(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}

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

	// Extract title
	title, err := page.Element(".novel-title")
	if err == nil {
		titleText := title.MustText()
		novel.Title = &titleText
	}

	// Extract alternative title
	altTitle, err := page.Element(".alternative-title")
	if err == nil {
		altTitleText := altTitle.MustText()
		novel.RawTitle = &altTitleText
	}

	// Extract author
	author, err := page.Element(".author span[itemprop='author']")
	if err == nil {
		authorText := author.MustText()
		novel.Author = &authorText
	}

	// Extract thumbnail
	img, err := page.Element(".fixed-img img")
	if err == nil {
		imgSrc := img.MustAttribute("src")
		if *imgSrc == "/static/picture/placeholder-158.jpg" {
			imgSrc = img.MustAttribute("data-src")
		}
		imageURL := page.MustInfo().URL + *imgSrc
		novel.Thumbnail = &imageURL
	}

	// Extract description
	description, err := page.Element("#info .summary .content")
	if err == nil {
		var paragraphs []string
		elements, err := description.Elements("p")
		if err == nil {
			for _, p := range elements {
				paragraphs = append(paragraphs, strings.TrimSpace(p.MustText()))
			}
			detailedSummary := strings.Join(paragraphs, "\n\n")
			novel.Description = &detailedSummary
		}
	}

	// Extract chapters
	chapters, err := c.extractChapters(url)
	if err != nil {
		log.Printf("Error extracting chapters: %s", err)
	}
	novel.Chapters = chapters

	return novel, nil
}

func (c *Crawler) crawl9999txt(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}

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

	// Extract title
	title, err := page.Element("#info h1")
	if err == nil {
		titleText := title.MustText()
		novel.Title = &titleText
	}

	// Extract thumbnail
	img, err := page.Element("#fmimg img")
	if err == nil {
		imageURL := img.MustAttribute("data-original")
		novel.Thumbnail = imageURL
	}

	// Extract author
	author, err := page.Element("#info > p:first-of-type a")
	if err == nil {
		authorText := author.MustText()
		novel.Author = &authorText
	}

	// Extract description
	description, err := page.Element("#intro")
	if err == nil {
		descriptionText := description.MustText()
		novel.Description = &descriptionText
	}

	// Extract chapters
	chapterListLink, err := page.Element(".readbtn .chapterlist")
	if err == nil {
		chapterListURL := page.MustInfo().URL + *chapterListLink.MustAttribute("href")
		chapters, err := c.extractChapters(chapterListURL)
		if err != nil {
			log.Printf("Error extracting chapters: %s", err)
		}
		novel.Chapters = chapters
	}

	return novel, nil
}

func (c *Crawler) crawlUukanshu(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}

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

	// Extract thumbnail
	img, err := page.Element(".thumbnail")
	if err == nil {
		imageURL := page.MustInfo().URL + *img.MustAttribute("src")
		novel.Thumbnail = &imageURL
	}

	// Extract title
	title, err := page.Element(".booktitle")
	if err == nil {
		titleText := title.MustText()
		novel.Title = &titleText
	}

	// Extract description
	description, err := page.Element(".bookintro")
	if err == nil {
		descriptionText := description.MustText()
		novel.Description = &descriptionText
	}

	// Extract author
	author, err := page.Element(".booktag a.red")
	if err == nil {
		authorText := strings.Replace(author.MustText(), "作者：", "", -1)
		novel.Author = &authorText
	}

	// Extract chapters
	var chapters []models.Chapter
	chapterElements, err := page.Elements(".book.chapterlist dd a")
	if err == nil {
		for i, el := range chapterElements {
			chapterURL := page.MustInfo().URL + *el.MustAttribute("href")
			chapterTitle := el.MustText()
			chapters = append(chapters, models.Chapter{
				URL:    chapterURL,
				Title:  chapterTitle,
				Number: i + 1,
			})
		}
	}
	novel.Chapters = chapters

	return novel, nil
}

func (c *Crawler) crawlWuxiaspot(url string) (*models.Novel, error) {
	// This function can be very similar to crawlWuxiabox
	// Just adjust the selectors if they're different
	return c.crawlWuxiabox(url)
}

func (c *Crawler) crawlLightNovelWorld(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}

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

	// Extract title
	title, err := page.Element(".novel-title")
	if err == nil {
		titleText := strings.TrimSpace(title.MustText())
		novel.Title = &titleText
	}

	// Extract alternative title
	altTitle, err := page.Element(".alternative-title")
	if err == nil {
		altTitleText := strings.TrimSpace(altTitle.MustText())
		novel.RawTitle = &altTitleText
	}

	// Extract author
	author, err := page.Element(".property-item span[itemprop='author']")
	if err == nil {
		authorText := strings.TrimSpace(author.MustText())
		novel.Author = &authorText
	}

	// Extract thumbnail
	img, err := page.Element(".fixed-img img")
	if err == nil {
		imageURL := img.MustAttribute("src")
		if strings.HasPrefix(*imageURL, "data:image") {
			imageURL = img.MustAttribute("data-src")
		}
		novel.Thumbnail = imageURL
	}

	// Extract description
	description, err := page.Element(".summary .content")
	if err == nil {
		var paragraphs []string
		elements, err := description.Elements("p")
		if err == nil {
			for _, p := range elements {
				paragraphs = append(paragraphs, p.MustText())
			}
			detailedSummary := strings.Join(paragraphs, "\n\n\n")
			novel.Description = &detailedSummary
		}
	}

	// Extract chapters
	chapterListLink, err := page.Element("a.chapter-latest-container")
	if err == nil {
		chapterListURL := page.MustInfo().URL + *chapterListLink.MustAttribute("href")
		chapters, err := c.extractChapters(chapterListURL)
		if err != nil {
			log.Printf("Error extracting chapters: %s", err)
		}
		novel.Chapters = chapters
	}

	return novel, nil
}

func (c *Crawler) crawl69Shu(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}

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

	// Extract title and author
	bookbox, err := page.Element(".bookbox")
	if err == nil {
		title, err := bookbox.Element("h1 a")
		if err == nil {
			titleText := title.MustText()
			novel.Title = &titleText
		}

		author, err := bookbox.Element("p:contains('作者：') a")
		if err == nil {
			authorText := author.MustText()
			novel.Author = &authorText
		}
	}

	// Extract thumbnail
	img, err := page.Element(".bookimg img")
	if err == nil {
		imageURL := page.MustInfo().URL + *img.MustAttribute("src")
		novel.Thumbnail = &imageURL
	}

	// Extract description
	description, err := page.Element(".bookintro")
	if err == nil {
		descriptionText := description.MustText()
		novel.Description = &descriptionText
	}

	// Extract chapters
	chapters, err := c.extract69shuChapter(url)
	if err != nil {
		log.Printf("Error extracting chapters: %s", err)
	}
	novel.Chapters = chapters

	return novel, nil
}

func (c *Crawler) crawl1stKiss(url string) (*models.Novel, error) {
	novel := &models.Novel{URL: &url}

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

	// Extract title
	title, err := page.Element("h1.entry-title")
	if err == nil {
		titleText := strings.TrimSpace(title.MustText())
		novel.Title = &titleText
	}

	// Extract author
	author, err := page.Element(".author-content a")
	if err == nil {
		authorText := strings.TrimSpace(author.MustText())
		novel.Author = &authorText
	}

	// Extract thumbnail
	img, err := page.Element(".summary_image img")
	if err == nil {
		imageURL := img.MustAttribute("data-src")
		novel.Thumbnail = imageURL
	}

	// Extract description
	description, err := page.Element(".summary__content")
	if err == nil {
		descriptionText := strings.TrimSpace(description.MustText())
		novel.Description = &descriptionText
	}

	// Extract chapters
	chapterListLink, err := page.Element("li.wp-manga-chapter a")
	if err == nil {
		chapterListURL := *chapterListLink.MustAttribute("href")
		chapters, err := c.extract1stKissChapters(chapterListURL)
		if err != nil {
			log.Printf("Error extracting chapters: %s", err)
		}
		novel.Chapters = chapters
	}

	return novel, nil
}
