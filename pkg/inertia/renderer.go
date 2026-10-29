// Package inertia implements the Inertia v3 protocol for anduin and templ.
package inertia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

// SharedProvider supplies request-scoped shared props.
type SharedProvider func(context.Context) (Props, error)

// VersionProvider supplies the current asset version.
type VersionProvider func(context.Context) (string, error)

// ReflashHandler preserves application flash data across redirect responses.
type ReflashHandler func(context.Context) error

// FlashProvider supplies the v3 top-level page flash value for each response.
type FlashProvider func(context.Context) any

// Option configures a Renderer.
type Option func(*Renderer) error

// Renderer implements the Inertia v3 server protocol for anduin and templ.
type Renderer struct {
	root                RootFunc
	containerID         string
	assetFS             fs.FS
	projectName         string
	environment         string
	buildPathURL        string
	entryPoint          string
	viteDevURL          string
	viteTags            viteTags
	version             string
	versionProvider     VersionProvider
	shared              Props
	sharedProviders     []SharedProvider
	requestFlash        []func(context.Context) any
	ssr                 SSRRenderer
	customSSR           bool
	ssrDefault          bool
	ssrURL              string
	ssrTimeout          time.Duration
	ssrMaxResponseBytes int64
	ssrFailFast         bool
	reflash             ReflashHandler
	protocolDebug       bool
}

// WithVersion configures a fixed asset version.
func WithVersion(version string) Option {
	return func(renderer *Renderer) error {
		renderer.version = version
		return nil
	}
}

// WithVersionProvider configures a request-scoped asset version provider.
func WithVersionProvider(provider VersionProvider) Option {
	return func(renderer *Renderer) error {
		renderer.versionProvider = provider
		return nil
	}
}

// WithShared configures static shared props. Renderer construction copies props.
func WithShared(props Props) Option {
	return func(renderer *Renderer) error {
		maps.Copy(renderer.shared, props)
		return nil
	}
}

// WithSharedProvider adds a deterministic request-scoped shared prop provider.
func WithSharedProvider(provider SharedProvider) Option {
	return func(renderer *Renderer) error {
		if provider == nil {
			return fmt.Errorf("inertia: shared provider cannot be nil")
		}

		renderer.sharedProviders = append(renderer.sharedProviders, provider)
		return nil
	}
}

// WithFlashProvider adds a request-scoped top-level flash provider.
func WithFlashProvider(provider FlashProvider) Option {
	return func(renderer *Renderer) error {
		if provider == nil {
			return fmt.Errorf("inertia: flash provider cannot be nil")
		}

		renderer.requestFlash = append(renderer.requestFlash, provider)
		return nil
	}
}

// WithSSRRenderer configures the SSR gateway used by pages that server-render:
// every page when WithSSRDefault(true) is set, otherwise the pages that call
// PageBuilder.SSR().
func WithSSRRenderer(ssr SSRRenderer) Option {
	return func(renderer *Renderer) error {
		renderer.ssr = ssr
		renderer.customSSR = ssr != nil
		return nil
	}
}

// WithSSRDefault server-renders every initial document response unless the
// page opts out with PageBuilder.CSR(). It is off by default so existing
// applications keep the per-page PageBuilder.SSR() opt-in.
func WithSSRDefault(enabled bool) Option {
	return func(renderer *Renderer) error {
		renderer.ssrDefault = enabled
		return nil
	}
}

// WithSSRFailFast makes per-response SSR failures abort the initial response.
func WithSSRFailFast(enabled bool) Option {
	return func(renderer *Renderer) error {
		renderer.ssrFailFast = enabled
		return nil
	}
}

// WithReflash configures redirect-time flash preservation. The callback runs
// after an Inertia handler returns a redirect and before the response commits.
func WithReflash(handler ReflashHandler) Option {
	return func(renderer *Renderer) error {
		if handler == nil {
			return fmt.Errorf("inertia: reflash handler cannot be nil")
		}

		renderer.reflash = handler
		return nil
	}
}

// WithProtocolDebug emits request classification and page metadata without
// logging prop values. It is intended for local protocol diagnostics.
func WithProtocolDebug(enabled bool) Option {
	return func(renderer *Renderer) error {
		renderer.protocolDebug = enabled
		return nil
	}
}

// SetReflashHandler configures flash preservation before the renderer starts
// serving requests. Generated applications use this at router construction.
func (renderer *Renderer) SetReflashHandler(handler ReflashHandler) error {
	if renderer == nil {
		return fmt.Errorf("inertia: renderer is nil")
	}

	if handler == nil {
		return fmt.Errorf("inertia: reflash handler cannot be nil")
	}

	renderer.reflash = handler
	return nil
}

// PageBuilder configures and renders a single Inertia page response.
type PageBuilder struct {
	renderer         *Renderer
	ctx              context.Context
	component        string
	props            Props
	status           int
	url              string
	ssr              bool
	csr              bool
	validationErrors map[string]string
	encryptHistory   bool
	clearHistory     bool
	preserveFragment bool
	flash            any
}

// SSR opts this initial document response into server-side rendering, even
// when the renderer-wide default is off.
func (p *PageBuilder) SSR() *PageBuilder { p.ssr = true; return p }

// CSR opts this initial document response out of server-side rendering, even
// when the renderer-wide default is on. PageBuilder.SSR() wins if both are set.
func (p *PageBuilder) CSR() *PageBuilder { p.csr = true; return p }

// serverRendered reports the effective SSR policy of this response.
func (p *PageBuilder) serverRendered() bool {
	return p.ssr || (p.renderer.ssrDefault && !p.csr)
}

// Status sets the page response HTTP status code.
func (p *PageBuilder) Status(status int) *PageBuilder {
	p.status = status
	return p
}

// URL overrides the page object's URL. It is for handlers that render after a
// POST and want the browser address to be a GET-able route; by default the
// page URL is the request URI.
func (p *PageBuilder) URL(url string) *PageBuilder {
	p.url = url
	return p
}

// ValidationErrors sets the protected errors prop for this response.
func (p *PageBuilder) ValidationErrors(errors map[string]string) *PageBuilder {
	p.validationErrors = errors
	return p
}

// Errors sets the protected errors prop from err's field map and reports
// whether err carried field errors.
func (p *PageBuilder) Errors(err error) (*PageBuilder, bool) {
	fields, ok := fieldErrors(err)
	if !ok {
		return p, false
	}

	p.validationErrors = fields

	return p, true
}

// HistoryEncryption controls encrypted browser history metadata.
func (p *PageBuilder) HistoryEncryption(enabled bool) *PageBuilder {
	p.encryptHistory = enabled
	return p
}

// HistoryClear clears client history for this response.
func (p *PageBuilder) HistoryClear() *PageBuilder { p.clearHistory = true; return p }

// PreserveFragment preserves the original fragment across a redirect.
func (p *PageBuilder) PreserveFragment() *PageBuilder { p.preserveFragment = true; return p }

// Flash adds v3 flash data to the page field (not props).
func (p *PageBuilder) Flash(flash any) *PageBuilder { p.flash = flash; return p }

// Page starts building one Inertia page response.
func (renderer *Renderer) Page(
	ctx context.Context,
	component string,
	props Props,
) *PageBuilder {
	return &PageBuilder{
		renderer:  renderer,
		ctx:       ctx,
		component: component,
		props:     props,
		status:    http.StatusOK,
	}
}

// Render resolves and renders the Inertia page response.
func (p *PageBuilder) Render() error {
	renderer := p.renderer
	ctx := p.ctx
	component := p.component
	request, err := requestState(ctx)
	if err != nil {
		return err
	}
	method := anduin.Request(ctx).Method

	shared := make(Props, len(renderer.shared))
	maps.Copy(shared, renderer.shared)
	for _, provider := range renderer.sharedProviders {
		provided, err := provider(ctx)
		if err != nil {
			return &Error{
				Kind:      ErrorProps,
				Operation: "resolve shared provider",
				Method:    method,
				URL:       requestURL(ctx),
				Err:       err,
			}
		}
		maps.Copy(shared, provided)
	}

	resolved, err := resolvePageProps(ctx, request, component, shared, p.props)
	if err != nil {
		return err
	}

	errorsProp := map[string]any{}
	if p.validationErrors != nil {
		plain := make(map[string]any, len(p.validationErrors))
		for field, message := range p.validationErrors {
			plain[field] = message
		}
		if request.ErrorBag == "" {
			errorsProp = plain
		} else {
			errorsProp = map[string]any{request.ErrorBag: plain}
		}
	}
	resolved.props["errors"] = errorsProp
	resolved.resolvedShared = append(resolved.resolvedShared, "errors")
	slices.Sort(resolved.resolvedShared)

	version, err := renderer.currentVersion(ctx)
	if err != nil {
		return err
	}

	if p.flash == nil {
		for _, provider := range renderer.requestFlash {
			if p.flash = provider(ctx); p.flash != nil {
				break
			}
		}
	}

	pageURL := p.url
	if pageURL == "" {
		pageURL = requestURL(ctx)
	}
	page := Page{
		Component:        component,
		Props:            resolved.props,
		URL:              pageURL,
		Version:          version,
		EncryptHistory:   p.encryptHistory,
		ClearHistory:     p.clearHistory,
		PreserveFragment: p.preserveFragment,
		MergeProps:       resolved.merge,
		PrependProps:     resolved.prepend,
		DeepMergeProps:   resolved.deepMerge,
		MatchPropsOn:     resolved.matchOn,
		ScrollProps:      emptyNil(resolved.scroll),
		DeferredProps:    emptyNil(resolved.deferred),
		RescuedProps:     resolved.rescued,
		SharedProps:      resolved.resolvedShared,
		OnceProps:        emptyNil(resolved.once),
		Flash:            p.flash,
	}

	if renderer.protocolDebug {
		propKeys := make([]string, 0, len(page.Props))
		for key := range page.Props {
			propKeys = append(propKeys, key)
		}
		slices.Sort(propKeys)
		deferredGroups := make([]string, 0, len(page.DeferredProps))
		for key := range page.DeferredProps {
			deferredGroups = append(deferredGroups, key)
		}
		slices.Sort(deferredGroups)
		slog.Default().Debug("inertia protocol page",
			slog.String("component", component),
			slog.String("method", method),
			slog.String("url", page.URL),
			slog.Any("prop_keys", propKeys),
			slog.Any("merge_props", page.MergeProps),
			slog.Any("deferred_groups", deferredGroups),
			slog.Int("once_props", len(page.OnceProps)),
		)
	}

	var pageBuffer bytes.Buffer
	encoder := json.NewEncoder(&pageBuffer)
	if err := encoder.Encode(page); err != nil {
		return &Error{
			Kind:      ErrorProps,
			Operation: "encode page",
			Component: component,
			Method:    method,
			URL:       page.URL,
			Err:       err,
		}
	}
	pageJSON := bytes.TrimSuffix(pageBuffer.Bytes(), []byte("\n"))

	anduin.AddVary(anduin.ResponseWriter(ctx).Header(), HeaderInertia)

	if request.Inertia {
		anduin.SetHeader(ctx, HeaderInertia, "true")
		return anduin.JSONBlob(ctx, p.status, pageJSON)
	}

	var ssrResponse *SSRResponse
	if p.serverRendered() {
		if renderer.ssr == nil {
			err = fmt.Errorf("SSR requested without a configured renderer")
		} else {
			ssrResponse, err = renderer.ssr.Render(anduin.Request(ctx).Context(), page)
			if err == nil {
				switch {
				case ssrResponse == nil:
					err = fmt.Errorf("empty SSR response")
				case !strings.Contains(ssrResponse.Body, `data-server-rendered="true"`):
					err = fmt.Errorf("SSR body is missing data-server-rendered marker")
				case !strings.Contains(ssrResponse.Body, "data-page="):
					err = fmt.Errorf("SSR body is missing page script")
				}
			}
		}
		if err != nil {
			wrapped := &Error{
				Kind:      ErrorSSR,
				Operation: "render",
				Component: component,
				Method:    method,
				URL:       page.URL,
				Err:       err,
			}
			if renderer.ssrFailFast {
				return wrapped
			}
			slog.Default().
				Error("inertia SSR fallback", slog.String("component", component), slog.String("url", page.URL), slog.Any("error", wrapped))
			ssrResponse = nil
		}
	}

	root := renderer.root(RootData{
		Page:        page,
		PageJSON:    pageJSON,
		ContainerID: renderer.containerID,
		ProjectName: renderer.projectName,
		Environment: renderer.environment,
		ViteHead:    template.HTML(renderer.viteTags.head),
		ViteBody:    template.HTML(renderer.viteTags.body),
		SSR:         ssrResponse,
	})
	if root == nil {
		return &Error{
			Kind:      ErrorRoot,
			Operation: "construct",
			Component: component,
			Method:    method,
			URL:       page.URL,
			Err:       fmt.Errorf("root returned nil component"),
		}
	}

	var document bytes.Buffer
	if err := root.Render(anduin.Request(ctx).Context(), &document); err != nil {
		return &Error{
			Kind:      ErrorRoot,
			Operation: "render",
			Component: component,
			Method:    method,
			URL:       page.URL,
			Err:       err,
		}
	}

	return anduin.HTMLBlob(ctx, p.status, document.Bytes())
}

func (renderer *Renderer) currentVersion(ctx context.Context) (string, error) {
	if renderer.versionProvider == nil {
		return renderer.version, nil
	}

	version, err := renderer.versionProvider(ctx)
	if err != nil {
		return "", &Error{
			Kind:      ErrorProtocol,
			Operation: "resolve version",
			Method:    anduin.Request(ctx).Method,
			URL:       requestURL(ctx),
			Err:       err,
		}
	}

	return version, nil
}

func requestURL(ctx context.Context) string {
	request := anduin.Request(ctx)
	if request == nil || request.URL == nil {
		return "/"
	}

	uri := request.URL.RequestURI()
	if uri == "" {
		return "/"
	}
	return uri
}

func emptyNil[K comparable, V any](values map[K]V) map[K]V {
	if len(values) == 0 {
		return nil
	}
	return values
}
