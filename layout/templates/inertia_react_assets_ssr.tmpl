import { createInertiaApp, type ResolvedComponent } from '@inertiajs/react'
import createServer from '@inertiajs/react/server'
import ReactDOMServer from 'react-dom/server'

type PageModule = {
  default: ResolvedComponent
}

const serverOptions = {
  host: process.env.INERTIA_SSR_HOST ?? '127.0.0.1',
  port: Number(process.env.INERTIA_SSR_PORT ?? '13714'),
}

createServer(page =>
  createInertiaApp({
    page,
    render: ReactDOMServer.renderToString,
    resolve: (name: string) => {
      const pages = import.meta.glob<PageModule>('./Pages/**/*.tsx', { eager: true })
      return pages[`./Pages/${name}.tsx`].default
    },
    setup: ({ App, props }) => <App {...props} />,
  }),
  serverOptions,
)
