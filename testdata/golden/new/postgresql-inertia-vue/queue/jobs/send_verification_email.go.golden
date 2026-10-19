package jobs

import "uuid"

// SendVerificationEmailArgs carries only the user id. The worker mints the
// code and renders the email, so no secret is persisted in river_job.args.
type SendVerificationEmailArgs struct {
	UserID uuid.UUID
}

func (SendVerificationEmailArgs) Kind() string { return "send_verification_email" }
