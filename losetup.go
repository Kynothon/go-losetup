package losetup

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Add will add a loopback device if it does not exist already.
func (device Device) Add() error {
	ctrl, err := os.OpenFile(LoopControlPath, os.O_RDWR, 0660)
	if err != nil {
		return fmt.Errorf("could not open %v: %v", LoopControlPath, err)
	}
	defer ctrl.Close()
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, ctrl.Fd(), CtlAdd, uintptr(device.Number))
	if errno == unix.EEXIST {
		return fmt.Errorf("device already exist")
	}
	if errno != 0 {
		return fmt.Errorf("could not add device (err: %d): %v", errno, errno)
	}
	return nil
}

// Remove will remove a loopback device if it is not busy.
func (device Device) Remove() error {
	ctrl, err := os.OpenFile(LoopControlPath, os.O_RDWR, 0660)
	if err != nil {
		return fmt.Errorf("could not open %v: %v", LoopControlPath, err)
	}
	defer ctrl.Close()
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, ctrl.Fd(), CtlRemove, uintptr(device.Number))
	if errno == unix.EBUSY {
		return fmt.Errorf("could not remove, device in use")
	}
	if errno != 0 {
		return fmt.Errorf("could not remove (err: %d): %v", errno, errno)
	}
	return nil
}

// GetFree searches for the first free loopback device. If it cannot find one,
// it will attempt to create one. If anything fails, GetFree will return an
// error.
func GetFree() (Device, error) {
	ctrl, err := os.OpenFile(LoopControlPath, os.O_RDWR, 0660)
	if err != nil {
		return Device{}, fmt.Errorf("could not open %v: %v", LoopControlPath, err)
	}
	defer ctrl.Close()
	dev, _, errno := unix.Syscall(unix.SYS_IOCTL, ctrl.Fd(), CtlGetFree, 0)
	if dev < 0 {
		return Device{}, fmt.Errorf("could not get free device (err: %d): %v", errno, errno)
	}
	return Device{Number: uint64(dev), Flags: os.O_RDWR}, nil
}

// Attach attaches backingFile to the loopback device starting at offset. If ro
// is true, then the file is attached read only.
func Attach(backingFile string, offset uint64, ro bool) (Device, error) {
	var dev Device

	dev, err := GetFree()
	if err != nil {
		return dev, err
	}

	return dev.Attach(backingFile, offset, ro)
}

func (device Device) Attach(backingFile string, offset uint64, ro bool) (Device, error) {
	flags := os.O_RDWR
	if ro {
		flags = os.O_RDONLY
	}

	back, err := os.OpenFile(backingFile, flags, 0660)
	if err != nil {
		return device, fmt.Errorf("could not open backing file: %v", err)
	}
	defer back.Close()

	device.Flags = flags

	loopFile, err := device.open()
	if err != nil {
		return device, fmt.Errorf("could not open loop device: %v", err)
	}
	defer loopFile.Close()

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, loopFile.Fd(), SetFd, back.Fd())
	if errno == 0 {
		info := Info{}
		copy(info.FileName[:], []byte(backingFile))
		info.Offset = offset
		if err := setInfo(loopFile.Fd(), info); err != nil {
			unix.Syscall(unix.SYS_IOCTL, loopFile.Fd(), ClrFd, 0)
			return device, fmt.Errorf("could not set info")
		}
		return device, nil
	} else {
		return device, errno
	}
}

// Detach removes the file backing the device.
func (device Device) Detach() error {

	loopFile, err := os.OpenFile(device.Path(), os.O_RDONLY, 0660)
	if err != nil {
		return fmt.Errorf("could not open loop device")
	}
	defer loopFile.Close()

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, loopFile.Fd(), ClrFd, 0)
	if errno != 0 {
		return fmt.Errorf("error clearing loopfile: %v", errno)
	}

	return nil
}
