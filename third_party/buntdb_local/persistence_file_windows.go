//go:build windows

package buntdb

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var setRenameInformation = syscall.NewLazyDLL("kernel32.dll").NewProc("SetFileInformationByHandle")

// All persistent handles are read/write files, opened here only by Open and
// Shrink. Share-delete lets Rename replace their names while their handles
// remain valid; it does not bypass external handles denying replacement.
func openPersistenceFile(path string, flags int, perm os.FileMode) (*os.File, error) {
	if flags&os.O_RDWR == 0 {
		return nil, os.ErrInvalid
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(absolute, `\\?\`) {
		if strings.HasPrefix(absolute, `\\`) {
			absolute = `\\?\UNC\` + absolute[2:]
		} else {
			absolute = `\\?\` + absolute
		}
	}
	name, err := syscall.UTF16PtrFromString(absolute)
	if err != nil {
		return nil, err
	}
	creation := uint32(syscall.OPEN_EXISTING)
	if flags&os.O_CREATE != 0 {
		creation = syscall.OPEN_ALWAYS
		if flags&os.O_EXCL != 0 {
			creation = syscall.CREATE_NEW
		}
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, creation, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

func replacePersistenceFile(source, destination string) error {
	// Use a brief DELETE-only handle for rename, not the durable writer. Giving
	// the durable writer DELETE access would prevent ordinary os.Open readers
	// (which do not share DELETE access) from reading the database afterwards.
	src, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(src, `\\?\`) {
		if strings.HasPrefix(src, `\\`) {
			src = `\\?\UNC\` + src[2:]
		} else {
			src = `\\?\` + src
		}
	}
	name, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	handle, err := syscall.CreateFile(name, 0x00010000, // DELETE
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = syscall.CloseHandle(handle)
		}
	}()
	dest, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	utf16, err := syscall.UTF16FromString(dest)
	if err != nil {
		return err
	}
	utf16 = utf16[:len(utf16)-1]
	type renameInfo struct {
		Flags          uint32
		RootDirectory  syscall.Handle
		FileNameLength uint32
		FileName       [1]uint16
	}
	var layout renameInfo
	offset := int(unsafe.Offsetof(layout.FileName))
	buffer := make([]byte, offset+len(utf16)*2)
	info := (*renameInfo)(unsafe.Pointer(&buffer[0]))
	info.Flags = 3 // FILE_RENAME_FLAG_REPLACE_IF_EXISTS | FILE_RENAME_FLAG_POSIX_SEMANTICS
	info.FileNameLength = uint32(len(utf16) * 2)
	for index, char := range utf16 {
		binary.LittleEndian.PutUint16(buffer[offset+index*2:], char)
	}
	result, _, callErr := setRenameInformation.Call(uintptr(handle), 22, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))) // FileRenameInfoEx
	runtime.KeepAlive(buffer)
	if result != 0 {
		return nil
	}
	if callErr == syscall.Errno(87) || callErr == syscall.Errno(50) {
		// Older Windows may not support FileRenameInfoEx. Try the ordinary
		// replacement once, keeping the original writer and destination intact
		// on failure. Never emulate replacement by deleting the destination.
		_ = syscall.CloseHandle(handle)
		closed = true
		return os.Rename(source, destination)
	}
	return &os.LinkError{Op: "replace", Old: source, New: destination, Err: callErr}
}
