package jobs

import "uuid"

// SendPasswordResetEmailArgs carries only the user id. The worker mints the
// reset token and renders the email, so no secret is persisted in
// river_job.args.
type SendPasswordResetEmailArgs struct {
	UserID uuid.UUID
}

func (SendPasswordResetEmailArgs) Kind() string { return "send_password_reset_email" }
