package files

import (
	"fmt"
)

// FileOperationError represents errors in file operations with context
type FileOperationError struct {
	Operation string
	Path      string
	Err       error
	Output    string // Optional command output
}

// Error performs the error operation.
func (e *FileOperationError) Error() string {
	if e.Output != "" {
		return fmt.Sprintf("file operation '%s' failed for path '%s': %v\nOutput: %s",
			e.Operation, e.Path, e.Err, e.Output)
	}
	return fmt.Sprintf("file operation '%s' failed for path '%s': %v",
		e.Operation, e.Path, e.Err)
}

// Unwrap performs the unwrap operation.
func (e *FileOperationError) Unwrap() error {
	return e.Err
}
