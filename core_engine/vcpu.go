package core_engine

import (
	"fmt"
	"core_engine/devices" // For serialPort argument in Run
	"core_engine/hypervisor" // For KVM_EXIT_IO etc.
	"syscall" // For ioctl
	"unsafe" // For unsafe.Pointer
)

// VCpu represents a virtual CPU.
type VCpu struct {
	fd         int    // File descriptor for the VCPU
	kvmRun     *hypervisor.KvmRun // Pointer to the KVM run structure
	guestMem   []byte // Reference to guest memory for direct access if needed
	// Add other VCPU-specific fields here, like registers if managed directly
}

// NewVCpu creates and initializes a new VCPU.
// guestMem is passed to allow the VCPU to potentially interact with it directly,
// though most memory access is handled by KVM.
func NewVCpu(vmFD int, guestMem []byte) (*VCpu, error) {
	vcpuFD, err := hypervisor.KVM_CREATE_VCPU(vmFD, 0) // Assuming VCPU ID 0
	if err != nil {
		return nil, fmt.Errorf("failed to create VCPU: %w", err)
	}

	// Map the KVM run structure
	kvmRunSize, err := hypervisor.KVM_GET_VCPU_MMAP_SIZE()
	if err != nil {
		return nil, fmt.Errorf("failed to get KVM_RUN mmap size: %w", err)
	}

	kvmRunMap, err := syscall.Mmap(vcpuFD, 0, int(kvmRunSize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("failed to mmap KVM_RUN: %w", err)
	}
	kvmRun := (*hypervisor.KvmRun)(unsafe.Pointer(&kvmRunMap[0]))

	return &VCpu{
		fd:       vcpuFD,
		kvmRun:   kvmRun,
		guestMem: guestMem,
	}, nil
}

// SetupRegisters configures initial VCPU registers.
// This is a placeholder and needs to be implemented based on boot requirements.
func (vcpu *VCpu) SetupRegisters() error {
	// Example: Fetch current sregs
	sregs, err := hypervisor.KVM_GET_SREGS(vcpu.fd)
	if err != nil {
		return fmt.Errorf("KVM_GET_SREGS failed: %w", err)
	}

	// Modify sregs for protected mode or long mode entry as needed
	// For a minimal boot, CS is often set to a real-mode segment
	sregs.Cs.Base = 0
	sregs.Cs.Selector = 0 // Or a specific real mode segment like 0xF000 for BIOS

	// TODO: Set up other segments (DS, ES, SS), GDT, IDT, CR0 (e.g., to enable PE bit)
	// For now, we rely on KVM defaults or minimal changes.

	if err := hypervisor.KVM_SET_SREGS(vcpu.fd, sregs); err != nil {
		return fmt.Errorf("KVM_SET_SREGS failed: %w", err)
	}

	// Setup general purpose registers (RIP, RSP, RFLAGS)
	regs, err := hypervisor.KVM_GET_REGS(vcpu.fd)
	if err != nil {
		return fmt.Errorf("KVM_GET_REGS failed: %w", err)
	}

	// Example: Set initial instruction pointer (RIP) to where guest code is loaded
	// For a typical PC boot, this might be 0xFFF0 (BIOS reset vector)
	// Or for custom loaded code, the entry point of that code.
	regs.Rip = 0x0 // Placeholder, needs actual entry point
	regs.Rsp = 0x8000 // Example stack pointer, adjust as needed
	regs.Rflags = 0x2 // Initial RFLAGS, typically with IF=0

	if err := hypervisor.KVM_SET_REGS(vcpu.fd, regs); err != nil {
		return fmt.Errorf("KVM_SET_REGS failed: %w", err)
	}

	fmt.Println("VCPU registers configured (minimal).")
	return nil
}


// Run starts the VCPU execution loop.
// It passes devices to handle KVM_EXIT_IO.
func (vcpu *VCpu) Run(serialPort *devices.SerialPortDevice, pit *devices.PITDevice, rtc *devices.RTCDevice) error {
	fmt.Println("VCPU run loop starting...")
	for {
		if _, err := hypervisor.KVM_RUN(vcpu.fd); err != nil {
			return fmt.Errorf("KVM_RUN failed: %w", err)
		}

		switch vcpu.kvmRun.ExitReason {
		case hypervisor.KVM_EXIT_HLT:
			fmt.Println("KVM_EXIT_HLT: Guest halted.")
			return nil // Or handle appropriately
		case hypervisor.KVM_EXIT_IO:
			direction := vcpu.kvmRun.Io.Direction
			size := vcpu.kvmRun.Io.Size
			port := vcpu.kvmRun.Io.Port
			count := vcpu.kvmRun.Io.Count
			isWrite := direction == hypervisor.KVM_EXIT_IO_OUT

			// Corrected dataSlice access
			baseAddr := uintptr(unsafe.Pointer(vcpu.kvmRun))
			dataAddr := baseAddr + uintptr(vcpu.kvmRun.Io.DataOffset)
			dataSlice := (*[hypervisor.KVM_EXIT_IO_MAX_DATA_SIZE]byte)(unsafe.Pointer(dataAddr))[:size]

			// KVM_EXIT_IO for string operations might have count > 1
			// For now, we assume count == 1 for simplicity, as most basic device I/O is not string I/O.
			if count != 1 && !(port == devices.PIT_COUNTER0_PORT || port == devices.PIT_COUNTER1_PORT || port == devices.PIT_COUNTER2_PORT || port == devices.PIT_COMMAND_PORT || port == devices.RTC_INDEX_PORT || port == devices.RTC_DATA_PORT || (port >= devices.COM1_BASE_ADDR && port < devices.COM1_BASE_ADDR+8) ) {
				// Log if count is not 1 for an I/O operation we expect to be singular.
				// This is more of a sanity check for non-string I/O.
				// String I/O (rep insb/outsb) would need a loop here.
				fmt.Printf("KVM_EXIT_IO: port=0x%x, direction=%d, size=%d, count=%d > 1 (string I/O?)\n",
					port, direction, size, count)
			}

			var val uint8
			var err error

			// Dispatch to appropriate device
			if port >= devices.COM1_BASE_ADDR && port < devices.COM1_BASE_ADDR+8 { // Serial Port Range
				if serialPort != nil {
					val, err = serialPort.HandleIO(port, dataSlice, isWrite)
					if err != nil {
						fmt.Printf("Serial Port I/O Error on port 0x%x: %v\n", port, err)
					}
				}
			} else if (port >= devices.PIT_COUNTER0_PORT && port <= devices.PIT_COMMAND_PORT) || port == devices.SYSTEM_CONTROL_PORT_B { // PIT Range + Port 0x61
				if pit != nil {
					val, err = pit.HandleIO(port, dataSlice, isWrite)
					if err != nil {
						fmt.Printf("PIT I/O Error on port 0x%x: %v\n", port, err)
					}
				}
			} else if port == devices.RTC_INDEX_PORT || port == devices.RTC_DATA_PORT { // RTC Range
				if rtc != nil {
					val, err = rtc.HandleIO(port, dataSlice, isWrite)
					if err != nil {
						fmt.Printf("RTC I/O Error on port 0x%x: %v\n", port, err)
					}
				}
			} else {
				fmt.Printf("KVM_EXIT_IO: Unhandled port=0x%x, direction=%d, size=%d, count=%d\n",
					port, direction, size, count)
			}

			// For read operations, copy the returned value back into the dataSlice if successful
			if !isWrite && err == nil && size > 0 {
				// Assuming single byte I/O for simplicity in this part of the example.
				// Multi-byte reads would need 'val' to be a slice or handle 'size' appropriately.
				// For now, all HandleIO methods return uint8, so this fits byte reads.
				if size == 1 {
					dataSlice[0] = val
				} else {
					// If size > 1, the HandleIO should ideally return a slice or fill dataSlice directly.
					// For now, this example only fully supports 1-byte reads back to guest via 'val'.
					// A more robust solution would have HandleIO return []byte or take dataSlice as a pointer.
					// Let's assume for now that devices only return single bytes for read.
					// If a device needs to return multiple bytes, its HandleIO needs adjustment.
					fmt.Printf("KVM_EXIT_IO: Read from port 0x%x with size %d > 1, but only first byte written from device.\n", port, size)
					dataSlice[0] = val // Default: put the single byte result in the first byte.
				}
			}
		case hypervisor.KVM_EXIT_FAIL_ENTRY:
			reason := uint64(vcpu.kvmRun.FailEntry.HardwareEntryFailureReason)
			fmt.Printf("KVM_EXIT_FAIL_ENTRY: Reason 0x%x\n", reason)
			return fmt.Errorf("KVM_EXIT_FAIL_ENTRY: Reason 0x%x", reason)
		case hypervisor.KVM_EXIT_INTERNAL_ERROR:
			fmt.Printf("KVM_EXIT_INTERNAL_ERROR: Suberror 0x%x\n", vcpu.kvmRun.Internal.Suberror)
			return fmt.Errorf("KVM_EXIT_INTERNAL_ERROR: Suberror 0x%x", vcpu.kvmRun.Internal.Suberror)
		default:
			fmt.Printf("KVM_EXIT: Unknown reason %d\n", vcpu.kvmRun.ExitReason)
			// Potentially stop VM or log error
		}
	}
}

// Stop performs any necessary cleanup for the VCPU.
func (vcpu *VCpu) Stop() {
	if vcpu.fd != 0 {
		syscall.Close(vcpu.fd)
	}
	// Unmap KVM_RUN structure if mapped (kvmRunMap)
	// Note: kvmRunMap itself is not stored in VCpu struct in this example,
	// so unmapping logic would need access to it if done here.
	// Typically, the mmap is cleaned up when the VCPU fd is closed or process exits.
	fmt.Println("VCPU stopped.")
}
