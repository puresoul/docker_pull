// Copyright (2012) Sandia Corporation.
// Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation,
// the U.S. Government retains certain rights in this software.

package nbd

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrNoDeviceAvailable = errors.New("no available nbds found")
)

const (
	// How many times to retry connecting to a nbd device when all are
	// currently in use.
	maxConnectRetries = 3
)

func Modprobe() error {
	// Load the kernel module
	// This will probably fail unless you are root
	p := process("modprobe")
	cmd := exec.Command(p, "nbd", "max_part=10")
	err := cmd.Run()
	if err != nil {
		return err
	}

	// It's possible nbd was already loaded but max_part wasn't set
	return Ready()
}

// Ready checks to see if the NBD kernel module has been loaded. If it does not
// find the module, it returns an error. NBD functions should only be used
// after this function returns no error.
func Ready() error {
	// Ensure that the kernel module has been loaded
	p := process("lsmod")
	cmd := exec.Command(p)
	result, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	if !strings.Contains(string(result), "nbd ") {
		return errors.New("add module 'nbd'")
	}

	// Warn if nbd wasn't loaded with a max_part parameter
	_, err = os.Stat("/sys/module/nbd/parameters/max_part")
	if err != nil {
		log.Println("no max_part parameter set for module nbd")
	}

	return nil
}

// GetDevice returns the first available NBD. If there are no devices
// available, returns ErrNoDeviceAvailable.
func GetDevice(t string) (string, error) {
	// Get a list of all devices
	var d string
	if t == "raw" {
		d = "/dev/mapper/"
	} else {
		d = "/dev"
	}

	devFiles, err := ioutil.ReadDir(d)
	if err != nil {
		return "", err
	}

	nbdPath := ""

	// Find the first available nbd
	for _, devInfo := range devFiles {
		dev := devInfo.Name()
		fmt.Println(devInfo.Name())
		if strings.Contains(dev, "loop") {
			nbdPath = filepath.Join(d, dev)
			break
		}
		// we don't want to include partitions here
		if !strings.Contains(dev, "nbd") || strings.Contains(dev, "p") {
			continue
		}

		// check whether a pid exists for the current nbd
		_, err = os.Stat(filepath.Join("/sys/block", dev, "pid"))
		if err != nil {
			log.Println("found available nbd: " + dev)
			nbdPath = filepath.Join(d, dev)
			break
		} else {
			log.Println("nbd %v could not be used", dev)
		}
	}

	if nbdPath == "" {
		return "", ErrNoDeviceAvailable
	}

	return nbdPath, nil
}

// ConnectImage exports a image using the NBD protocol using the qemu-nbd. If
// successful, returns the NBD device.
func ConnectImage(image string) (string, error) {
	var nbdPath string
	var err error

	for i := 0; i < maxConnectRetries; i++ {
		nbdPath, err = GetDevice("nbd")
		if err != ErrNoDeviceAvailable {
			break
		}

		log.Println("all nbds in use, sleeping before retrying")
		time.Sleep(time.Second * 10)
	}

	if err != nil {
		return "", err
	}

	// connect it to qemu-nbd
	p := process("qemu-nbd")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			"-c",
			nbdPath,
			image,
		},
		Env: nil,
		Dir: "",
	}
	log.Println("connecting to nbd with cmd: %v", cmd)

	err = cmd.Run()
	if err != nil {
		return "", err
	}
	return nbdPath, nil
}

// DisconnectDevice disconnects a given NBD using qemu-nbd.
func DisconnectDevice(dev string) error {
	// disconnect nbd
	p := process("qemu-nbd")
	cmd := &exec.Cmd{
		Path: p,
		Args: []string{
			p,
			"-d",
			dev,
		},
		Env: nil,
		Dir: "",
	}

	log.Println("disconnecting nbd with cmd: %v", cmd)
	return cmd.Run()
}
