import '../../css/base.css'

import { createRoot, hydrateRoot } from 'react-dom/client'
import { createInertiaApp, type ResolvedComponent } from '@inertiajs/react'

type PageProps = {
  [key: string]: unknown
}

type PageModule = {
  default: ResolvedComponent
}

createInertiaApp<PageProps>({
  resolve: (name: string) => {
    const pages = import.meta.glob<PageModule>('./Pages/**/*.tsx', { eager: true })
    return pages[`./Pages/${name}.tsx`].default
  },
  setup({ el, App, props }) {
    const app = <App {...props} />
    if (el.dataset.serverRendered === 'true') {
      hydrateRoot(el, app)
    } else {
      createRoot(el).render(app)
    }
  },
})
