package constants

import "os"

const (
	// FilePermissionPrivate is used for files containing private data.
	FilePermissionPrivate os.FileMode = 0o600

	// FilePermissionPublic is used for regular readable project files.
	FilePermissionPublic os.FileMode = 0o644

	// DirPermissionDefault is used for regular project directories.
	DirPermissionDefault os.FileMode = 0o755

	// DirPermissionPrivate is used for directories containing private data.
	DirPermissionPrivate os.FileMode = 0o700
)
