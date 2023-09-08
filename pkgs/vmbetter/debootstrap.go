// Copyright (2012) Sandia Corporation.
// Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation,
// the U.S. Government retains certain rights in this software.

package vmbetter

import (
	"fmt"
	"os/exec"
	"strings"
)


// Debootstrap will invoke the debootstrap tool with a target build directory
// in build_path, using configuration from c.
func Debootstrap(buildPath string) error {
	p := process("debootstrap")

	// build debootstrap parameters
	var args []string
	if CF.F_dstrp_append != "" {
		args = append(args, strings.Split(CF.F_dstrp_append, " ")...)
	}
	args = append(args, "--variant=minbase")
	args = append(args, fmt.Sprintf("--include=%v", ""))
	args = append(args, CF.F_branch)
	args = append(args, buildPath)
	args = append(args, CF.F_debian_mirror)


	cmd := exec.Command(p, args...)

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
