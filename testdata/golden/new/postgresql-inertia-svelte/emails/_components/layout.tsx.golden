import type { ReactNode } from "react"
import {
  Body,
  Container,
  Head,
  Hr,
  Html,
  Preview,
  Section,
  Text,
} from "react-email"
import { darkTheme, theme } from "../theme"

// The reset below was ported from the former email stylesheet. Keep
// properties physical and colors hex: Gmail, Outlook and Yahoo drop
// padding-inline/block, oklch(), var() and rgb() space syntax. The dark block
// is a progressive enhancement for Apple Mail and Outlook.com; the light
// colors stay inline for every other client.
const emailStyles = `
html, body { margin: 0 !important; padding: 0 !important; width: 100% !important; }
table, td { border-collapse: collapse !important; mso-table-lspace: 0 !important; mso-table-rspace: 0 !important; }
img { border: 0; display: block; line-height: 100%; outline: none; text-decoration: none; }
.ExternalClass { width: 100%; }
.ExternalClass, .ExternalClass p, .ExternalClass span, .ExternalClass font, .ExternalClass td, .ExternalClass div { line-height: 100%; }
#MessageViewBody a { color: inherit; font-family: inherit; font-size: inherit; font-weight: inherit; line-height: inherit; text-decoration: none; }
@media (max-width: 620px) {
  .email-shell, .email-shell > tbody > tr > td { padding: 0 !important; }
  .email-card { border-radius: 0 !important; }
  .email-px, .email-px > tbody > tr > td { padding-left: 24px !important; padding-right: 24px !important; }
}
@media (prefers-color-scheme: dark) {
  .email-bg, body.email-bg > table > tbody > tr > td { background-color: ${darkTheme.background} !important; }
  .email-card { background-color: ${darkTheme.card} !important; }
  .email-fg { color: ${darkTheme.foreground} !important; }
  .email-muted { color: ${darkTheme.mutedForeground} !important; }
  .email-primary { color: ${darkTheme.primary} !important; }
  .email-muted-bg { background-color: ${darkTheme.background} !important; }
  .email-border { border-color: ${darkTheme.border} !important; }
}
[data-ogsb].email-bg, [data-ogsb] .email-bg, body[data-ogsb].email-bg > table > tbody > tr > td, [data-ogsb] body.email-bg > table > tbody > tr > td { background-color: ${darkTheme.background} !important; }
[data-ogsb].email-card, [data-ogsb] .email-card { background-color: ${darkTheme.card} !important; }
[data-ogsb].email-muted-bg, [data-ogsb] .email-muted-bg { background-color: ${darkTheme.background} !important; }
[data-ogsc].email-fg, [data-ogsc] .email-fg { color: ${darkTheme.foreground} !important; }
[data-ogsc].email-muted, [data-ogsc] .email-muted { color: ${darkTheme.mutedForeground} !important; }
[data-ogsc].email-primary, [data-ogsc] .email-primary { color: ${darkTheme.primary} !important; }
[data-ogsc].email-border, [data-ogsc] .email-border { border-color: ${darkTheme.border} !important; }
`

type LayoutProps = {
  title: string
  preview: string
  children: ReactNode
}

export function Layout({ title, preview, children }: LayoutProps) {
  return (
    <Html lang="en" className="email-bg">
      <Head>
        <title>{title}</title>
        <meta name="color-scheme" content="light dark" />
        <meta name="supported-color-schemes" content="light dark" />
        <meta name="viewport" content="width=device-width" />
        <meta name="robots" content="noindex" />
        <style>{emailStyles}</style>
      </Head>
      <Preview>{preview}</Preview>
      <Body
        className="email-bg"
        style={{
          backgroundColor: theme.background,
          fontFamily: theme.fontSans,
          margin: 0,
          padding: 0,
          width: "100%",
        }}
      >
        <Section className="email-shell" style={{ padding: "40px 16px" }}>
          <Container
            className="email-card"
            style={{
              backgroundColor: theme.card,
              borderRadius: 16,
              maxWidth: 600,
              width: "100%",
            }}
          >
            {children}
            <Section
              className="email-px"
              style={{ padding: "32px 48px 40px 48px" }}
            >
              <Hr
                className="email-border"
                style={{
                  borderColor: theme.border,
                  borderTop: `1px solid ${theme.border}`,
                  margin: "0 0 24px 0",
                  width: "100%",
                }}
              />
              <Text
                className="email-muted"
                style={{
                  color: theme.mutedForeground,
                  fontFamily: theme.fontSans,
                  fontSize: 12,
                  lineHeight: "20px",
                  margin: 0,
                }}
              >
                This email relates to your account.
                <br />
                Powered by <strong>Andurel</strong>
              </Text>
            </Section>
          </Container>
        </Section>
      </Body>
    </Html>
  )
}
