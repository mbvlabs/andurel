import Layout from '@/Layouts/Layout'

type ValidationProps = {
  errors?: Record<string, string>
}

export default function Validation({ errors = {} }: ValidationProps) {
  return (
    <Layout>
      <section className="w-full max-w-md border border-border bg-card/90 p-6 text-center shadow-sm shadow-black/40">
        <p className="text-sm font-medium uppercase tracking-wide text-accent">422</p>
        <h1 className="mt-2 text-2xl font-semibold text-card-foreground">Validation failed</h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">Please correct the highlighted fields and try again.</p>
        {Object.keys(errors).length > 0 && (
          <ul className="mt-4 space-y-1 text-left text-sm font-medium text-destructive">
            {Object.entries(errors).map(([field, message]) => (
              <li key={field}>{message}</li>
            ))}
          </ul>
        )}
      </section>
    </Layout>
  )
}
