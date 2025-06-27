package devices

import (
	"fmt"   // Added import
	"io"
	"log"
	"os"
	"sync"
	"unsafe" // Added import
	// "v-architect/core_engine/hypervisor" // Removed to break import cycle
)

// Constants for I/O direction, local to devices package or passed as uint8
const (
	IO_IN  uint8 = 0 // Matches hypervisor.KVM_EXIT_IO_IN
	IO_OUT uint8 = 1 // Matches hypervisor.KVM_EXIT_IO_OUT
)

// SerialPortDevice represents a virtual 16550A UART.
type SerialPortDevice struct {
	mu sync.Mutex

	baseAddr uint16
	writer   io.Writer // For characters sent by the guest

	// Registers state
	// Most registers are write-through or have fixed read values for a simple console.
	// We need to store LCR to handle DLAB bit.
	lineControlReg uint8 // LCR
	ier            uint8 // Interrupt Enable Register
	scratchReg     uint8 // SCR

	// TODO: Add input buffer if we want to support host typing into guest
}

// NewSerialPortDevice creates a new virtual serial port.
// writer is where guest output will be sent (e.g., os.Stdout).
func NewSerialPortDevice(baseAddress uint16, writer io.Writer) *SerialPortDevice {
	if writer == nil {
		writer = os.Stdout // Default to stdout
	}
	spd := &SerialPortDevice{
		baseAddr: baseAddress,
		writer:   writer,
		// Default LCR: 8 bits, no parity, 1 stop bit (0x03)
		// DLAB is 0 initially.
		lineControlReg: 0x03,
		// Default IER: typically 0 (all interrupts disabled)
		ier: 0x00,
		// Default SCR: 0xFF is a common test value written by BIOS/bootloaders
		scratchReg: 0x00,
	}
	log.Printf("SerialPortDevice created for base_addr=0x%X", baseAddress)
	return spd
}

// The HandleIO method will be added in the next step.

// Helper to get register offset from a given port address
func (s *SerialPortDevice) getRegisterOffset(port uint16) (uint16, bool) {
	if port >= s.baseAddr && port < s.baseAddr+NUM_SERIAL_REGISTERS {
		return port - s.baseAddr, true
	}
	return 0, false
}

// isDLABSet checks if the Divisor Latch Access Bit is set in LCR.
func (s *SerialPortDevice) isDLABSet() bool {
	return (s.lineControlReg & LCR_DLAB) != 0
}

// BaseAddr returns the base I/O address of the serial port.
func (s *SerialPortDevice) BaseAddr() uint16 {
	// s.mu.Lock() // Not strictly needed for reading a fixed value if baseAddr is immutable after creation
	// defer s.mu.Unlock()
	return s.baseAddr
}

// HandleIO processes an I/O operation directed at this serial port.
// port is the absolute I/O port address.
// kvmRunDataPtr is a pointer to the start of the VCPU's kvm_run data area.
// direction is KVM_EXIT_IO_IN or KVM_EXIT_IO_OUT (these need to be defined based on KVM convention).
// size is the size of the I/O operation (1, 2, or 4 bytes).
// dataOffset is the offset within kvm_run where data should be read from/written to.
func (s *SerialPortDevice) HandleIO(port uint16, kvmRunDataPtr unsafe.Pointer, direction uint8, size uint8, dataOffset uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	regOffset, ok := s.getRegisterOffset(port)
	if !ok {
		// Should not happen if called correctly from vcpu runLoop
		log.Printf("SerialDevice.HandleIO: Port 0x%X out of range for base 0x%X", port, s.baseAddr)
		return fmt.Errorf("port 0x%X out of range for serial device at base 0x%X", port, s.baseAddr)
	}

	// Pointer to the data area within kvm_run specific to this I/O operation
	ioDataPayloadPtr := unsafe.Pointer(uintptr(kvmRunDataPtr) + uintptr(dataOffset))

	// Most serial operations are byte-sized. Some drivers might try word/dword.
	if size != 1 && size != 2 && size != 4 {
		log.Printf("SerialDevice.HandleIO: Unsupported I/O size %d for port 0x%X", size, port)
		// What to do? For now, let's ignore non-byte ops for simplicity, though real hardware might take LSB.
		// Or return an error / signal guest error? For now, just log and ignore.
		return nil
	}


	dlabSet := s.isDLABSet()

	if direction == IO_OUT { // Guest is writing to the serial port (PIO OUT)
		// Read the value written by the guest from the kvm_run data area
		var value uint8
		switch size {
		case 1:
			value = *(*uint8)(ioDataPayloadPtr)
		case 2: // Take LSB for word writes to byte registers
			value = uint8(*(*uint16)(ioDataPayloadPtr) & 0xFF)
			log.Printf("SerialDevice.HandleIO: Port 0x%X (offset %d) received WORD write 0x%04X, using LSB 0x%02X", port, regOffset, *(*uint16)(ioDataPayloadPtr), value)
		case 4: // Take LSB for dword writes
			value = uint8(*(*uint32)(ioDataPayloadPtr) & 0xFF)
			log.Printf("SerialDevice.HandleIO: Port 0x%X (offset %d) received DWORD write 0x%08X, using LSB 0x%02X", port, regOffset, *(*uint32)(ioDataPayloadPtr), value)
		}
		// log.Printf("Serial OUT: Port 0x%X (offset %d), Value 0x%02X, Size %d, DLAB=%t", port, regOffset, value, size, dlabSet)


		switch regOffset {
		case DATA_REG_OFFSET: // THR (Transmit Holding Register) or DLL (Divisor Latch LSB)
			if dlabSet { // DLL
				log.Printf("Serial LCR.DLAB=1: Write to DLL (Divisor Latch LSB) via port 0x%X: 0x%02X (Baud rate setting - ignored for now)", port, value)
				// Store DLL if we were emulating baud rate fully
			} else { // THR
				// Guest is writing a character to be transmitted.
				if s.writer != nil {
					_, err := s.writer.Write([]byte{value})
					if err != nil {
						log.Printf("SerialDevice: Error writing to output writer: %v", err)
						// Non-fatal for the VM, but indicates an issue with host side.
					}
				} else {
					// This case should ideally not happen if NewSerialPortDevice defaults to os.Stdout
					log.Printf("SerialDevice: Output writer is nil. Char '0x%02X' (%c) dropped.", value, value)
				}
			}
		case IER_REG_OFFSET: // IER (Interrupt Enable Register) or DLM (Divisor Latch MSB)
			if dlabSet { // DLM
				log.Printf("Serial LCR.DLAB=1: Write to DLM (Divisor Latch MSB) via port 0x%X: 0x%02X (Baud rate setting - ignored for now)", port, value)
				// Store DLM
			} else { // IER
				log.Printf("Serial IER write to port 0x%X: 0x%02X (Interrupt enabling - ignored for now)", port, value)
				s.ier = value
			}
		case FCR_REG_OFFSET: // FCR (FIFO Control Register) - Write Only
			log.Printf("Serial FCR write to port 0x%X: 0x%02X (FIFO control - ignored for now)", port, value)
			// Handle FIFO settings if emulating them
		case LCR_REG_OFFSET: // LCR (Line Control Register)
			log.Printf("Serial LCR write to port 0x%X: 0x%02X", port, value)
			s.lineControlReg = value
		case MCR_REG_OFFSET: // MCR (Modem Control Register)
			log.Printf("Serial MCR write to port 0x%X: 0x%02X (Modem control - ignored for now)", port, value)
			// Handle MCR bits like DTR, RTS, OUT1, OUT2, LOOP if needed
		case SCR_REG_OFFSET: // SCR (Scratch Register)
			log.Printf("Serial SCR write to port 0x%X: 0x%02X", port, value)
			s.scratchReg = value
		default:
			log.Printf("SerialDevice.HandleIO: Write to unhandled/read-only register offset %d (port 0x%X) with value 0x%02X", regOffset, port, value)
		}

	} else { // Guest is reading from the serial port (PIO IN)
		var value uint8
		switch regOffset {
		case DATA_REG_OFFSET: // RHR (Read Holding Register) or DLL
			if dlabSet { // DLL
				value = 0x00 // Pretend baud rate LSB is 0 (or last written value if stored)
				log.Printf("Serial LCR.DLAB=1: Read from DLL (Divisor Latch LSB) via port 0x%X -> 0x%02X", port, value)
			} else { // RHR
				// TODO: Implement actual input buffering if we want host to type into guest.
				// For now, reading from RHR returns nothing (or a dummy value like 0 or last char if loopback).
				// Some OSes might expect continuous reads to not block or error if nothing is there.
				// Let's return 0 for now, which might be interpreted as null char or no data.
				value = 0x00 // No data ready / dummy value
				log.Printf("Serial RHR read from port 0x%X (no input implemented) -> 0x%02X", port, value)
			}
		case IER_REG_OFFSET: // IER or DLM
			if dlabSet { // DLM
				value = 0x00 // Pretend baud rate MSB is 0
				log.Printf("Serial LCR.DLAB=1: Read from DLM (Divisor Latch MSB) via port 0x%X -> 0x%02X", port, value)
			} else { // IER
				value = s.ier
				log.Printf("Serial IER read from port 0x%X -> 0x%02X", port, value)
			}
		case IIR_REG_OFFSET: // IIR (Interrupt Identification Register) - Read Only
			// Indicate no interrupt pending for basic console.
			// Bit 0 is 0 if interrupt pending, 1 if not.
			// Bits 1-3 identify interrupt.
			// Common value for "no interrupt pending": 0x01
			value = IIR_NO_INTERRUPT_PENDING
			log.Printf("Serial IIR read from port 0x%X -> 0x%02X (No interrupt pending)", port, value)
		case LCR_REG_OFFSET: // LCR
			value = s.lineControlReg
			log.Printf("Serial LCR read from port 0x%X -> 0x%02X", port, value)
		case MCR_REG_OFFSET: // MCR
			value = 0x00 // Or last written value if stored and relevant
			log.Printf("Serial MCR read from port 0x%X -> 0x%02X", port, value)
		case LSR_REG_OFFSET: // LSR (Line Status Register) - Read Only
			// Crucial for polling. Guest checks this to see if it can write (THR empty) or read (DR data ready).
			// For a simple stdout console, THR is always empty immediately.
			// Data Ready (DR) is 0 unless we implement input.
			value = LSR_TRANSMITTER_HOLDING_REG_EMPTY | LSR_TRANSMITTER_EMPTY // Always ready to transmit
			// if input_available { value |= LSR_DATA_READY }
			log.Printf("Serial LSR read from port 0x%X -> 0x%02X (TX Empty, No Data Ready)", port, value)
		case MSR_REG_OFFSET: // MSR (Modem Status Register) - Read Only
			// Default values indicating no modem status changes. DCD, RI, DSR, CTS.
			// Often 0x20 (DSR) or 0xB0 (DCD, DSR, CTS high, RI low) are typical idle states.
			// Let's return a common value like 0x60 (CTS and DSR set).
			value = 0x60 // Example: CTS and DSR active.
			log.Printf("Serial MSR read from port 0x%X -> 0x%02X", port, value)
		case SCR_REG_OFFSET: // SCR (Scratch Register)
			value = s.scratchReg
			log.Printf("Serial SCR read from port 0x%X -> 0x%02X", port, value)
		default:
			value = 0xFF // Common return for reads from unimplemented/write-only ports
			log.Printf("SerialDevice.HandleIO: Read from unhandled register offset %d (port 0x%X) -> 0x%02X", regOffset, port, value)
		}

		// Write the value back into the kvm_run data area for the guest to read
		switch size {
		case 1:
			*(*uint8)(ioDataPayloadPtr) = value
		case 2:
			*(*uint16)(ioDataPayloadPtr) = uint16(value) // Guest reads LSB for byte registers
			log.Printf("SerialDevice.HandleIO: Port 0x%X (offset %d) providing WORD read 0x%04X from byte value 0x%02X", port, regOffset, uint16(value), value)
		case 4:
			*(*uint32)(ioDataPayloadPtr) = uint32(value) // Guest reads LSB
			log.Printf("SerialDevice.HandleIO: Port 0x%X (offset %d) providing DWORD read 0x%08X from byte value 0x%02X", port, regOffset, uint32(value), value)
		}
		// log.Printf("Serial IN: Port 0x%X (offset %d), Value 0x%02X, Size %d, DLAB=%t", port, regOffset, value, size, dlabSet)
	}
	return nil
}
