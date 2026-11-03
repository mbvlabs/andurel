import { Section, Text } from "react-email"
import { theme } from "../theme"

type CodeBoxProps = {
  label: string
  code: string
}

export function CodeBox({ label, code }: CodeBoxProps) {
  return (
    <Section
      className="email-muted-bg email-border"
      style={{
        backgroundColor: theme.muted,
        border: `1px solid ${theme.border}`,
        borderRadius: theme.radius,
        margin: "0 0 24px 0",
      }}
    >
      <Section style={{ padding: "24px 24px 8px 24px", textAlign: "center" }}>
        <Text
          className="email-muted"
          style={{
            color: theme.mutedForeground,
            fontFamily: theme.fontSans,
            fontSize: 12,
            fontWeight: 700,
            letterSpacing: "1px",
            lineHeight: "16px",
            margin: 0,
            textTransform: "uppercase",
          }}
        >
          {label}
        </Text>
      </Section>
      <Section style={{ padding: "0 24px 24px 24px", textAlign: "center" }}>
        <Text
          className="email-primary"
          style={{
            color: theme.primary,
            fontFamily: theme.fontMono,
            fontSize: 36,
            fontWeight: 700,
            letterSpacing: "8px",
            lineHeight: "44px",
            margin: 0,
          }}
        >
          {code}
        </Text>
      </Section>
    </Section>
  )
}
