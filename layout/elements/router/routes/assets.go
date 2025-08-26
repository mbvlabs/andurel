package routes

import (
	"fmt"
	"net/http"
	"time"
)

const (
	AssetsRoutePrefix = "/assets"
	assetsNamePrefix  = "assets"
)

var assetRoutes = []Route{
	Robots,
	Sitemap,
	CSSEntrypoint,
	CSSFile,
	JSEntrypoint,
	JSFile,
}

var startTime = time.Now().Unix()

var Robots = Route{
	Name:         assetsNamePrefix + ".robots",
	Path:         assetsNamePrefix + "/robots.txt",
	Method:       http.MethodGet,
	Handler:      "Assets",
	HandleMethod: "Robots",
}

var Sitemap = Route{
	Name:         assetsNamePrefix + ".sitemap",
	Path:         assetsNamePrefix + "/sitemap.xml",
	Method:       http.MethodGet,
	Handler:      "Assets",
	HandleMethod: "Sitemap",
}

var CSSEntrypoint = Route{
	Name:         assetsNamePrefix + "css.entry",
	Path:         assetsNamePrefix + fmt.Sprintf("/css/%v/styles.css", startTime),
	Method:       http.MethodGet,
	Handler:      "Assets",
	HandleMethod: "CSSEntrypoint",
}

var CSSFile = Route{
	Name:         assetsNamePrefix + "css.all",
	Path:         assetsNamePrefix + fmt.Sprintf("/css/%v/:file", startTime),
	Method:       http.MethodGet,
	Handler:      "Assets",
	HandleMethod: "CSSFile",
}

var JSEntrypoint = Route{
	Name:         assetsNamePrefix + "js.entry",
	Path:         assetsNamePrefix + fmt.Sprintf("/js/%v/script.js", startTime),
	Method:       http.MethodGet,
	Handler:      "Assets",
	HandleMethod: "JSEntrypoint",
}

var JSFile = Route{
	Name:         assetsNamePrefix + "js.all",
	Path:         assetsNamePrefix + fmt.Sprintf("/js/%v/:file", startTime),
	Method:       http.MethodGet,
	Handler:      "Assets",
	HandleMethod: "JSFile",
}
