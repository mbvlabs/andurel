package constants

import "os"

const (
	FilePermissionPrivate os.FileMode = 0o600

	FilePermissionPublic os.FileMode = 0o644

	DirPermissionDefault os.FileMode = 0o755

	DirPermissionPrivate os.FileMode = 0o700
)
