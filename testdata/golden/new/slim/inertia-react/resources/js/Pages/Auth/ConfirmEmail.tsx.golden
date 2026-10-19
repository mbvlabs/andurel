import { useForm } from '@inertiajs/react'

import Layout from '@/Layouts/Layout'
import { routes } from '@/routes'

type ConfirmEmailProps = {
  errors?: Record<string, string>
}

export default function ConfirmEmail({ errors = {} }: ConfirmEmailProps) {
  const form = useForm({ code: '' })

  function submit(event: SubmitEvent) {
    event.preventDefault()
    form.post(routes.confirmationCreate())
  }

  return (
    <Layout>
      <section className="w-full max-w-md border border-border bg-card/90 shadow-sm shadow-black/40">
        <div className="p-6 pb-0">
          <h1 className="text-xl font-semibold text-card-foreground">Verify Your Email</h1>
          <p className="mt-1 text-sm text-muted-foreground">Please enter the 6-digit verification code sent to your email.</p>
        </div>
        <div className="p-6">
          <form onSubmit={(event) => submit(event.nativeEvent as SubmitEvent)} className="space-y-5">
            <div className="space-y-1">
              <label className="text-sm font-medium text-muted-foreground" htmlFor="code">Verification Code</label>
              <input id="code" type="text" maxLength={6} value={form.data.code} onChange={(event) => form.setData('code', event.target.value)} className="flex h-9 w-full border border-border bg-background px-3 py-1 text-center text-sm tracking-[0.3em] text-foreground shadow-inner shadow-black/35 focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/20" required />
              {errors.code && <p className="text-sm font-medium text-destructive">{errors.code}</p>}
            </div>
            <button type="submit" disabled={form.processing} className="inline-flex w-full items-center justify-center bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-sm shadow-black/40 hover:bg-primary/90 disabled:opacity-60">{form.processing ? 'Loading' : 'Verify Email'}</button>
          </form>
        </div>
      </section>
    </Layout>
  )
}
