import { Link } from '@inertiajs/react'

import Layout from '@/Layouts/Layout'

type WelcomeProps = {
  appName: string
  docsURL: string
  registrationURL: string
  loginURL: string
}

export default function Welcome({
  appName,
  docsURL,
  registrationURL,
  loginURL,
}: WelcomeProps) {
  return (
    <Layout>
      <main className="relative flex flex-1 items-center justify-center overflow-hidden bg-background text-foreground">
        <div
          className="pointer-events-none absolute inset-0 opacity-60"
          style={{
            backgroundImage:
              'radial-gradient(circle at 12% 18%, #f2ead8 0 1px, transparent 1.5px), radial-gradient(circle at 82% 22%, #aaa393 0 1px, transparent 1.5px), radial-gradient(circle at 67% 72%, #f2ead8 0 1px, transparent 1.5px), radial-gradient(circle at 24% 83%, #8f8a7d 0 1px, transparent 1.5px)',
          }}
        />
        <div className="relative mx-auto w-full max-w-[960px] px-4 py-4">
          <section className="grid items-center gap-6 py-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
            <div className="max-w-3xl">
              <h1 className="text-4xl font-semibold text-card-foreground sm:text-5xl lg:text-6xl">
                Space-grade Go, wired locally.
              </h1>
              <p className="mt-5 max-w-2xl text-lg leading-7 text-muted-foreground">
                {appName} is ready with routing, controllers, validation, authentication,
                email, queues, and an Inertia frontend.
              </p>
              <div className="mt-8 flex flex-wrap gap-3">
                <a
                  className="bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground shadow-sm shadow-black/40 transition hover:bg-primary/90"
                  href={docsURL}
                >
                  Read the docs
                </a>
                <Link
                  className="border border-border bg-card/80 px-4 py-2 text-sm font-semibold text-foreground transition hover:border-border hover:text-card-foreground"
                  href={registrationURL}
                >
                  Create account
                </Link>
              </div>
            </div>
            <div className="border border-border bg-card/80 p-4 shadow-sm shadow-black/40">
              <div className="mb-4 flex items-center justify-between border-b border-border pb-3 text-xs uppercase text-muted-foreground">
                <span>deploy console</span>
                <span className="text-ring">ready</span>
              </div>
              <div className="space-y-4 font-mono text-sm">
                <ConsoleLine label="auth" value="sessions, registration, reset password" />
                <ConsoleLine label="pages" value="Inertia React scaffold online" />
                <ConsoleLine label="routes" value="slugged endpoints mounted" />
                <ConsoleLine label="jobs" value="queue worker ready" />
              </div>
            </div>
            <section className="grid gap-4 md:grid-cols-2 lg:col-span-2">
              <WelcomeCard
                title="Documentation"
                description="Start with the framework guides and learn the conventions that shape an Andurel app."
                href={docsURL}
              />
              <WelcomeCard
                title="Authentication"
                description="Registration, sessions, email confirmation, and password reset are already wired."
                href={registrationURL}
              />
              <WelcomeCard
                title="Inertia pages"
                description="Server-driven React pages with shared props, Vite, and optional SSR."
                href={loginURL}
              />
              <WelcomeCard
                title="Command line"
                description="Generate models, factories, controllers, routes, jobs, emails, and pages from the CLI."
                href={docsURL}
              />
            </section>
          </section>
        </div>
      </main>
    </Layout>
  )
}

function WelcomeCard({
  title,
  description,
  href,
}: {
  title: string
  description: string
  href: string
}) {
  return (
    <Link
      className="group border border-border bg-card/80 p-4 text-left shadow-sm shadow-black/40 transition hover:border-border"
      href={href}
    >
      <div className="mb-4 flex size-8 items-center justify-center border border-border bg-background text-ring transition group-hover:border-ring">
        <span className="size-2 bg-ring" />
      </div>
      <h2 className="text-base font-semibold text-card-foreground">{title}</h2>
      <p className="mt-2 text-sm leading-5 text-muted-foreground">{description}</p>
    </Link>
  )
}

function ConsoleLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start gap-3">
      <span className="mt-1 size-1.5 bg-ring" />
      <div>
        <p className="text-ring">{label}</p>
        <p className="text-muted-foreground">{value}</p>
      </div>
    </div>
  )
}
