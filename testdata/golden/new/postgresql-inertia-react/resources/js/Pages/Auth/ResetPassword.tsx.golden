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
      <section className="w-full max-w-md border border-[#2f3a37] bg-[#101414]/90 shadow-sm shadow-black/40">
        <div className="p-6 pb-0">
          <h1 className="text-xl font-semibold text-[#f2ead8]">Reset Your Password</h1>
          <p className="mt-1 text-sm text-[#8f8a7d]">Enter your new password below.</p>
        </div>
        <div className="p-6">
          <form onSubmit={(event) => submit(event.nativeEvent as SubmitEvent)} className="space-y-5">
            {errors.resetPasswordToken && <p className="text-sm font-medium text-[#ff875f]">{errors.resetPasswordToken}</p>}
            <div className="space-y-1">
              <label className="text-sm font-medium text-[#c7c0ad]" htmlFor="password">New Password</label>
              <input id="password" type="password" value={form.data.password} onChange={(event) => form.setData('password', event.target.value)} className="flex h-9 w-full border border-[#2f3a37] bg-[#090c0d] px-3 py-1 text-sm text-[#e4dfd2] shadow-inner shadow-black/35 focus:border-[#8df7a4] focus:outline-none focus:ring-2 focus:ring-[#8df7a4]/20" required />
              {errors.password && <p className="text-sm font-medium text-[#ff875f]">{errors.password}</p>}
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium text-[#c7c0ad]" htmlFor="confirmPassword">Confirm New Password</label>
              <input id="confirmPassword" type="password" value={form.data.confirmPassword} onChange={(event) => form.setData('confirmPassword', event.target.value)} className="flex h-9 w-full border border-[#2f3a37] bg-[#090c0d] px-3 py-1 text-sm text-[#e4dfd2] shadow-inner shadow-black/35 focus:border-[#8df7a4] focus:outline-none focus:ring-2 focus:ring-[#8df7a4]/20" required />
              {errors.confirmPassword && <p className="text-sm font-medium text-[#ff875f]">{errors.confirmPassword}</p>}
            </div>
            <button type="submit" disabled={form.processing} className="inline-flex w-full items-center justify-center bg-[#ff6b1a] px-4 py-2 text-sm font-medium text-[#130f0b] shadow-sm shadow-black/40 hover:bg-[#ff8748] disabled:opacity-60">{form.processing ? 'Loading' : 'Reset Password'}</button>
          </form>
        </div>
      </section>
    </Layout>
  )
}
