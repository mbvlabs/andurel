# Email

Email is authored as a templ template under `email/` and compiled by
`emailcompiler` into an inlined Go renderer. `andurel sync email` runs the
compiler after templates or Tailwind inputs change. Email keeps templ; page
views do not.

## Files

### email/<name>.templ
```go
package email

import (
	"bytes"
	"context"
)

// WelcomeEmail is a transactional email. The struct carries the data; the
// render method is the templ body; ToHTML/ToText render it for mail clients.
type WelcomeEmail struct {
	UserName string
}

var _ Transformer = (*WelcomeEmail)(nil)

func (w WelcomeEmail) ToHTML() (string, error) {
	var buf bytes.Buffer
	if err := w.render().Render(context.Background(), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (w WelcomeEmail) ToText() (string, error) {
	html, err := w.ToHTML()
	if err != nil {
		return "", err
	}
	return HTMLToText(html)
}

templ (w WelcomeEmail) render() {
	@baseLayout("Welcome", "Thanks for joining us.") {
		<h1 class="m-0 text-[30px] leading-[38px] font-bold text-foreground">Welcome, { w.UserName }!</h1>
	}
}
```

### services/<name>.go
```go
package services

import (
	"context"

	"app/domains"
	appemail "app/email"
	"app/queue/jobs"

	"github.com/mbvlabs/andurel/pkg/email"
)

func (i Identity) sendWelcome(ctx context.Context, user domains.User) error {
	message := appemail.WelcomeEmail{UserName: user.Email.String()}

	html, err := message.ToHTML()
	if err != nil {
		return err
	}
	text, err := message.ToText()
	if err != nil {
		return err
	}

	_, err = i.insertOnly.Insert(ctx, jobs.SendTransactionalEmailArgs{
		Data: email.TransactionalData{
			To:       user.Email.String(),
			From:     i.defaultSenderSignature,
			Subject:  "Welcome",
			HTMLBody: html,
			TextBody: text,
		},
	}, nil)
	return err
}
```

## Rules

- Templates live in `email/`; run `andurel sync email` to compile them.
- An email is a struct plus a `render()` templ method; render it with `ToHTML`/`ToText`.
- Send transactional mail by enqueuing `jobs.SendTransactionalEmailArgs` from a service, not directly from a handler.
- Do not hand-edit the compiled `*_templ.go`; it is derived.
- Email is the only templ home besides `resources/root.templ`.

## Checklist

- [ ] Template added under `email/` with `ToHTML`/`ToText`.
- [ ] `andurel sync email` run and compiles cleanly.
- [ ] Send helper lives in a service and enqueues the email job.
- [ ] No page-view templ added.
