// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build aix darwin dragonfly freebsd hurd linux netbsd openbsd solaris

package os

import (
	"internal/syscall/unix"
	"io"
	"runtime"
	"syscall"
)

func removeAll(path string) error {
	if path == "" {
		// fail silently to retain compatibility with previous behavior
		// of RemoveAll. See issue 28830.
		return nil
	}

	// The rmdir system call does not permit removing ".",
	// so we don't permit it either.
	if endsWithDot(path) {
		return &PathError{"RemoveAll", path, syscall.EINVAL}
	}

	// Simple case: if Remove works, we're done.
	err := Remove(path)
	if err == nil || IsNotExist(err) {
		return nil
	}

	// RemoveAll recurses by deleting the path base from
	// its parent directory
	parentDir, base := splitPath(path)

	parent, err := Open(parentDir)
	if IsNotExist(err) {
		// If parent does not exist, base cannot exist. Fail silently
		return nil
	}
	if err != nil {
		return err
	}
	defer parent.Close()

	return removeAllFrom(parent, base)
}

func removeAllFrom(parent *File, path string) error {
	parentFd := int(parent.Fd())
	// Simple case: if Unlink (aka remove) works, we're done.
	err := unix.Unlinkat(parentFd, path, 0)
	if err == nil || IsNotExist(err) {
		return nil
	}

	// EISDIR means that we have a directory, and we need to
	// remove its contents.
	// EPERM or EACCES means that we don't have write permission on
	// the parent directory, but this entry might still be a directory
	// whose contents need to be removed.
	// Otherwise just return the error.
	if err != syscall.EISDIR && err != syscall.EPERM && err != syscall.EACCES {
		return err
	}

	// Is this a directory we need to recurse into?
	var statInfo syscall.Stat_t
	statErr := unix.Fstatat(parentFd, path, &statInfo, unix.AT_SYMLINK_NOFOLLOW)
	if statErr != nil {
		if IsNotExist(statErr) {
			return nil
		}
		return statErr
	}
	if statInfo.Mode&syscall.S_IFMT != syscall.S_IFDIR {
		// Not a directory; return the error from the Remove.
		return err
	}

	// Remove the directory's entries.
	var recurseErr error
	for {
		const request = 1024

		// Open the directory to recurse into
		file, err := openFdAt(parentFd, path)
		if err != nil {
			if IsNotExist(err) {
				return nil
			}
			recurseErr = err
			break
		}

		names, readErr := file.Readdirnames(request)
		// Errors other than EOF should stop us from continuing.
		if readErr != nil && readErr != io.EOF {
			file.Close()
			if IsNotExist(readErr) {
				return nil
			}
			return readErr
		}

		for _, name := range names {
			err := removeAllFrom(file, name)
			if err != nil {
				recurseErr = err
			}
		}

		// Removing files from the directory may have caused
		// the OS to reshuffle it. Simply calling Readdirnames
		// again may skip some entries. The only reliable way
		// to avoid this is to close and re-open the
		// directory. See issue 20841.
		file.Close()

		// Finish when the end of the directory is reached
		if len(names) < request {
			break
		}
	}

	// Remove the directory itself.
	unlinkError := unix.Unlinkat(parentFd, path, unix.AT_REMOVEDIR)
	if unlinkError == nil || IsNotExist(unlinkError) {
		return nil
	}

	if recurseErr != nil {
		return recurseErr
	}
	return unlinkError
}

// openFdAt opens path relative to the directory in fd.
// Other than that this should act like openFileNolog.
// This acts like openFileNolog rather than OpenFile because
// we are going to (try to) remove the file.
// The contents of this file are not relevant for test caching.
func openFdAt(dirfd int, name string) (*File, error) {
	var r int
	for {
		var e error
		r, e = unix.Openat(dirfd, name, O_RDONLY, 0)
		if e == nil {
			break
		}

		// See comment in openFileNolog.
		if runtime.GOOS == "darwin" && e == syscall.EINTR {
			continue
		}

		return nil, &PathError{"openat", name, e}
	}

	if !supportsCloseOnExec {
		syscall.CloseOnExec(r)
	}

	return newFile(uintptr(r), name, kindOpenFile), nil
}
