<script lang="ts">
  import { Link, useForm } from '@inertiajs/svelte'

  import Layout from '@/Layouts/Layout.svelte'
  import { routes } from '@/routes'

  let { errors = {} }: { errors?: Record<string, string> } = $props()
  const form = useForm({ email: '' })

  function submit(event: SubmitEvent) {
    event.preventDefault()
    $form.post(routes.passwordCreate())
  }
</script>

<Layout>
  <section class="w-full max-w-md border border-[#2f3a37] bg-[#101414]/90 shadow-sm shadow-black/40">
    <div class="p-6 pb-0">
      <h1 class="text-xl font-semibold text-[#f2ead8]">Reset Password</h1>
      <p class="mt-1 text-sm text-[#8f8a7d]">Enter your email address and we'll send you a code to reset your password.</p>
    </div>
    <div class="p-6">
      <form class="space-y-5" onsubmit={submit}>
        <div class="space-y-1">
          <label class="text-sm font-medium text-[#c7c0ad]" for="email">Email</label>
          <input id="email" bind:value={$form.email} type="email" class="flex h-9 w-full border border-[#2f3a37] bg-[#090c0d] px-3 py-1 text-sm text-[#e4dfd2] shadow-inner shadow-black/35 focus:border-[#8df7a4] focus:outline-none focus:ring-2 focus:ring-[#8df7a4]/20" required />
          {#if errors.email}<p class="text-sm font-medium text-[#ff875f]">{errors.email}</p>{/if}
        </div>
        <button type="submit" disabled={$form.processing} class="inline-flex w-full items-center justify-center bg-[#ff6b1a] px-4 py-2 text-sm font-medium text-[#130f0b] shadow-sm shadow-black/40 hover:bg-[#ff8748] disabled:opacity-60">{$form.processing ? 'Loading' : 'Send Reset Code'}</button>
      </form>
      <p class="mt-6 text-center text-sm text-[#8f8a7d]">Remember your password? <Link class="text-[#d7d0bf] hover:text-[#f2ead8] hover:underline" href={routes.sessionNew()}>Login</Link></p>
    </div>
  </section>
</Layout>
