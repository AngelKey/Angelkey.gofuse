// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO: Rewrite using package syscall not cgo

package fuse

/*

// Adapted from Plan 9 from User Space's src/cmd/9pfuse/fuse.c,
// which carries this notice:
//
// The files in this directory are subject to the following license.
//
// The author of this software is Russ Cox.
//
//         Copyright (c) 2006 Russ Cox
//
// Permission to use, copy, modify, and distribute this software for any
// purpose without fee is hereby granted, provided that this entire notice
// is included in all copies of any software which is or includes a copy
// or modification of this software and in all copies of the supporting
// documentation for such software.
//
// THIS SOFTWARE IS BEING PROVIDED "AS IS", WITHOUT ANY EXPRESS OR IMPLIED
// WARRANTY.  IN PARTICULAR, THE AUTHOR MAKES NO REPRESENTATION OR WARRANTY
// OF ANY KIND CONCERNING THE MERCHANTABILITY OF THIS SOFTWARE OR ITS
// FITNESS FOR ANY PARTICULAR PURPOSE.

#include <stdlib.h>
#include <sys/param.h>
#include <sys/mount.h>
#include <unistd.h>
#include <string.h>
#include <stdio.h>
#include <errno.h>
#include <fcntl.h>

#define nil ((void*)0)

static int
mountfuse(int fd, char *mtpt, char **err)
{
	int pid;
	char buf[200];

	pid = fork();
	if(pid < 0)
		return -1;
	if(pid == 0){
		snprintf(buf, sizeof buf, "%d", fd);
		setenv("MOUNT_FUSEFS_CALL_BY_LIB", "", 1);
		// Different versions of MacFUSE put the
		// mount_fusefs binary in different places.
		// Try all.
		// Leopard location
		setenv("MOUNT_FUSEFS_DAEMON_PATH", "/Library/Filesystems/osxfusefs.fs/Support/mount_osxfusefs", 1);
		execl("/Library/Filesystems/osxfusefs.fs/Support/mount_osxfusefs", "mount_osxfusefs", "-o", "iosize=4096", buf, mtpt, nil);
		fprintf(stderr, "exec mount_osxfusefs: %s\n", strerror(errno));
		_exit(1);
	}
	return fd;
}

*/
import "C"
import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

func getvfsbyname(name string, vfc *C.struct_vfsconf) error {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	status, err := C.getvfsbyname(cname, vfc)
	if status != 0 {
		return err
	}
	return nil
}

const (
	vfsName            = "osxfusefs"
	loadOsxfusefsPath  = "/Library/Filesystems/osxfusefs.fs/Support/load_osxfusefs"
	mountOsxfusefsPath = "/Library/Filesystems/osxfusefs.fs/Support/mount_osxfusefs"
)

func mountGo(dir string, options string) (*os.File, error) {
	var vfc C.struct_vfsconf
	if err := getvfsbyname(vfsName, &vfc); err != nil {
		if _, err := os.Stat(loadOsxfusefsPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot find load_osfusefs")
		}

		cmd := exec.Command(loadOsxfusefsPath)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("%s: %s", loadOsxfusefsPath, err)
		}

		if err := getvfsbyname(vfsName, &vfc); err != nil {
			return nil, fmt.Errorf("getvfsbyname(%s): %s", vfsName, err)
		}
	}

	// Look for available FUSE device.
	var file *os.File
	var devPath string
	for i := 0; ; i++ {
		devPath = fmt.Sprintf("/dev/osxfuse%d", i)
		if _, err := os.Stat(devPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("no available fuse devices %d", i)
		}

		var fd int
		var err error
		fd, err = syscall.Open(devPath, syscall.O_RDWR, 0)
		if err == nil {
			file = os.NewFile(uintptr(fd), devPath)
			break
		}
	}

	errp := (**C.char)(C.malloc(16))
	*errp = nil
	defer C.free(unsafe.Pointer(errp))
	cdir := C.CString(dir)
	defer C.free(unsafe.Pointer(cdir))
	C.mountfuse(C.int(file.Fd()), cdir, errp)

	/*
		cmd := exec.Cmd{
			Path:       mountOsxfusefsPath,
			Args:       []string{"mount_osxfusefs", "-o", "debug", "-o", "iosize=4096", "3", dir},
			Env:        []string{"MOUNT_FUSEFS_CALL_BY_LIB=", "MOUNT_FUSEFS_DAEMON_PATH=" + mountOsxfusefsPath},
			Stdout:     os.Stdout,
			Stderr:     os.Stderr,
			ExtraFiles: []*os.File{file},
		}
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("%s: %s", mountOsxfusefsPath, err)
		}*/

	return file, nil
}

/*
func mount(dir string, options string) (int, error) {
	errp := (**C.char)(C.malloc(16))
	*errp = nil
	defer C.free(unsafe.Pointer(errp))
	cdir := C.CString(dir)
	defer C.free(unsafe.Pointer(cdir))
	fd := C.mountfuse(cdir, errp)
	if *errp != nil {
		return -1, mountError(C.GoString(*errp))
	}
	return int(fd), nil
}*/

type mountError string

func (m mountError) Error() string {
	return string(m)
}

func unmount(mountPoint string) error {
	if err := syscall.Unmount(mountPoint, 0); err != nil {
		return fmt.Errorf("umount(%q): %v", mountPoint, err)
	}
	return nil
}

var umountBinary string

func init() {
	var err error
	umountBinary, err = exec.LookPath("umount")
	if err != nil {
		log.Fatalf("Could not find umount binary: %v", err)
	}
}
