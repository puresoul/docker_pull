// Copyright (2012) Sandia Corporation.
// Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation,
// the U.S. Government retains certain rights in this software.

package vmbetter

import (
	"fmt"
	"os/exec"
	"strings"
	"flag"
)

var (
    f_debian_mirror = flag.String("mirror", "http://mirrors.ocf.berkeley.edu/debian", "path to the debian mirror to use")
    f_noclean       = flag.Bool("noclean", false, "do not remove build directory")
    f_stage1        = flag.Bool("1", false, "stop after stage one, and copy build files to <config>_stage1")
    f_stage2        = flag.String("2", "", "complete stage 2 from an existing stage 1 directory")
    f_branch        = flag.String("branch", "testing", "debian branch to use")
    f_disk          = flag.Bool("disk", false, "generate a disk image, use -format to set format")
    f_diskSize      = flag.String("size", "1G", "disk image size (e.g. 1G, 1024M)")
    f_format        = flag.String("format", "qcow2", "disk format to use when -disk is set")
    f_mbr           = flag.String("mbr", "/usr/lib/syslinux/mbr/mbr.bin", "path to mbr.bin if building disk images")
    f_iso           = flag.Bool("iso", false, "generate an ISO")
    f_isolinux      = flag.String("isolinux", "misc/isolinux/", "path to a directory containing isolinux.bin, ldlinux.c32, and isolinux.cfg")
    f_rootfs        = flag.Bool("rootfs", false, "generate a simple rootfs")
    f_dstrp_append  = flag.String("debootstrap-append", "", "additional arguments to be passed to debootstrap")
    f_constraints   = flag.String("constraints", "debian,amd64", "specify build constraints, separated by commas")
    f_target        = flag.String("O", "", "specify output name, by default uses name of config")
    f_dry_run       = flag.Bool("dry-run", false, "parse and print configs and then exit")
)

// Debootstrap will invoke the debootstrap tool with a target build directory
// in build_path, using configuration from c.
func Debootstrap(buildPath string) error {
	p := process("debootstrap")

	// build debootstrap parameters
	var args []string
	if *f_dstrp_append != "" {
		args = append(args, strings.Split(*f_dstrp_append, " ")...)
	}
	args = append(args, "--variant=minbase")
	args = append(args, fmt.Sprintf("--include=%v", ""))
	args = append(args, *f_branch)
	args = append(args, buildPath)
	args = append(args, *f_debian_mirror)


	cmd := exec.Command(p, args...)

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
