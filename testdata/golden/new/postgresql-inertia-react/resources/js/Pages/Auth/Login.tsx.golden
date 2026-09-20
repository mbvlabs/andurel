import { Link, useForm } from '@inertiajs/react'

import Layout from '@/Layouts/Layout'
import { routes } from '@/routes'

type LoginProps = {
  errors?: Record<string, string>
}

export default function Login({ errors = {} }: LoginProps) {
  const form = useForm({ email: '', password: '' })

  function submit(event: SubmitEvent) {
    event.preventDefault()
    form.post(routes.sessionCreate())
  }

  return (
    <Layout>
      <section className="w-full max-w-md border border-[#2f3a37] bg-[#101414]/90 shadow-sm shadow-black/40">
        <div className="p-6 pb-0">
          <h1 className="text-xl font-semibold text-[#f2ead8]">Login to your account</h1>
          <p className="mt-1 text-sm text-[#8f8a7d]">Enter your details below to login to your account</p>
        </div>
        <div className="p-6">
          <form onSubmit={(event) => submit(event.nativeEvent as SubmitEvent)} className="space-y-5">
            <div className="space-y-1">
              <label className="text-sm font-medium text-[#c7c0ad]" htmlFor="email">Email</label>
              <input id="email" type="email" value={form.data.email} onChange={(event) => form.setData('email', event.target.value)} className="flex h-9 w-full border border-[#2f3a37] bg-[#090c0d] px-3 py-1 text-sm text-[#e4dfd2] shadow-inner shadow-black/35 focus:border-[#8df7a4] focus:outline-none focus:ring-2 focus:ring-[#8df7a4]/20" required />
              {errors.email && <p className="text-sm font-medium text-[#ff875f]">{errors.email}</p>}
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium text-[#c7c0ad]" htmlFor="password">Password</label>
              <input id="password" type="password" value={form.data.password} onChange={(event) => form.setData('password', event.target.value)} className="flex h-9 w-full border border-[#2f3a37] bg-[#090c0d] px-3 py-1 text-sm text-[#e4dfd2] shadow-inner shadow-black/35 focus:border-[#8df7a4] focus:outline-none focus:ring-2 focus:ring-[#8df7a4]/20" required />
              {errors.password && <p className="text-sm font-medium text-[#ff875f]">{errors.password}</p>}
            </div>
            <p className="text-right text-sm"><Link className="text-[#d7d0bf] hover:text-[#f2ead8] hover:underline" href={routes.passwordNew()}>Forgot your password?</Link></p>
            <button type="submit" disabled={form.processing} className="inline-flex w-full items-center justify-center bg-[#ff6b1a] px-4 py-2 text-sm font-medium text-[#130f0b] shadow-sm shadow-black/40 hover:bg-[#ff8748] disabled:opacity-60">{form.processing ? 'Loading' : 'Login'}</button>
          </form>
          <p className="mt-6 text-center text-sm text-[#8f8a7d]">Don't have an account? <Link className="text-[#d7d0bf] hover:text-[#f2ead8] hover:underline" href={routes.registrationNew()}>Sign up</Link></p>
        </div>
      </section>
    </Layout>
  )
}
