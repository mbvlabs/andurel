import { Link, useForm } from '@inertiajs/react'

import Layout from '@/Layouts/Layout'
import { routes } from '@/routes'

type ResetPasswordRequestProps = {
  errors?: Record<string, string>
}

export default function ResetPasswordRequest({ errors = {} }: ResetPasswordRequestProps) {
  const form = useForm({ email: '' })

  function submit(event: SubmitEvent) {
    event.preventDefault()
    form.post(routes.passwordCreate())
  }

  return (
    <Layout>
      <section className="w-full max-w-md border border-border bg-card/90 shadow-sm shadow-black/40">
        <div className="p-6 pb-0">
          <h1 className="text-xl font-semibold text-card-foreground">Reset Password</h1>
          <p className="mt-1 text-sm text-muted-foreground">Enter your email address and we'll send you a code to reset your password.</p>
        </div>
        <div className="p-6">
          <form onSubmit={(event) => submit(event.nativeEvent as SubmitEvent)} className="space-y-5">
            <div className="space-y-1">
              <label className="text-sm font-medium text-muted-foreground" htmlFor="email">Email</label>
              <input id="email" type="email" value={form.data.email} onChange={(event) => form.setData('email', event.target.value)} className="flex h-9 w-full border border-border bg-background px-3 py-1 text-sm text-foreground shadow-inner shadow-black/35 focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/20" required />
              {errors.email && <p className="text-sm font-medium text-destructive">{errors.email}</p>}
            </div>
            <button type="submit" disabled={form.processing} className="inline-flex w-full items-center justify-center bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-sm shadow-black/40 hover:bg-primary/90 disabled:opacity-60">{form.processing ? 'Loading' : 'Send Reset Code'}</button>
          </form>
          <p className="mt-6 text-center text-sm text-muted-foreground">Remember your password? <Link className="text-foreground hover:text-card-foreground hover:underline" href={routes.sessionNew()}>Login</Link></p>
        </div>
      </section>
    </Layout>
  )
}
