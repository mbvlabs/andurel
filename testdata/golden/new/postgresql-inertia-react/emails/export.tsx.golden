import * as fs from "node:fs"
import * as path from "node:path"
import { fileURLToPath } from "node:url"
import type { ComponentType } from "react"
import { render } from "react-email"
import { emails } from "./generated/manifest"

const outputDir = path.join(path.dirname(fileURLToPath(import.meta.url)), "templates")

const placeholderCount = (value: string) => value.split("{{").length - 1

// Rendered output becomes a Go text/template. Every prop must be represented
// by its generated placeholder, and no other "{{" may appear: a literal "{{"
// in the authored TSX or CSS would break template parsing at render time.
const assertPlaceholders = (
  name: string,
  kind: string,
  output: string,
  vars: Record<string, unknown>,
) => {
  let expected = 0
  for (const [field, value] of Object.entries(vars)) {
    if (typeof value !== "string") {
      throw new Error(`${name}: ${kind} var ${field} is not a placeholder string`)
    }
    const occurrences = output.split(value).length - 1
    if (occurrences === 0) {
      throw new Error(
        `${name}: ${kind} output does not render the placeholder for ${field}. ` +
          `Render every prop somewhere in the template.`,
      )
    }
    expected += occurrences
  }
  const actual = placeholderCount(output)
  if (actual !== expected) {
    throw new Error(
      `${name}: ${kind} output contains ${actual} "{{" occurrences but only ` +
        `${expected} belong to generated placeholders. Remove literal "{{" ` +
        `from the authored TSX/CSS; Go text/template would parse it.`,
    )
  }
}

const exportEmail = async (email: (typeof emails)[number]) => {
  const Component = email.component as ComponentType<Record<string, unknown>>
  const html = await render(<Component {...email.htmlVars} />, {
    pretty: false,
  })
  const text = await render(<Component {...email.textVars} />, {
    pretty: false,
    plainText: true,
  })

  assertPlaceholders(email.name, "html", html, email.htmlVars)
  assertPlaceholders(email.name, "text", text, email.textVars)

  fs.writeFileSync(
    path.join(outputDir, `${email.name}.html.tmpl`),
    html,
    "utf8",
  )
  fs.writeFileSync(
    path.join(outputDir, `${email.name}.txt.tmpl`),
    text,
    "utf8",
  )
  console.log(
    `email:export ${email.name}: html ${Buffer.byteLength(html)}B, text ${Buffer.byteLength(text)}B`,
  )
}

const main = async () => {
  fs.mkdirSync(outputDir, { recursive: true })
  for (const email of emails) {
    await exportEmail(email)
  }
  console.log(`email:export wrote ${emails.length} email(s) to emails/templates`)
}

main().catch((error: unknown) => {
  console.error(error instanceof Error ? error.message : error)
  process.exitCode = 1
})
