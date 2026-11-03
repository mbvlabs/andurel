package emails

import (
	"github.com/mbvlabs/andurel/pkg/postmand"
)

type ResetPassword struct {
	ResetURL string
}

var _ postmand.Transformer = (*ResetPassword)(nil)

func (r ResetPassword) ToHTML() (string, error) {
	return templates.HTML("reset_password", r)
}

func (r ResetPassword) ToText() (string, error) {
	return templates.Text("reset_password", r)
}
