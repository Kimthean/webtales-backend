package crawler

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"golang.org/x/exp/rand"
)

type Crawler struct {
	userAgents []string
	browser    *rod.Browser
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
		browser: nil,
	}
}

func (c *Crawler) initBrowser() error {
	if c.browser != nil {
		return nil
	}

	log.Println("Initializing browser")

	chromeURL := os.Getenv("CHROME_URL")
	if chromeURL == "" {
		chromeURL = "ws://chrome:3000" // Default URL if not set in environment
	}

	browser := rod.New().ControlURL(chromeURL)
	log.Println("Connecting to browser")
	err := browser.Connect()
	if err != nil {
		return fmt.Errorf("connecting to browser: %w", err)
	}
	log.Println("Connected to browser")

	c.browser = stealth.MustPage(browser).Browser()
	log.Println("Browser initialization complete")
	return nil
}

func (c *Crawler) CloseBrowser() {
	if c.browser != nil {
		c.browser.MustClose()
		c.browser = nil
	}
}

func (c *Crawler) newPage() (*rod.Page, error) {
	err := c.initBrowser()
	if err != nil {
		return nil, fmt.Errorf("initializing browser: %w", err)
	}

	page := stealth.MustPage(c.browser)

	userAgent := c.randomUserAgent()
	err = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: userAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("setting user agent: %w", err)
	}

	headers := []string{
		"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Language", "en-US,en;q=0.5",
		"Referer", "https://www.google.com/",
	}
	if len(headers)%2 == 0 {
		_, err = page.SetExtraHeaders(headers)
		if err != nil {
			return nil, fmt.Errorf("setting extra headers: %w", err)
		}
	} else {
		log.Println("Warning: Odd number of header parameters, skipping SetExtraHeaders")
	}

	page.Timeout(30 * time.Second)

	return page, nil
}

func (c *Crawler) randomUserAgent() string {
	return c.userAgents[rand.Intn(len(c.userAgents))]
}
