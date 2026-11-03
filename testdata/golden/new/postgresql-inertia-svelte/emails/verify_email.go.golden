package emails

import (
	"github.com/mbvlabs/andurel/pkg/postmand"
)

type VerifyEmail struct {
	VerificationCode string
}

var _ postmand.Transformer = (*VerifyEmail)(nil)

func (v VerifyEmail) ToHTML() (string, error) {
	return templates.HTML("verify_email", v)
}

func (v VerifyEmail) ToText() (string, error) {
	return templates.Text("verify_email", v)
}
