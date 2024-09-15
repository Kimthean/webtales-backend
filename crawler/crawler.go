package crawler

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"time"

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
		RandomDelay: 1 * time.Second,
	})
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*lightnovelworld.co*",
		RandomDelay: 4 * time.Second,
	})
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*69shu.me*",
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

func convertToUTF8(body []byte, contentType string) ([]byte, error) {
	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}
