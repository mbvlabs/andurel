import { createInertiaApp, type ResolvedComponent } from '@inertiajs/react'
import createServer from '@inertiajs/react/server'
import ReactDOMServer from 'react-dom/server'
import { AsyncLocalStorage } from 'node:async_hooks'
import { configureRouteHostScope, withRouteHosts } from './routes'

type PageModule = {
  default: ResolvedComponent
}

// Route origins are per request: each render runs inside its own host context,
// so two concurrent renders for different hosts cannot share module state.
const routeHosts = new AsyncLocalStorage<Record<string, string>>()

configureRouteHostScope({
  current: () => routeHosts.getStore(),
  run: (hosts, render) => routeHosts.run(hosts, render),
})

const serverOptions = {
  host: process.env.INERTIA_SSR_HOST ?? '127.0.0.1',
  port: Number(process.env.INERTIA_SSR_PORT ?? '13714'),
}

createServer(
  page =>
    withRouteHosts((page.props.hosts ?? {}) as Record<string, string>, () =>
      createInertiaApp({
        page,
        render: ReactDOMServer.renderToString,
        resolve: (name: string) => {
          const pages = import.meta.glob<PageModule>('./Pages/**/*.tsx', { eager: true })
          return pages[`./Pages/${name}.tsx`].default
        },
        setup: ({ App, props }) => <App {...props} />,
      }),
    ),
  serverOptions,
)
