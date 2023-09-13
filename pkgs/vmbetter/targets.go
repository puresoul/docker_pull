// Copyright (2012) Sandia Corporation.
// Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation,
// the U.S. Government retains certain rights in this software.

package vmbetter

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

    "go_pull/pkgs/nbd"
    "go_pull/pkgs/vmconfig"
)

var (
	kernelName string
	initrdName string
	dev string
)

// BuildRootFS generates simple rootfs a from the stage 1 directory.
func BuildRootFS(buildPath string, c vmconfig.Config) error {
	targetName := strings.Split(filepath.Base(c.Path), ".")[0] + "_rootfs"
	if vmconfig.CF.F_target != "" {
		targetName = vmconfig.CF.F_target
	}

	err := os.Mkdir(targetName, 0666)
	if err != nil {
		return err
	}

	p := process("cp")
	cmd := exec.Command(p, "-r", "-v", buildPath+"/.", targetName)
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// BuildISO generates a bootable ISO from the stage 1 directory.
func BuildISO(buildPath string, c vmconfig.Config) error {
	targetName := strings.Split(filepath.Base(c.Path), ".")[0]
	if vmconfig.CF.F_target != "" {
		targetName = vmconfig.CF.F_target
	}

	// Set up a temporary directory
	tdir, err := ioutil.TempDir("", targetName)
	if err != nil {
		return err
	}

	liveDir := tdir + "/image/live/"
	isolinuxDir := tdir + "/image/isolinux/"

	err = os.MkdirAll(liveDir, os.ModeDir|0755)
	if err != nil {
		return err
	}
	err = os.MkdirAll(isolinuxDir, os.ModeDir|0755)
	if err != nil {
		return err
	}

	// Get the kernel path we'll be using
	matches, err := filepath.Glob(buildPath + "/boot/vmlinu*")
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return errors.New("couldn't find kernel")
	}
	kernel := matches[0]

	// Get the initrd path
	matches, err = filepath.Glob(buildPath + "/boot/initrd*")
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return errors.New("couldn't find initrd")
	}
	initrd := matches[0]

	// Copy the kernel and initrd into the appropriate places
	p := process("cp")
	cmd := exec.Command(p, kernel, liveDir+"vmlinuz")
	err = cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command(p, initrd, liveDir+"initrd")
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Copy over the ISOLINUX stuff
	matches, err = filepath.Glob(vmconfig.CF.F_isolinux + "/*")
	if err != nil {
		return err
	}
	for _, m := range matches {
		cmd = exec.Command(p, m, isolinuxDir)
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	// Now compress the chroot
	p = process("mksquashfs")
	cmd = exec.Command(p, buildPath, liveDir+"filesystem.squashfs", "-e", "boot")
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Finally, run genisoimage
	//genisoimage -rational-rock -volid "Minimega" -cache-inodes -joliet -full-iso9660-filenames -b isolinux/isolinux.bin -c isolinux/boot.cat -no-emul-boot -boot-load-size 4 -boot-info-table -output ../minimega.iso .
	p = process("genisoimage")
	cmd = exec.Command(p, "-rational-rock", "-volid", "\"Minimega\"", "-cache-inodes", "-joliet", "-full-iso9660-filenames", "-b", "isolinux/isolinux.bin", "-c", "isolinux/boot.cat", "-no-emul-boot", "-boot-load-size", "4", "-boot-info-table", "-output", targetName+".iso", tdir+"/image")

	err = cmd.Run()
	if err != nil {
		return err
	}

	// clean up
	err = os.RemoveAll(tdir)
	if err != nil {
		return err
	}

	return nil
}

// BuildTargets generates the initrd and kernel files as the last stage of the
// build process. It does so by writing a find/cpio/gzip command as a script
// to a temporary file and executing that in a bash shell. The output filenames
// are equal to the base name of the input config file. So a config called
// 'my_vm.conf' will generate 'my_vm.initrd' and 'my_vm.kernel'. The kernel
// image is the one found in /boot of the build directory.
func BuildTargets(buildPath string, c vmconfig.Config) error {
	targetName := strings.Split(filepath.Base(c.Path), ".")[0]
	if vmconfig.CF.F_target != "" {
		targetName = vmconfig.CF.F_target
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	targetInitrd := fmt.Sprintf("%v/%v.initrd", wd, targetName)
	targetKernel := fmt.Sprintf("%v/%v.kernel", wd, targetName)

	f, err := ioutil.TempFile("", "vmbetter_cpio")
	if err != nil {
		return err
	}

	eName := f.Name()
	initrdCommand := fmt.Sprintf("cd %v && find . -print0 | cpio --quiet --null -ov --format=newc | gzip -9 > %v\ncp boot/vmlinu* %v", buildPath, targetInitrd, targetKernel)
	f.WriteString(initrdCommand)
	f.Close()


	p := process("bash")
	cmd := exec.Command(p, eName)
	// the cpio command outputs regular stuff to stderr, so i have a hack to push all output to the INFO level, instead of INFO/ERROR

	err = cmd.Run()
	if err != nil {
		return err
	}
	os.Remove(eName)

	return nil
}

func ExtractDocker(mount,file string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile("", "docker_tar")
	if err != nil {
		return err
	}

	eName := f.Name()
	initrdCommand := fmt.Sprintf("cd %v && tar xvfO \"%v\" --wildcards --no-anchored \"*.tar\" | tar xivf -", mount, wd+"/"+file)
	f.WriteString(initrdCommand)
	f.Close()

	p := process("bash")
	cmd := exec.Command(p, eName)

	err = cmd.Run()
	if err != nil {
		return err
	}

	os.Remove(eName)
	return nil
}


// BuildDisk creates a disk image using qemu-img, qemu-nbd, sfdisk, mkfs.ext3,
// cp, and extlinux.
func BuildDisk(buildPath string, c vmconfig.Config) (string, error) {
	switch vmconfig.CF.F_format {
	case "qcow", "qcow2", "raw", "vmdk":
	default:
		return "", fmt.Errorf("unknown disk format: %v", vmconfig.CF.F_format)
	}

	targetName := strings.Split(filepath.Base(c.Path), ".")[0]
	if vmconfig.CF.F_target != "" {
		targetName = vmconfig.CF.F_target
	}

	if vmconfig.CF.F_format != "raw" {
		if err := nbd.Modprobe(); err != nil {
			return "", err
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Final disk target
	out := filepath.Join(wd, targetName+"."+vmconfig.CF.F_format)
	// Temporary file, will be renamed to out
	outTmp := out + ".tmp"

	if err := createDisk(outTmp, vmconfig.CF.F_diskSize, vmconfig.CF.F_format); err != nil {
		return "", err
	}

	if vmconfig.CF.F_format != "raw" {
		dev, err = nbd.ConnectImage(outTmp)
		if err != nil {
			return "", err
		}
		dev = dev + "p1"
	} else {
		dev = outTmp
	} 

	if err = partitionDisk(dev); err != nil {
		return "", err
	}

	if vmconfig.CF.F_format == "raw" {
		if err := kpartx(dev, "-a"); err != nil {
			return "", err
		}
		dev, err = nbd.GetDevice("raw")
		if err != nil {
			return "", err
		}
	}

	if err := formatDisk(dev,"ext4"); err != nil {
		return "", err
	}

	mountPath, err := mountDisk(dev)
	if err != nil {
		return "", err
	}

	return mountPath, nil
}

func FinishDisk(mountPath string,c vmconfig.Config) error {
	targetName := strings.Split(filepath.Base(c.Path), ".")[0]
	if vmconfig.CF.F_target != "" {
		targetName = vmconfig.CF.F_target
	}

	if err := nbd.Modprobe(); err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Final disk target
	out := filepath.Join(wd, targetName+"."+vmconfig.CF.F_format)
	// Temporary file, will be renamed to out
	outTmp := out + ".tmp"

	if err := umountDisk(mountPath); err != nil {
		return err
	}
	if err := kpartx(outTmp,"-d"); err != nil {
		return err
	}

	return os.Rename(outTmp, out)
}


// createDisk creates a target disk image using qemu-img. Size specifies the
// size of the image in bytes but optional suffixes such as "K" and "G" can be
// used. See qemu-img(8) for details.
func createDisk(target, size, format string) error {
	// create our disk image
	p := process("qemu-img")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			"create",
			"-f",
			format,
			target,
			size,
		},
		Env: nil,
		Dir: "",
	}



	return cmd.Run()
}

// partitionDisk partitions the provided device creating one primary partition
// that is the size of the whole device and bootable.
func partitionDisk(dev string) error {
	// partition with fdisk
	p := process("sfdisk")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			dev,
		},
		Env: nil,
		Dir: "",
	}
	sIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}
	io.WriteString(sIn, ",500M\n,\n")
	sIn.Close()
	return cmd.Wait()
}

// formatDis formats a partition with the default linux filesystem type.
func formatDisk(dev,t string) error {
	// make an ext3 filesystem
	p := process("mkfs")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			"-t",
			t,
			dev,
		},
		Env: nil,
		Dir: "",
	}


	return cmd.Run()
}


func kpartx(file, arg string) error {
	p := process("kpartx")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			arg,
			file,
		},
		Env: nil,
		Dir: "",
	}

	return cmd.Run()
}


// mountDisk mounts a partition to a temporary directory. If successful,
// returns the path to that temporary directory.
func mountDisk(dev string) (string, error) {
	// mount the filesystem
	mountPath, err := ioutil.TempDir("", "vmbetter_mount_")
	if err != nil {
	}
	p := process("mount")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			dev,
			mountPath,
		},
		Env: nil,
		Dir: "",
	}


	err = cmd.Run()
	if err != nil {
		return "", err
	}
	return mountPath, nil
}

// copyDisk recursively copies files from src to dst using cp.
func copyDisk(src, dst string) error {
	// copy everything over
	p := process("cp")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			"-a",
			"-v",
			src + "/.",
			dst,
		},
		Env: nil,
		Dir: "",
	}



	return cmd.Run()
}

// extlinux installs the SYSLINUX bootloader using extlinux. Path should be the
// root directory for the filesystem. extlinux also writes out a
// minimega-specific configuration file for SYSLINUX.
func extlinux(path string) error {
	// install extlinux
	p := process("extlinux")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			"--install",
			filepath.Join(path, "boot"),
		},
		Env: nil,
		Dir: "",
	}



	err := cmd.Run()
	if err != nil {
		return err
	}

	// write out the bootloader config, but first figure out the kernel and
	// initrd files in /boot
	filepath.Walk(filepath.Join(path, "boot"), kernelWalker)
	if kernelName == "" {
		return fmt.Errorf("could not find kernel name")
	}
	if initrdName == "" {
		return fmt.Errorf("could not find initrd name")
	}

	extlinuxConfig := fmt.Sprintf("DEFAULT minimegalinux\nLABEL minimegalinux\nSAY booting minimegalinux\nLINUX /boot/%v\nAPPEND root=/dev/sda1\nINITRD /boot/%v", kernelName, initrdName)

	return ioutil.WriteFile(filepath.Join(path, "/boot/extlinux.conf"), []byte(extlinuxConfig), os.FileMode(0660))
}

// umountDisk unmounts disk image that was previously mounted with mountDisk.
func umountDisk(path string) error {
	// unmount
	p := process("umount")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			path,
		},
		Env: nil,
		Dir: "",
	}


	return cmd.Run()
}

// extlinuxMBR installs the specified master boot record in the partition table
// for the provided device.
func extlinuxMBR(dev, mbr string) error {
	// dd the mbr image
	p := process("dd")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			fmt.Sprintf("if=%v", mbr),
			"conv=notrunc",
			"bs=440",
			"count=1",
			fmt.Sprintf("of=%v", dev),
		},
		Env: nil,
		Dir: "",
	}



	return cmd.Run()
}

func kernelWalker(path string, info os.FileInfo, err error) error {
	if strings.Contains(info.Name(), "vmlinuz") {
		kernelName = info.Name()
	}
	if strings.Contains(info.Name(), "initrd") {
		initrdName = info.Name()
	}
	return nil
}
