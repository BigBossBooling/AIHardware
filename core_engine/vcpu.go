package core_engine

import (
	"fmt"
	"v-architect/core_engine/devices" // For serialPort argument in Run
	"v-architect/core_engine/hypervisor" // For KVM_EXIT_IO etc.
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
	vcpuFD, err := hypervisor.KvmIoctlCreateVcpu(vmFD, 0) // Assuming VCPU ID 0
	if err != nil {
		return nil, fmt.Errorf("failed to create VCPU: %w", err)
	}

	// Map the KVM run structure
	kvmRunSize, err := hypervisor.KvmIoctlGetVcpuMmapSize()
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
	sregs, err := hypervisor.KvmIoctlGetSregs(vcpu.fd)
	if err != nil {
		return fmt.Errorf("KVM_GET_SREGS failed for initial read: %w", err)
	}

	// Setup for Real Mode execution at 07C0:0000 (linear 0x7C00)
	// CS: Segment a MBR is typically loaded into and executed from.
	// BIOS loads MBR at 0000:7C00. CS=0, IP=0x7C00.
	// Or, set CS=0x07C0, IP=0x0000. Linear address = (CS << 4) + IP.
	// (0x07C0 << 4) + 0x0000 = 0x7C00. This is a common setup.
	sregs.Cs.Base = 0x0000 // For real mode, base is often 0 for segments selectors that are 0,
	                       // or selector * 16 if selector is non-zero.
	                       // KVM might expect base = selector << 4 for real mode.
	sregs.Cs.Selector = 0x07C0
	sregs.Cs.Limit = 0xFFFF
	sregs.Cs.Type = 0x0B    // Code, Execute/Read, Accessed
	sregs.Cs.Present = 1
	sregs.Cs.Dpl = 0
	sregs.Cs.Db = 0         // 16-bit segment
	sregs.Cs.S = 1          // Code or Data segment
	sregs.Cs.L = 0          // Not long mode
	sregs.Cs.G = 0          // Byte granularity

	// Other segments (DS, ES, SS) typically point to 0x0000 base in real mode after MBR load.
	// Selector 0x0000, Base 0x0000.
	dataSeg := hypervisor.KvmSegment{
		Base:     0x0000,
		Selector: 0x0000,
		Limit:    0xFFFF,
		Type:     0x03, // Data, Read/Write, Accessed
		Present:  1,
		Dpl:      0,
		Db:       0,    // 16-bit
		S:        1,
		L:        0,
		G:        0,
	}
	sregs.Ds = dataSeg
	sregs.Es = dataSeg
	sregs.Ss = dataSeg // Stack segment also typically 0000:xxxx

	// Ensure CR0.PE = 0 for real mode (KVM default for new VCPU is real mode)
	// sregs.Cr0 &= ^uint64(1) // Clear PE bit. KVM usually starts VCPU in real mode.
	// We can read CR0 first if we want to be sure, but KVM default should be fine.

	if err := hypervisor.KvmIoctlSetSregs(vcpu.fd, sregs); err != nil {
		return fmt.Errorf("KVM_SET_SREGS failed for boot setup: %w", err)
	}

	// Setup general purpose registers (RIP, RSP, RFLAGS)
	regs, err := hypervisor.KvmIoctlGetRegs(vcpu.fd)
	if err != nil {
		return fmt.Errorf("KVM_GET_REGS failed for initial read: %w", err)
	}

	regs.Rip = 0x0000    // IP part for CS=0x07C0 -> effective address 0x7C00
	regs.Rsp = 0x7C00    // Stack pointer, grows downwards from just below MBR.
	                     // Or 0xFFFE, a common initial real-mode SP.
	                     // For MBR, often set by MBR itself if needed. 0x7C00 is simple.
	regs.Rflags = 0x2    // Initial RFLAGS, typically with IF=0 (interrupts disabled)
	// Other GPRs (RAX, RBX etc.) are often 0 or depend on BIOS values (e.g. DL=drive).
	// For a minimal bootloader, these can be zero.

	if err := hypervisor.KvmIoctlSetRegs(vcpu.fd, regs); err != nil {
		return fmt.Errorf("KVM_SET_REGS failed for boot setup: %w", err)
	}

	fmt.Println("VCPU registers configured for bootloader at 07C0:0000 (linear 0x7C00).")
	return nil
}


// Run starts the VCPU execution loop.
// It passes devices to handle KVM_EXIT_IO and the PIC for interrupt checks.
func (vcpu *VCpu) Run(
	serialPort *devices.SerialPortDevice,
	pit *devices.PITDevice,
	rtc *devices.RTCDevice,
	pic *devices.PICController,
	ata *devices.ATADevice,
	ne2000 *devices.NE2000Device, // Added NE2000 device
) error {
	fmt.Println("VCPU run loop starting...")
	for {
		// Before running, check for pending interrupts from PIC
		// This is a simplified check. KVM has more advanced ways to handle this (e.g. KVM_INTERRUPT ioctl before KVM_RUN)
		// or checking vcpu.kvmRun.RequestInterruptWindow.
		// For now, we check PIC and inject if needed.
		if pic.HasPendingInterrupt() {
			if (vcpu.kvmRun.ReadyForInterruptInjection == 1) || (vcpu.kvmRun.IfFlag == 1) { // Check if guest IF flag is set
				vector, ok := pic.GetInterruptVector()
				if ok {
					// fmt.Printf("VCPU: PIC has pending interrupt. Vector: 0x%02X. Injecting...\n", vector)
					if err := hypervisor.KvmIoctlInterrupt(vcpu.fd, vector); err != nil {
						// Log error but attempt to continue. This might be problematic.
						fmt.Printf("Error injecting KVM interrupt (vector 0x%02X): %v\n", vector, err)
					}
					// After attempting injection, KVM_RUN should be called.
					// The interrupt might not be taken immediately if conditions (like IF=0) prevent it.
				}
			}
		}

		if _, err := hypervisor.KvmIoctlRun(vcpu.fd); err != nil {
			return fmt.Errorf("KVM_RUN failed: %w", err)
		}

		// After KVM_RUN, check again for interrupts that might have been unblocked or newly asserted.
		// This is particularly relevant if KVM_RUN exited for a reason other than interrupt processing.
		if pic.HasPendingInterrupt() {
			if (vcpu.kvmRun.ReadyForInterruptInjection == 1) || (vcpu.kvmRun.IfFlag == 1) {
				vector, ok := pic.GetInterruptVector()
				if ok {
					// fmt.Printf("VCPU (post-run): PIC has pending interrupt. Vector: 0x%02X. Injecting...\n", vector)
					if err := hypervisor.KvmIoctlInterrupt(vcpu.fd, vector); err != nil {
						fmt.Printf("Error injecting KVM interrupt post-run (vector 0x%02X): %v\n", vector, err)
					}
				}
			}
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
			if count != 1 && !(port == devices.PIT_CHANNEL0_DATA || port == devices.PIT_CHANNEL1_DATA || port == devices.PIT_CHANNEL2_DATA || port == devices.PIT_COMMAND_REG || port == devices.RTC_INDEX_PORT || port == devices.RTC_DATA_PORT || (port >= devices.COM1_BASE_ADDR && port < devices.COM1_BASE_ADDR+8) ) {
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
			} else if (port >= devices.PIT_CHANNEL0_DATA && port <= devices.PIT_COMMAND_REG) || port == devices.SYSTEM_CONTROL_PORT_B { // PIT Range + Port 0x61
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
			// Check ATA device ports (Primary channel: 0x1F0-0x1F7 and 0x3F6-0x3F7)
			} else if (port >= devices.ATA_PRIMARY_IO_BASE && port <= devices.ATA_PRIMARY_IO_BASE+7) ||
				(port >= devices.ATA_PRIMARY_CTL_BASE && port <= devices.ATA_PRIMARY_CTL_BASE+1) { // +1 to include 0x3F7 if used. DeviceControl is 0x3F6.
				if ata != nil {
					val, err = ata.HandleIO(port, dataSlice, isWrite)
					if err != nil {
						fmt.Printf("ATA I/O Error on port 0x%x: %v\n", port, err)
					}
				}
			// Check NE2000 device ports (e.g., 0x300-0x31F)
			} else if (port >= devices.NE2000_IO_BASE && port < devices.NE2000_IO_BASE+32) { // NE2000 uses 32 ports
				if ne2000 != nil {
					val, err = ne2000.HandleIO(port, dataSlice, isWrite)
					if err != nil {
						fmt.Printf("NE2000 I/O Error on port 0x%x: %v\n", port, err)
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
