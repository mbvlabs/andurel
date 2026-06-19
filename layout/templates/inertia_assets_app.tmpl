import '../../css/base.css'

import { createApp, h, defineComponent, ref, watch, type DefineComponent } from 'vue'
import { createInertiaApp, usePage } from '@inertiajs/vue3'

type FlashMessage = {
  Type: string
  Message: string
}

const FlashToasts = defineComponent({
  setup() {
    const toasts = ref<Array<FlashMessage & { id: number }>>([])
    let nextId = 0

    watch(
      () => usePage().props.flash as FlashMessage[] | undefined,
      (flashes) => {
        if (!flashes || flashes.length === 0) {return}
        for (const f of flashes) {
          const id = nextId++
          toasts.value.push({ ...f, id })
          setTimeout(() => {
            toasts.value = toasts.value.filter(t => t.id !== id)
          }, 5000)
        }
      },
      { immediate: true, deep: true },
    )

    return () => {
      if (toasts.value.length === 0) {return null}
      const items = toasts.value.map(t => {
        const color = t.Type === 'success' ? 'bg-green-600' : t.Type === 'error' ? 'bg-red-600' : 'bg-blue-600'
        return h('div', {
          key: t.id,
          class: `${color} text-white px-4 py-3 rounded-lg shadow-lg mb-2 transition-opacity duration-300`,
        }, t.Message)
      })
      return h('div', { class: 'fixed bottom-4 right-4 z-50' }, items)
    }
  },
})

createInertiaApp({
  resolve: (name: string): DefineComponent => {
    const pages = import.meta.glob<DefineComponent>('./Pages/**/*.vue', { eager: true })
    return pages[`./Pages/${name}.vue`]
  },
  setup({ el, App, props, plugin }) {
    createApp({
      render() {
        return h('div', [
          h(App, props),
          h(FlashToasts),
        ])
      },
    })
      .use(plugin)
      .mount(el)
  },
})
