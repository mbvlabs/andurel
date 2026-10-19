import { useForm } from '@inertiajs/react'

import Layout from '@/Layouts/Layout'
import { routes } from '@/routes'

type ResetPasswordProps = {
  token: string
  errors?: Record<string, string>
}

export default function ResetPassword({ token, errors = {} }: ResetPasswordProps) {
  const form = useForm({ resetPasswordToken: token, password: '', confirmPassword: '' })

  function submit(event: SubmitEvent) {
    event.preventDefault()
    form.put(routes.passwordUpdate())
  }

  return (
    <Layout>
      <section className="w-full max-w-md border border-border bg-card/90 shadow-sm shadow-black/40">
        <div className="p-6 pb-0">
          <h1 className="text-xl font-semibold text-card-foreground">Reset Your Password</h1>
          <p className="mt-1 text-sm text-muted-foreground">Enter your new password below.</p>
        </div>
        <div className="p-6">
          <form onSubmit={(event) => submit(event.nativeEvent as SubmitEvent)} className="space-y-5">
            {errors.resetPasswordToken && <p className="text-sm font-medium text-destructive">{errors.resetPasswordToken}</p>}
            <div className="space-y-1">
              <label className="text-sm font-medium text-muted-foreground" htmlFor="password">New Password</label>
              <input id="password" type="password" value={form.data.password} onChange={(event) => form.setData('password', event.target.value)} className="flex h-9 w-full border border-border bg-background px-3 py-1 text-sm text-foreground shadow-inner shadow-black/35 focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/20" required />
              {errors.password && <p className="text-sm font-medium text-destructive">{errors.password}</p>}
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium text-muted-foreground" htmlFor="confirmPassword">Confirm New Password</label>
              <input id="confirmPassword" type="password" value={form.data.confirmPassword} onChange={(event) => form.setData('confirmPassword', event.target.value)} className="flex h-9 w-full border border-border bg-background px-3 py-1 text-sm text-foreground shadow-inner shadow-black/35 focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/20" required />
              {errors.confirmPassword && <p className="text-sm font-medium text-destructive">{errors.confirmPassword}</p>}
            </div>
            <button type="submit" disabled={form.processing} className="inline-flex w-full items-center justify-center bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-sm shadow-black/40 hover:bg-primary/90 disabled:opacity-60">{form.processing ? 'Loading' : 'Reset Password'}</button>
          </form>
        </div>
      </section>
    </Layout>
  )
}
