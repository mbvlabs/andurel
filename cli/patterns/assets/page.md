# Page

An Inertia page is a Go handler call plus a component file. The handler owns
the props and the status; the component owns presentation. Pages live under
`resources/js/Pages/`, layouts under `resources/js/Layouts/`, and reusable
components under `resources/js/Components/` or `resources/js/components/ui/`.

The component string passed to `renderer.Page` is the page's path relative to
`resources/js/Pages/`, without the extension and case-sensitive:
`"Product/Index"` resolves to `resources/js/Pages/Product/Index.tsx` (React),
`.vue` (Vue), or `.svelte` (Svelte). Keep the file name and the Go string in
sync; the adapter's `import.meta.glob('./Pages/**/*.<ext>')` is the resolver.

## Files

### handlers/products.go (edit)
```go fragment
package handlers

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/inertia"
)

// ProductIndexProps is the page contract: one exported field per prop, with
// the JSON name the component receives.
type ProductIndexProps struct {
	Products []domains.Product `json:"products"`
}

func (h ProductHandler) Index(ctx context.Context) error {
	result, err := h.products.Paginate(ctx, 1, 25)
	if err != nil {
		return err
	}

	return h.renderer.Page(ctx, "Product/Index", inertia.FromStruct(ProductIndexProps{
		Products: result.Items,
	})).Render()
}
```

### resources/js/Pages/Product/Index.tsx
```tsx
import { Link } from '@inertiajs/react'

import Layout from '@/Layouts/Layout'
import { routes } from '@/routes'

type Product = {
  id: string
  name: string
  priceCents: number
}

type ProductIndexProps = {
  products: Product[]
}

export default function ProductIndex({ products }: ProductIndexProps) {
  return (
    <Layout>
      <div className="mx-auto w-full max-w-[960px] px-4 py-4">
        <h1 className="text-2xl font-semibold text-card-foreground">Products</h1>
        <ul className="mt-4 divide-y divide-border border border-border bg-card/80">
          {products.map((product) => (
            <li key={product.id} className="flex items-center justify-between gap-3 p-3">
              <span className="text-sm text-card-foreground">{product.name}</span>
              <Link className="text-sm text-accent" href={routes.productShow.path(product.id)}>
                View
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </Layout>
  )
}
```

### resources/js/Pages/Product/Index.vue
```vue
<script setup lang="ts">
import { Link } from '@inertiajs/vue3'

import Layout from '@/Layouts/Layout.vue'
import { routes } from '@/routes'

type Product = {
  id: string
  name: string
  priceCents: number
}

defineProps<{
  products: Product[]
}>()
</script>

<template>
  <Layout>
    <div class="mx-auto w-full max-w-[960px] px-4 py-4">
      <h1 class="text-2xl font-semibold text-card-foreground">Products</h1>
      <ul class="mt-4 divide-y divide-border border border-border bg-card/80">
        <li
          v-for="product in products"
          :key="product.id"
          class="flex items-center justify-between gap-3 p-3"
        >
          <span class="text-sm text-card-foreground">{{ product.name }}</span>
          <Link class="text-sm text-accent" :href="routes.productShow.path(product.id)">
            View
          </Link>
        </li>
      </ul>
    </div>
  </Layout>
</template>
```

### resources/js/Pages/Product/Index.svelte
```svelte
<script lang="ts">
  import { Link } from '@inertiajs/svelte'

  import Layout from '@/Layouts/Layout.svelte'
  import { routes } from '@/routes'

  type Product = {
    id: string
    name: string
    priceCents: number
  }

  let { products }: { products: Product[] } = $props()
</script>

<Layout>
  <div class="mx-auto w-full max-w-[960px] px-4 py-4">
    <h1 class="text-2xl font-semibold text-card-foreground">Products</h1>
    <ul class="mt-4 divide-y divide-border border border-border bg-card/80">
      {#each products as product (product.id)}
        <li class="flex items-center justify-between gap-3 p-3">
          <span class="text-sm text-card-foreground">{product.name}</span>
          <Link class="text-sm text-accent" href={routes.productShow.path(product.id)}>
            View
          </Link>
        </li>
      {/each}
    </ul>
  </div>
</Layout>
```

## Rules

- One component file per page, named after the `renderer.Page` component
  string: `Product/Index` → `Pages/Product/Index.tsx` (React), `.vue` (Vue), or
  `.svelte` (Svelte). The default scaffold adapter is React.
- Pass props from Go with `inertia.Props{...}` for a small map or
  `inertia.FromStruct(SomeProps{...})` for a typed contract. Only exported
  fields become props; each uses its `json` name (or the Go field name when
  untagged), and `json:"-"` is skipped. Do not pass a domain entity if the page
  needs a subset — define a page-owned props struct.
- Every page also receives the shared props: `auth.isAuthenticated` from the
  scaffold's shared provider, `hosts` (feed it to `configureRouteHosts` at
  boot), `appVersion`, and the `errors` map (empty on success, populated on a
  validation re-render). Read them through the page's props; do not hard-code a
  user, host, or asset URL.
- Wrap the page in the adapter's layout (`Layout.tsx`, `Layout.vue`, or
  `Layout.svelte`). Layouts do not receive page props; they read shared props
  with `usePage()` (React/Vue) or `page.props` (Svelte). A new layout belongs in
  `resources/js/Layouts/`, not in `Pages/`.
- Import URLs from `resources/js/routes.ts` after `andurel sync routes`; use
  `routes.name.path(...)` in `href` and `routes.name(...)` for an absolute URL.
  Build them at render time, after `configureRouteHosts` has run.
- `resources/root.templ` is the framework-owned document. It receives
  `inertia.RootData` fields (`Page`, `PageJSON`, `ContainerID`,
  `ProjectName`, `Environment`, `ViteHead`, `ViteBody`, `SSR`) and must not be
  used to smuggle page data: page props belong to the page.
- Server-side rendering is opt-in per render with `.SSR()` on the Go
  `PageBuilder` (for example
  `renderer.Page(...).SSR().Render()`); the component needs no change. Error
  and validation re-renders need their own `.SSR()` if they should stay
  server-rendered.
- Presentation adapters stay in the component: label maps, initials, date
  labels, and JSON-to-list formatting do not belong in `domains/`.

## Checklist

- [ ] Component file path matches the `renderer.Page` component string for the
      project's adapter.
- [ ] Props are a named Go struct or an explicit map; no entity leaks by
      accident.
- [ ] Shared `auth`, `hosts`, `appVersion`, and `errors` are read, not
      re-invented.
- [ ] The page is wrapped in a layout, and reusable pieces live in
      `Components/`.
- [ ] URLs come from `@/routes` after `andurel sync routes`.
- [ ] The page renders in dev (`andurel run`) and, when opted in, under SSR.
