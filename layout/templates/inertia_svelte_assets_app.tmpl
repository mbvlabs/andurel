import '../../css/base.css'

import { createInertiaApp, type ResolvedComponent } from '@inertiajs/svelte'
import { mount } from 'svelte'

import FlashToasts from '@/Components/FlashToasts.svelte'

createInertiaApp({
  resolve: (name: string) => {
    const pages = import.meta.glob<ResolvedComponent>('./Pages/**/*.svelte', { eager: true })
    return pages[`./Pages/${name}.svelte`]
  },
  setup({ el, App, props }) {
    if (!el) return

    mount(App, { target: el, props })
    mount(FlashToasts, {
      target: document.body,
      props: { initialFlashes: props.initialPage.props.flash },
    })
  },
})
