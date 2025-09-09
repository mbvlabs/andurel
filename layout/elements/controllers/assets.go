package controllers

import (
	//nolint:gosec //only needed for browser caching

	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"

	"mbvlabs/andurel/layout/elements/assets"
	"mbvlabs/andurel/layout/elements/config"
	"mbvlabs/andurel/layout/elements/router/routes"

	"github.com/labstack/echo/v4"
	"github.com/maypok86/otter"
	"gopkg.in/yaml.v2"
)

const (
	sitemapCacheKey = "assets.sitemap"
	robotsCacheKey  = "assets.robots"
	weekInHours     = 168
	threeInHours    = 72
)

type Assets struct {
	sitemapCache otter.Cache[string, Sitemap]
	assetsCache  otter.Cache[string, string]
}

func newAssets() Assets {
	sitemapCacheBuilder, err := otter.NewBuilder[string, Sitemap](1)
	if err != nil {
		panic(err)
	}

	sitemapCache, err := sitemapCacheBuilder.WithTTL(threeInHours).Build()
	if err != nil {
		panic(err)
	}

	robotsCacheBuilder, err := otter.NewBuilder[string, string](1)
	if err != nil {
		panic(err)
	}

	robotsCache, err := robotsCacheBuilder.WithTTL(weekInHours).Build()
	if err != nil {
		panic(err)
	}

	return Assets{sitemapCache, robotsCache}
}

func (a Assets) enableCaching(c echo.Context, content []byte) echo.Context {
	// if config.Cfg.App.Env == config.PROD_ENVIRONMENT {
	// 	//nolint:gosec //only needed for browser caching
	// 	hash := md5.Sum(content)
	// 	etag := fmt.Sprintf(`W/"%x-%x"`, hash, len(content))
	//
	// 	c.Response().
	// 		Header().
	// 		Set("Cache-Control", fmt.Sprintf("public, max-age=%s", threeMonthsCache))
	// 	c.Response().
	// 		Header().
	// 		Set("Vary", "Accept-Encoding")
	// 	c.Response().
	// 		Header().
	// 		Set("ETag", etag)
	// }

	return c
}

func (a Assets) Robots(c echo.Context) error {
	if value, ok := a.assetsCache.Get(robotsCacheKey); ok {
		return c.String(http.StatusOK, string(value))
	}

	type robotsTxt struct {
		UserAgent string `yaml:"User-agent"`
		Allow     string `yaml:"Allow"`
		Sitemap   string `yaml:"Sitemap"`
	}

	robots, err := yaml.Marshal(robotsTxt{
		UserAgent: "*",
		Allow:     "/",
		Sitemap: fmt.Sprintf(
			"%s%s",
			config.App.GetFullDomain(),
			routes.Sitemap.Path,
		),
	})
	if err != nil {
		return err
	}

	return c.String(http.StatusOK, string(robots))
}

func (a Assets) Sitemap(c echo.Context) error {
	if value, ok := a.sitemapCache.Get(sitemapCacheKey); ok {
		return c.XML(http.StatusOK, value)
	}

	sitemap, err := createSitemap(c)
	if err != nil {
		return err
	}

	if ok := a.sitemapCache.Set(sitemapCacheKey, sitemap); !ok {
		slog.ErrorContext(
			c.Request().Context(),
			"could not set sitemap cache",
			"error",
			err,
		)
	}

	return c.XML(http.StatusOK, sitemap)
}

type URL struct {
	XMLName    xml.Name `xml:"url"`
	Loc        string   `xml:"loc"`
	ChangeFreq string   `xml:"changefreq"`
	LastMod    string   `xml:"lastmod,omitempty"`
	Priority   string   `xml:"priority,omitempty"`
}

type Sitemap struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr"`
	URL     []URL    `xml:"url"`
}

func createSitemap(c echo.Context) (Sitemap, error) {
	baseURL := config.App.GetFullDomain()

	var urls []URL

	urls = append(urls, URL{
		Loc:        baseURL,
		ChangeFreq: "monthly",
		LastMod:    "2024-10-22T09:43:09+00:00",
		Priority:   "1",
	})

	for _, r := range c.Echo().Routes() {
		switch r.Name {
		// case routes.AboutPage.Name:
		// 	urls = append(urls, URL{
		// 		Loc: fmt.Sprintf(
		// 			"%s%s",
		// 			baseUrl,
		// 			r.Path,
		// 		),
		// 		ChangeFreq: "monthly",
		// 	})
		}
	}

	sitemap := Sitemap{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URL:   urls,
	}

	return sitemap, nil
}

func (a Assets) Stylesheet(c echo.Context) error {
	stylesheet, err := assets.Files.ReadFile(
		"css/tw.css",
	)
	if err != nil {
		return err
	}

	c = a.enableCaching(c, stylesheet)
	return c.Blob(http.StatusOK, "text/css", stylesheet)
}

func (a Assets) Scripts(c echo.Context) error {
	stylesheet, err := assets.Files.ReadFile(
		"js/scripts.js",
	)
	if err != nil {
		return err
	}

	c = a.enableCaching(c, stylesheet)
	return c.Blob(http.StatusOK, "application/javascript", stylesheet)
}

func (a Assets) Script(c echo.Context) error {
	param := c.Param("file")
	stylesheet, err := assets.Files.ReadFile(
		fmt.Sprintf("js/%s", param),
	)
	if err != nil {
		return err
	}

	c = a.enableCaching(c, stylesheet)
	return c.Blob(http.StatusOK, "application/javascript", stylesheet)
}
