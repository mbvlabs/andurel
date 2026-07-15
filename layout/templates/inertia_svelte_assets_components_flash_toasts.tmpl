<script lang="ts">
  import { router } from '@inertiajs/svelte'

  type FlashMessage = {
    Type: string
    Message: string
  }

  type Toast = FlashMessage & { id: number }

  let { initialFlashes }: { initialFlashes?: unknown } = $props()
  let toasts = $state<Toast[]>([])
  let nextId = 0

  function isFlashMessage(value: unknown): value is FlashMessage {
    if (!value || typeof value !== 'object') return false

    const flash = value as Record<string, unknown>
    return typeof flash.Type === 'string' && typeof flash.Message === 'string'
  }

  function pushFlashes(value: unknown) {
    if (!Array.isArray(value)) return

    for (const flash of value.filter(isFlashMessage)) {
      const id = nextId++
      toasts.push({ ...flash, id })
      window.setTimeout(() => {
        toasts = toasts.filter((toast) => toast.id !== id)
      }, 5000)
    }
  }

  $effect(() => {
    pushFlashes(initialFlashes)
    const removeListener = router.on('success', (event) => {
      pushFlashes(event.detail.page.props.flash)
    })

    return removeListener
  })
</script>

{#if toasts.length > 0}
  <div class="fixed bottom-4 right-4 z-50">
    {#each toasts as toast (toast.id)}
      <div
        class={`${toast.Type === 'success'
          ? 'border-[#8df7a4] text-[#8df7a4]'
          : toast.Type === 'error'
            ? 'border-[#ff875f] text-[#ff875f]'
            : 'border-[#ff6b1a] text-[#e4dfd2]'} mb-2 border bg-[#101414] px-4 py-3 shadow-lg shadow-black/40 transition-opacity duration-300`}
      >
        {toast.Message}
      </div>
    {/each}
  </div>
{/if}
