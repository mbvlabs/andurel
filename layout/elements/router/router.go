// Package router defines the routing logic for the web application using the Echo framework.
package router

import (
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log/slog"
	"mbvlabs/andurel/layout/elements/config"
	"mbvlabs/andurel/layout/elements/controllers"
	"mbvlabs/andurel/layout/elements/router/cookies"
	"mbvlabs/andurel/layout/elements/router/routes"
	"net/http"
	"reflect"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"

	echomw "github.com/labstack/echo/v4/middleware"
)

type Router struct {
	Handler     *echo.Echo
	controllers controllers.Controllers
}

func New(
	controllers controllers.Controllers,
	// riverUI *riverui.Server,
	// traceProvider trace.TracerProvider,
) (*Router, error) {
	gob.Register(uuid.UUID{})
	gob.Register(cookies.FlashMessage{})

	router := echo.New()

	if config.App.Env != config.ProdEnvironment {
		router.Debug = true
	}

	authKey, err := hex.DecodeString(config.Auth.SessionKey)
	if err != nil {
		return nil, err
	}
	encKey, err := hex.DecodeString(config.Auth.SessionEncryptionKey)
	if err != nil {
		return nil, err
	}

	// httpRequestsTotal, err := telemetry.HTTPRequestsTotal()
	// if err != nil {
	// 	return nil, err
	// }
	//
	// httpDuration, err := telemetry.HTTPRequestDuration()
	// if err != nil {
	// 	return nil, err
	// }
	//
	// httpInFlight, err := telemetry.HTTPRequestsInFlight()
	// if err != nil {
	// 	return nil, err
	// }

	router.Use(
		session.Middleware(
			sessions.NewCookieStore(
				authKey,
				encKey,
			),
		),
		registerAppContext,
		registerFlashMessagesContext,

		echomw.CSRFWithConfig(echomw.CSRFConfig{Skipper: func(c echo.Context) bool {
			return strings.HasPrefix(c.Request().URL.Path, routes.AssetsRoutePrefix)
		}, TokenLookup: "cookie:_csrf", CookiePath: "/", CookieDomain: func() string {
			if config.App.Env == config.ProdEnvironment {
				return config.App.Domain
			}

			return ""
		}(), CookieSecure: false, CookieHTTPOnly: true, CookieSameSite: http.SameSiteStrictMode}),
		// middleware.Logging(
		// traceProvider,
		// httpRequestsTotal,
		// httpDuration,
		// httpInFlight,
		// ),
		echomw.Recover(),
	)

	// router.Any("/river*", echo.WrapHandler(riverUI), middleware.AuthOnly)

	return &Router{
		router,
		controllers,
	}, nil
}

func (r *Router) SetupRoutes() *echo.Echo {
	registeredRoutes := []string{}
	controllersValue := reflect.ValueOf(r.controllers)

	for _, route := range routes.BuildRoutes {
		if registered := slices.Contains(registeredRoutes, route.Name); registered {
			panic(
				fmt.Sprintf(
					"%s is registered more than once",
					route.Name,
				),
			)
		}

		if route.Handler == "" || route.HandleMethod == "" {
			panic("Route must specify Handler and HandleMethod fields")
		}

		controllerField := controllersValue.FieldByName(route.Handler)
		if !controllerField.IsValid() {
			panic(
				fmt.Sprintf(
					"Controller field %s not found in controllers struct",
					route.Handler,
				),
			)
		}

		controller := controllerField.Interface()
		controllerFunc := getHandlerFunc(controller, route.HandleMethod)

		var middlewareFuncs []echo.MiddlewareFunc
		for _, mw := range route.Middleware {
			middlewareFuncs = append(middlewareFuncs, echo.MiddlewareFunc(mw))
		}

		switch route.Method {
		case http.MethodGet:
			registeredRoutes = append(registeredRoutes, route.Name)
			r.Handler.GET(route.Path, controllerFunc, middlewareFuncs...).Name = route.Name
		case http.MethodPost:
			registeredRoutes = append(registeredRoutes, route.Name)
			r.Handler.POST(route.Path, controllerFunc, middlewareFuncs...).Name = route.Name
		case http.MethodPut:
			registeredRoutes = append(registeredRoutes, route.Name)
			r.Handler.PUT(route.Path, controllerFunc, middlewareFuncs...).Name = route.Name
		case http.MethodDelete:
			registeredRoutes = append(registeredRoutes, route.Name)
			r.Handler.DELETE(route.Path, controllerFunc, middlewareFuncs...).Name = route.Name
		}
	}

	r.Handler.RouteNotFound(
		"/*",
		getHandlerFunc(r.controllers.Pages, "NotFound"),
	)

	return r.Handler
}

func getHandlerFunc(controller any, methodName string) echo.HandlerFunc {
	appType := reflect.TypeOf(controller)
	method, found := appType.MethodByName(methodName)
	if !found {
		panic(fmt.Sprintf("Controller method %s not found", methodName))
	}

	return func(c echo.Context) error {
		values := method.Func.Call([]reflect.Value{
			reflect.ValueOf(controller),
			reflect.ValueOf(c),
		})

		if len(values) != 1 {
			panic(
				fmt.Sprintf(
					"Controller method %s does not return exactly one value",
					methodName,
				),
			)
		}

		if values[0].IsNil() {
			return nil
		}

		return values[0].Interface().(error)
	}
}

func registerFlashMessagesContext(
	next echo.HandlerFunc,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		if strings.HasPrefix(c.Request().URL.Path, routes.AssetsRoutePrefix) {
			return next(c)
		}

		flashMessages, err := cookies.GetFlashes(c)
		if err != nil {
			slog.WarnContext(c.Request().Context(),
				"Failed to get flash messages",
				"error", err,
			)

			return next(c)
		}

		c.Set(cookies.FlashKey, flashMessages)

		return next(c)
	}
}

func registerAppContext(
	next echo.HandlerFunc,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		// if strings.HasPrefix(c.Request().URL.Path, "/static") ||
		// 	strings.HasPrefix(c.Request().URL.Path, "/fragments") {
		// 	return next(c)
		// }

		c.Set(cookies.AppKey, cookies.GetApp(c))

		return next(c)
	}
}
