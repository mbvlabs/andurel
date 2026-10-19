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
	"github.com/mbvlabs/andurel/pkg/storage"
)

// Pass i.queue.InTx(tx) from inside storage.AtomicOps so the email is only
// delivered if the user commits.
func (i Identity) sendWelcome(ctx context.Context, queue storage.InsertQueue, user domains.User) error {
	message := appemail.WelcomeEmail{UserName: user.Email}

	html, err := message.ToHTML()
	if err != nil {
		return err
	}
	text, err := message.ToText()
	if err != nil {
		return err
	}

	_, err = queue.Insert(ctx, jobs.SendTransactionalEmailArgs{
		Data: email.TransactionalData{
			To:       user.Email,
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
- Send transactional mail by enqueuing `jobs.SendTransactionalEmailArgs` on a `storage.InsertQueue` from a service, not directly from a handler. When the email depends on data written in the same use case, pass `i.queue.InTx(tx)` from inside `storage.AtomicOps`.
- Job args persist in `river_job`. An email that carries a secret (verification code, reset link) is not rendered at enqueue time: enqueue a job with the user id and let its worker mint the secret and render the email, as `jobs.SendVerificationEmailArgs` and `jobs.SendPasswordResetEmailArgs` do.
- Do not hand-edit the compiled `*_templ.go`; it is derived.
- Email is the only templ home besides `resources/root.templ`.

## Checklist

- [ ] Template added under `email/` with `ToHTML`/`ToText`.
- [ ] `andurel sync email` run and compiles cleanly.
- [ ] Send helper lives in a service and enqueues the email job.
- [ ] No page-view templ added.
