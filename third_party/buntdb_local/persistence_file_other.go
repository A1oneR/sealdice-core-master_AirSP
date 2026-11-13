//go:build !windows

package buntdb

import "os"

func openPersistenceFile(path string, flags int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flags, perm)
}

func replacePersistenceFile(source, destination string) error {
	return os.Rename(source, destination)
}
