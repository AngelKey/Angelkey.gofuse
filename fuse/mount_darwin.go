// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

/*
#include <stdlib.h>
#include <sys/param.h>
#include <sys/mount.h>
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

		var err error
		file, err = os.OpenFile(devPath, syscall.O_RDWR, 0)
		if err == nil {
			break
		}
	}

	cmd := exec.Cmd{
		Path:       mountOsxfusefsPath,
		Args:       []string{"mount_osxfusefs", "-o", "debug", "-o", "iosize=4096", "3", dir},
		Env:        append(os.Environ(), "MOUNT_FUSEFS_CALL_BY_LIB=", "MOUNT_FUSEFS_DAEMON_PATH="+mountOsxfusefsPath),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		ExtraFiles: []*os.File{file},
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %s", mountOsxfusefsPath, err)
	}

	return file, nil
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
