package core_engine

import (
	"fmt"
	"os" // Used for os.Stdout conceptually
	// "io" // Would be used for dev.output, dev.input
	// "github.com/creack/pty" // For PTY creation if that backend is chosen
	// "sync" // For mutex if register access needs to be thread-safe for complex state
)

// Standard I/O port offsets for a 8250/16550 UART
const (
	UART_RX  = 0 // Receiver Buffer Register (RBR) on read, Transmitter Holding Register (THR) on write
	UART_IER = 1 // Interrupt Enable Register
	UART_IIR = 2 // Interrupt Identification Register (IIR) on read, FIFO Control Register (FCR) on write
	UART_LCR = 3 // Line Control Register
	UART_MCR = 4 // Modem Control Register
	UART_LSR = 5 // Line Status Register
	UART_MSR = 6 // Modem Status Register
	UART_SCR = 7 // Scratch Register

	// Line Status Register bits
	UART_LSR_DATA_READY = 0x01 // Data available for reading from RBR
	UART_LSR_OE         = 0x02 // Overrun Error
	UART_LSR_PE         = 0x04 // Parity Error
	UART_LSR_FE         = 0x08 // Framing Error
	UART_LSR_BI         = 0x10 // Break Interrupt
	UART_LSR_TX_EMPTY   = 0x20 // Transmitter Holding Register is empty (ready for next char)
	UART_LSR_TX_IDLE    = 0x40 // Transmitter is empty and line is idle

	DEFAULT_SERIAL_IO_BASE_COM1 = 0x3F8
	DEFAULT_SERIAL_IO_BASE_COM2 = 0x2F8
	// ... other standard COM port bases
)

// SerialPortDevice represents an emulated serial port (e.g., 8250/16550 UART).
type SerialPortDevice struct {
	id         string
	ioBaseAddr uint16
	// output     io.Writer // Where the serial output goes (e.g., os.Stdout, a PTY master)
	// input      io.Reader // Where serial input comes from (e.g., a PTY master) - for later input handling

	// Registers (simplified for this conceptual step)
	// For RBR/THR, we don't need to store THR content as it's immediately "sent".
	// RBR would hold a byte if input is implemented and data has arrived.
	ier byte // Interrupt Enable Register
	iir byte // Interrupt Identification Register (primarily to show "no interrupt pending")
	fcr byte // FIFO Control Register (written to same offset as IIR)
	lcr byte // Line Control Register
	mcr byte // Modem Control Register
	lsr byte // Line Status Register
	msr byte // Modem Status Register
	scr byte // Scratch Register

	// PTY specific (if used for backend)
	// ptyFile *os.File
	// TtyPath string // Publicly accessible path to the TTY slave, for DOL to show user

	// lock sync.Mutex // For protecting register state if accessed by multiple goroutines (e.g., I/O thread and control thread)
}

// NewSerialPortDevice creates a new emulated serial port.
// ioBase is the starting I/O port address for this serial device.
// outputToStdOut determines if output is immediately printed to host stdout or if a PTY should be set up.
func NewSerialPortDevice(id string, ioBase uint16, outputToStdOut bool /*, config pb.SerialPortConfig */) (*SerialPortDevice, error) {
	fmt.Printf("Conceptual Serial: NewSerialPortDevice %s at I/O Base 0x%X. OutputToStdout: %t\n", id, ioBase, outputToStdOut)

	dev := &SerialPortDevice{
		id:         id,
		ioBaseAddr: ioBase,
		// Initial state of LSR: Transmitter is empty and idle. No data ready.
		lsr: UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE,
		iir: 0x01, // Bit 0 set to 1 indicates no interrupt pending. Bits 6,7 for FIFO enabled (conceptual default).
		// Other registers default to 0.
	}

	// Conceptual: Based on pb.SerialPortConfig.Type, setup backend.
	// For this sub-issue, we focus on outputting to stdout or a conceptual PTY.
	if outputToStdOut {
		// dev.output = os.Stdout // In a real implementation
		fmt.Printf("Conceptual Serial: Port %s configured to output to os.Stdout.\n", id)
	} else {
		// Conceptual PTY setup:
		// ptmx, tty, err := pty.Open() // From "github.com/creack/pty"
		// if err != nil { return nil, fmt.Errorf("failed to open PTY for serial %s: %w", id, err) }
		// dev.ptyFile = ptmx
		// dev.output = ptmx // Write to PTY master
		// dev.input = ptmx  // Read from PTY master (for future input)
		// dev.TtyPath = tty.Name()
		// fmt.Printf("Conceptual Serial: Port %s connected to PTY: %s (master fd: %d).\n", id, dev.TtyPath, ptmx.Fd())
		// The DOL would need dev.TtyPath to inform the user or connect a terminal emulator.
		dev.TtyPath_placeholder := fmt.Sprintf("/dev/pts/VARCH_%s", id) // Placeholder for TTY path
		fmt.Printf("Conceptual Serial: Port %s would be connected to a PTY (e.g., %s).\n", id, dev.TtyPath_placeholder)
	}
	return dev, nil
}

var TtyPath_placeholder string // Exported for conceptual access by other parts if needed

// HandlePIORead handles a Port I/O read from the guest for this serial device.
// `port` is the absolute I/O port address.
// `size` is the access size in bytes (1, 2, or 4 typically for PIO).
// Returns the value read as uint64 and an error.
func (s *SerialPortDevice) HandlePIORead(port uint16, size int) (uint64, error) {
	// s.lock.Lock()
	// defer s.lock.Unlock()

	offset := port - s.ioBaseAddr // Calculate offset from the base I/O address
	var val byte = 0

	// fmt.Printf("Conceptual Serial %s: PIO Read from port 0x%X (offset %d), size %d\n", s.id, port, offset, size)

	if size != 1 {
		// Most UART registers are 1 byte. Reads of other sizes might be undefined or return part of data.
		// For simplicity, we'll only handle 1-byte reads correctly.
		fmt.Printf("Warning Serial %s: Read from port 0x%X with unsupported size %d, returning 0.\n", s.id, port, size)
		return 0, nil // Or return an error
	}

	switch offset {
	case UART_RX: // Read RBR (Receive Buffer Register)
		// For now, as input is not implemented, reading RBR always returns 0 (or last char if input buffer existed).
		// If input were implemented and data was available:
		//   val = s.inputBuffer.ReadByte()
		//   s.lsr &= ^UART_LSR_DATA_READY // Clear "Data Ready" bit in LSR
		//   // If IER enables "Received Data Available Interrupt", trigger IRQ.
		val = 0 // No input implemented yet
		s.lsr &= ^UART_LSR_DATA_READY
		fmt.Printf("Conceptual Serial %s: Read RBR -> 0x%02X (No input implemented)\n", s.id, val)
	case UART_IER:
		val = s.ier
		fmt.Printf("Conceptual Serial %s: Read IER -> 0x%02X\n", s.id, val)
	case UART_IIR: // Read IIR (Interrupt Identification Register)
		// This typically shows current interrupt status.
		// 0x01: No interrupt pending.
		// Other values for specific interrupts if IER enables them.
		// Bits 6,7 (0xC0) often indicate FIFO enabled.
		val = s.iir // Return current IIR state (e.g. no interrupt pending)
		fmt.Printf("Conceptual Serial %s: Read IIR -> 0x%02X\n", s.id, val)
		// Reading IIR can clear some interrupt conditions in real hardware.
	case UART_LCR:
		val = s.lcr
		fmt.Printf("Conceptual Serial %s: Read LCR -> 0x%02X\n", s.id, val)
	case UART_MCR:
		val = s.mcr
		fmt.Printf("Conceptual Serial %s: Read MCR -> 0x%02X\n", s.id, val)
	case UART_LSR: // Read LSR (Line Status Register)
		val = s.lsr
		// Reading LSR often clears some error bits in real hardware, but not DATA_READY or TX_EMPTY.
		// For conceptual: After guest reads LSR, assume it might send data, so TX is ready.
		// This is a simplification; real TX readiness depends on actual transmission completion.
		s.lsr |= (UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE)
		fmt.Printf("Conceptual Serial %s: Read LSR -> 0x%02X (TX status reset to empty/idle after read)\n", s.id, val)
	case UART_MSR:
		val = s.msr
		fmt.Printf("Conceptual Serial %s: Read MSR -> 0x%02X\n", s.id, val)
	case UART_SCR:
		val = s.scr
		fmt.Printf("Conceptual Serial %s: Read SCR -> 0x%02X\n", s.id, val)
	default:
		fmt.Printf("Warning Serial %s: Read from unhandled/unknown port offset %d (0x%X)\n", s.id, offset, port)
		// Some OSes probe by reading from base+8, base+9 etc.
		// It's often safe to return 0xFF or 0x00 for unassigned/unknown registers.
		return 0xFF, nil // Or return an error: fmt.Errorf("serial read from unhandled port 0x%X", port)
	}
	return uint64(val), nil
}

// HandlePIOWrite handles a Port I/O write from the guest for this serial device.
// `port` is the absolute I/O port address.
// `data` is the value being written (up to 8 bytes, but UART usually uses 1 byte).
// `size` is the access size in bytes.
func (s *SerialPortDevice) HandlePIOWrite(port uint16, data uint64, size int) error {
	// s.lock.Lock()
	// defer s.lock.Unlock()

	offset := port - s.ioBaseAddr
	val := byte(data) // Assuming 1-byte writes for UART registers for simplicity

	// fmt.Printf("Conceptual Serial %s: PIO Write to port 0x%X (offset %d), data 0x%X, size %d\n", s.id, port, offset, data, size)

	if size != 1 {
		fmt.Printf("Warning Serial %s: Write to port 0x%X with unsupported size %d (data 0x%X), ignoring.\n", s.id, port, size, data)
		return nil // Or return an error
	}

	switch offset {
	case UART_RX: // Write THR (Transmit Holding Register)
		// if s.output != nil {
		//    _, err := s.output.Write([]byte{val})
		//    if err != nil { return fmt.Errorf("serial port %s failed to write byte: %w", s.id, err) }
		// } else {
		//    // Fallback if no output writer is configured (should not happen with proper setup)
		//    fmt.Printf("[Serial %s Output (no writer)]: %c (0x%X)\n", s.id, val, val)
		// }
		// For this conceptual implementation, directly print to V-Architect's stdout.
		// A real implementation would write to the configured backend (PTY, file, socket).
		fmt.Printf("[VM Serial - %s]: %c\n", s.id, val) // Character output

		// After writing to THR, it's momentarily not empty.
		// s.lsr &= ^(UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE)
		// Then, simulate it becoming empty very quickly for the next character.
		// A real implementation would manage this based on actual backend write speed or buffering.
		s.lsr |= (UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE)
		// If IER enables "Transmitter Holding Register Empty Interrupt", trigger IRQ.

	case UART_IER:
		s.ier = val
		fmt.Printf("Conceptual Serial %s: Write IER <- 0x%02X\n", s.id, val)
	case UART_IIR: // Write FCR (FIFO Control Register)
		s.fcr = val
		// Handle FIFO control bits if emulating 16550 features (e.g., enable FIFO, reset FIFOs, set trigger levels).
		// For now, just acknowledge.
		fmt.Printf("Conceptual Serial %s: Write FCR <- 0x%02X (FIFO controls conceptually set)\n", s.id, val)
		if (val & 0x01) != 0 { // FIFO Enable bit
			s.iir |= 0xC0 // Indicate FIFO enabled in IIR reads
		} else {
			s.iir &= ^uint8(0xC0)
		}
	case UART_LCR:
		s.lcr = val
		fmt.Printf("Conceptual Serial %s: Write LCR <- 0x%02X (baud rate divisor access might change if DLAB set)\n", s.id, val)
		// If LCR DLAB (Divisor Latch Access Bit) is set (bit 7), then port offsets 0 (RX/THR) and 1 (IER)
		// become DLL (Divisor Latch LSB) and DLM (Divisor Latch MSB) for baud rate setting.
		// This conceptual model doesn't implement baud rate setting yet.
	case UART_MCR:
		s.mcr = val
		fmt.Printf("Conceptual Serial %s: Write MCR <- 0x%02X (modem controls, loopback, OUT1/2)\n", s.id, val)
		// Check MCR bit 4 for loopback mode if emulating fully.
		// OUT1, OUT2 bits might control interrupt line.
	case UART_LSR:
		// LSR is read-only for the most part; guest writes are usually ignored.
		fmt.Printf("Conceptual Serial %s: Write LSR <- 0x%02X (ignored, LSR is read-only)\n", s.id, val)
	case UART_SCR:
		s.scr = val
		fmt.Printf("Conceptual Serial %s: Write SCR <- 0x%02X (scratch register)\n", s.id, val)
	default:
		fmt.Printf("Warning Serial %s: Write to unhandled/unknown port offset %d (0x%X) with data 0x%X\n", s.id, offset, port, data)
		return fmt.Errorf("serial write to unhandled port 0x%X", port)
	}
	return nil
}

// Close cleans up resources associated with the serial port device (e.g., PTY file).
func (s *SerialPortDevice) Close() error {
	fmt.Printf("Conceptual Serial: SerialPortDevice %s Close() called.\n", s.id)
	// if s.ptyFile != nil {
	//    fmt.Printf("Conceptual Serial: Closing PTY file for %s (Path: %s).\n", s.id, s.TtyPath)
	//    err := s.ptyFile.Close()
	//    s.ptyFile = nil
	//    if err != nil {
	//        return fmt.Errorf("failed to close PTY for serial %s: %w", s.id, err)
	//    }
	// }
	return nil
}
