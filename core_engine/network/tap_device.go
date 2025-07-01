package network

import (
	"fmt"
	"os"
	"strings"
	"syscall" // For basic syscalls like Close, Read, Write on fd
	"unsafe"  // For ioctl argument conversion

	"golang.org/x/sys/unix" // For TUNSETIFF ioctl and related constants
)

const (
	ifReqFlagsOffset = unix.IFNAMSIZ // Offset of ifr_flags in struct ifreq
	ifReqNameOffset  = 0            // Offset of ifr_name in struct ifreq
)

// TapDevice represents a TUN/TAP network interface.
type TapDevice struct {
	file     *os.File
	ifName   string
	fd       int
}

// NewTapDevice creates and configures a new TAP device.
// ifNamePrefix can be something like "tap" and a number will be appended if the name is taken,
// or a specific name like "tap0". If empty, "tap%d" is used.
func NewTapDevice(ifNameDesired string) (*TapDevice, error) {
	if ifNameDesired == "" {
		ifNameDesired = "tap%d" // Kernel will find the next available tapN interface
	}

	// Open the TUN/TAP device file
	file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/net/tun: %w", err)
	}

	// Prepare the ifreq structure for TUNSETIFF ioctl
	var ifr struct {
		Name  [unix.IFNAMSIZ]byte // Typically 16 bytes
		Flags uint16              // 2 bytes
		Pad   [22]byte            // Padding to make struct approx 40 bytes (16+2+22=40), a common size for ifreq
	}

	copy(ifr.Name[:], []byte(ifNameDesired))
	ifr.Flags = unix.IFF_TAP | unix.IFF_NO_PI // TAP device (Ethernet frames), no packet info

	// Call TUNSETIFF ioctl to create/configure the interface
	// int ioctl(int fd, unsigned long request, ...);
	// The third argument is a pointer to struct ifreq.
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		file.Fd(),
		uintptr(unix.TUNSETIFF),
		uintptr(unsafe.Pointer(&ifr)),
	)
	if errno != 0 {
		file.Close()
		return nil, fmt.Errorf("ioctl TUNSETIFF failed for %s: %s", ifNameDesired, errno.Error())
	}

	// The kernel might have assigned a different name if "tap%d" was used or ifNameDesired was taken.
	// The actual interface name is returned in ifr.Name.
	actualIfName := strings.TrimRight(string(ifr.Name[:]), "\x00")
	fd := int(file.Fd())

	// Set the interface to non-blocking mode for ReadPacket
	// This prevents ReadPacket from blocking indefinitely if no packet is available.
	err = unix.SetNonblock(fd, true)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to set TAP interface %s to non-blocking: %w", actualIfName, err)
	}

	fmt.Printf("TAP device %s created successfully. FD: %d\n", actualIfName, fd)
	fmt.Printf("Note: Interface %s may need to be brought UP and assigned an IP manually, e.g.:\n", actualIfName)
	fmt.Printf("  sudo ip link set dev %s up\n", actualIfName)
	fmt.Printf("  sudo ip addr add <some_ip_address/mask> dev %s\n", actualIfName)


	return &TapDevice{
		file:   file,
		ifName: actualIfName,
		fd:     fd,
	}, nil
}

// ReadPacket reads a single Ethernet frame from the TAP device.
// It returns the packet data or an error.
// Returns (nil, nil) if no packet is available and the FD is non-blocking (EAGAIN).
func (tap *TapDevice) ReadPacket(buffer []byte) (int, error) {
	if tap.file == nil {
		return 0, fmt.Errorf("tap device not open or already closed")
	}
	// n, err := tap.file.Read(buffer)
	n, err := unix.Read(tap.fd, buffer) // Use unix.Read for non-blocking behavior consistency
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == syscall.EAGAIN {
			return 0, nil // No packet available right now
		}
		return 0, fmt.Errorf("failed to read from TAP device %s: %w", tap.ifName, err)
	}
	return n, nil
}

// WritePacket writes a single Ethernet frame to the TAP device.
func (tap *TapDevice) WritePacket(packet []byte) (int, error) {
	if tap.file == nil {
		return 0, fmt.Errorf("tap device not open or already closed")
	}
	// n, err := tap.file.Write(packet)
	n, err := unix.Write(tap.fd, packet)
	if err != nil {
		return 0, fmt.Errorf("failed to write to TAP device %s: %w", tap.ifName, err)
	}
	return n, nil
}

// Close closes the TAP device file descriptor.
func (tap *TapDevice) Close() error {
	if tap.file == nil {
		return nil // Already closed or not opened
	}
	fd := tap.file.Fd() // Get FD before closing file, as file.Fd() might error after close
	err := tap.file.Close()
	tap.file = nil
	tap.fd = -1
	if err != nil {
		return fmt.Errorf("failed to close TAP device %s (FD %d): %w", tap.ifName, fd, err)
	}
	// log.Printf("TAP device %s (FD %d) closed.", tap.ifName, fd)
	return nil
}

// IfName returns the actual name of the interface.
func (tap *TapDevice) IfName() string {
	return tap.ifName
}

// Fd returns the file descriptor of the TAP interface.
func (tap *TapDevice) Fd() int {
	return tap.fd
}
