//go:build windows

package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	modShell32           = syscall.NewLazyDLL("shell32.dll")
	procSHFileOperationW = modShell32.NewProc("SHFileOperationW")
)

const (
	foDelete          = 3
	fofAllowUndo      = 0x0040 // Send to Recycle Bin instead of permanent delete
	fofNoConfirmation = 0x0010 // Do not display Windows confirmation dialog
	fofSilent         = 0x0004 // Do not display progress dialog
	fofNoErrorUI      = 0x0400 // Do not display error dialog
)

type shFileOpStructW struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

// moveToRecycleBin sends a file to the Windows Recycle Bin safely.
func moveToRecycleBin(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absPath); err != nil {
		return err
	}

	// SHFileOperationW expects pFrom to be double null-terminated
	utf16Chars, err := syscall.UTF16FromString(absPath)
	if err != nil {
		return err
	}
	utf16Chars = append(utf16Chars, 0)

	op := shFileOpStructW{
		wFunc:  foDelete,
		pFrom:  &utf16Chars[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoErrorUI,
	}

	ret, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("SHFileOperationW failed with code %d", ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("recycle bin operation was aborted")
	}
	return nil
}
