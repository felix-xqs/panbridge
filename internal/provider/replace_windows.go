//go:build windows

package provider

import "os"

// Go uses MoveFileEx with MOVEFILE_REPLACE_EXISTING on Windows. If replacement
// fails (for example because a file is locked), preserve the existing file.
func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
