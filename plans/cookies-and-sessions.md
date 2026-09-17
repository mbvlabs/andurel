# Cookies, sessions, and flashes

Implemented by `pkg/kiks` plus generated `router/cookies`.

Applications define named cookies in one place, middleware loads them onto
`context.Context`, and controllers, Templ, and Inertia all use that bag.

```go
kiks.Get[cookies.App](ctx)
kiks.Set(ctx, cookies.App{IsAuthenticated: true})
kiks.AddFlash(ctx, kiks.FlashSuccess, "Saved")
kiks.Flashes(ctx)
```

Session storage is an encrypted cookie. Flashes live in the session
the session envelope. Persist flashes only when the response will not render
them (HTTP redirect, Datastar `X-Andurel-Client-Redirect`). Templ and Inertia
toasts share the `FlashMessage` JSON contract and the `toast-stack` markup/CSS.

There is no `router/appctx` package.
