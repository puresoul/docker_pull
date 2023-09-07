// Copyright (2012) Sandia Corporation.
// Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation,
// the U.S. Government retains certain rights in this software.

package vmbetter

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
        "go_pull/pkgs/vmconfig"
)

// PostBuildCommands invokes any commands listed in the postbuild variable
// of a config file. It does so by copying the entire string of the postbuild
// variable into a bash script under /tmp of the build directory, and then
// executing it with bash inside of a chroot. Post build commands are executed
// in depth-first order.
func PostBuildCommands(buildPath string, c vmconfig.Config) error {
	if len(c.Postbuilds) == 0 {
		return nil
	}

	// mount /dev and /proc inside the chroot
	proc := filepath.Join(buildPath, "proc")
	if err := exec.Command("mount", "-t", "proc", "none", proc).Run(); err != nil {
		return err
	}

	defer func() {
		if err := exec.Command("umount", proc).Run(); err != nil {
		}
	}()

	dev := filepath.Join(buildPath, "dev")
	if err := exec.Command("mount", "-o", "bind", "/dev", dev).Run(); err != nil {
		return err
	}

	defer func() {
		if err := exec.Command("umount", dev).Run(); err != nil {
		}
	}()

	for _, pb := range c.Postbuilds {

		tmpfile := buildPath + "/tmp/postbuild.bash"

		ioutil.WriteFile(tmpfile, []byte(pb), 0770)

		p := process("chroot")
		cmd := exec.Command(p, buildPath, "/bin/bash", "/tmp/postbuild.bash")

		err := cmd.Run()
		if err != nil {
			return err
		}
		os.Remove(tmpfile)
	}
	return nil
}
