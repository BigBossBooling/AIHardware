package core_engine

import (
	"fmt"
	"os" // Used for os.Stdout conceptually
	// "io" // Would be used for dev.output, dev.input if using file/PTY backend
	// "github.com/creack/pty" // For PTY creation if that backend is chosen
	"sync" // For mutex if register access needs to be thread-safe for complex state

	pb "github.com/V-Architect/v-architect-core/proto" // Import generated protobuf types
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

	// Default I/O Base addresses for common COM ports
	DEFAULT_SERIAL_IO_BASE_COM1 = 0x3F8
	DEFAULT_SERIAL_IO_BASE_COM2 = 0x2F8
	DEFAULT_SERIAL_IO_BASE_COM3 = 0x3E8
	DEFAULT_SERIAL_IO_BASE_COM4 = 0x2E8
)

// SerialPortDevice represents an emulated serial port (e.g., 8250/16550 UART).
type SerialPortDevice struct {
	ID         string
	Config     *pb.SerialPortConfig // From VMConfig
	HostBackend interface{} // e.g., *os.File for stdio/file, or a PTY backend later

	guestIoBaseAddr uint16 // e.g., common COM1 port 0x3F8
	irq             uint32 // e.g., 4 for COM1

	// Registers
	ierReg uint8  // Interrupt Enable Register (offset 1)
	iirReg uint8  // Interrupt Identification Register (offset 2) - Read
	fcrReg uint8  // FIFO Control Register (offset 2) - Write
	lcrReg uint8  // Line Control Register (offset 3)
	mcrReg uint8  // Modem Control Register (offset 4)
	lsrReg uint8  // Line Status Register (offset 5) - Read
	msrReg uint8  // Modem Status Register (offset 6) - Read
	scrReg uint8  // Scratch Register (offset 7)

	dllReg uint8 // Divisor Latch LSB (when LCR.DLAB=1, accessed at offset 0)
	dlmReg uint8 // Divisor Latch MSB (when LCR.DLAB=1, accessed at offset 1)

	mutex sync.Mutex // To protect register access and rxBuffer

	rxBuffer []byte     // Buffer for data received from host (e.g., PTY), to be read by guest
	rxHead   int
	rxTail   int
	TtyPathPlaceholder string // Placeholder for actual TTY path if PTY is used
}

// NewSerialPortDevice creates a new emulated serial port.
func NewSerialPortDevice(id string, config *pb.SerialPortConfig, ioBase uint16, irqNum uint32) (*SerialPortDevice, error) {
	fmt.Printf("Conceptual Serial: NewSerialPortDevice '%s' at I/O Base 0x%X, IRQ %d, Type: %s\n",
		id, ioBase, irqNum, config.GetType())

	dev := &SerialPortDevice{
		ID:              id,
		Config:          config,
		guestIoBaseAddr: ioBase,
		irq:             irqNum,
		lsrReg:          UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE,
		iirReg:          0x01, // No interrupt pending initially
		rxBuffer:        make([]byte, 256),
	}

	switch config.GetType() {
	case pb.SerialPortConfig_STDIO:
		dev.HostBackend = os.Stdout
		fmt.Printf("Conceptual Serial: Port '%s' configured to output to VMM os.Stdout.\n", id)
	case pb.SerialPortConfig_FILE:
		fmt.Printf("Conceptual Serial: Port '%s' configured to output to file '%s'. (File opening not implemented)\n", id, config.GetPathOrAddress())
		// file, err := os.OpenFile(config.GetPathOrAddress(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		// if err != nil { return nil, fmt.Errorf("failed to open file %s for serial port %s: %w", config.GetPathOrAddress(), id, err) }
		// dev.HostBackend = file
	case pb.SerialPortConfig_LOG_ONLY:
		fmt.Printf("Conceptual Serial: Port '%s' configured for LOG_ONLY.\n", id)
	case pb.SerialPortConfig_PTY:
		dev.TtyPathPlaceholder = fmt.Sprintf("/dev/pts/VARCH_%s_conceptual", id)
		fmt.Printf("Conceptual Serial: Port '%s' PTY backend conceptually at %s.\n", id, dev.TtyPathPlaceholder)
	default:
		fmt.Printf("Warning: Serial port '%s' configured with unsupported/unspecified backend type: %s. Defaulting to LOG_ONLY.\n", id, config.GetType())
		// Ensure config type reflects this default if it was unspecified
		if dev.Config.Type == pb.SerialPortConfig_SERIAL_TYPE_UNSPECIFIED {
			dev.Config.Type = pb.SerialPortConfig_LOG_ONLY
		}
	}
	return dev, nil
}

// HandlePIORead handles a Port I/O read from the guest for this serial device.
func (s *SerialPortDevice) HandlePIORead(offset uint16, size int) (uint8, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if size != 1 {
		return 0, fmt.Errorf("serial port %s: read with size %d not supported at offset 0x%X", s.ID, size, offset)
	}

	var val uint8
	isDLABSet := (s.lcrReg & 0x80) != 0

	switch offset {
	case UART_RX:
		if isDLABSet {
			val = s.dllReg
			fmt.Printf("Conceptual Serial %s: Read DLL (DLAB=1) -> 0x%02X\n", s.ID, val)
		} else {
			if s.rxHead != s.rxTail {
				val = s.rxBuffer[s.rxHead]
				s.rxHead = (s.rxHead + 1) % len(s.rxBuffer)
				if s.rxHead == s.rxTail {
					s.lsrReg &= ^UART_LSR_DATA_READY
				}
				fmt.Printf("Conceptual Serial %s: Read RBR <- 0x%02X from rxBuffer\n", s.ID, val)
			} else {
				val = 0
				s.lsrReg &= ^UART_LSR_DATA_READY
				fmt.Printf("Conceptual Serial %s: Read RBR -> 0x00 (rxBuffer empty)\n", s.ID)
			}
		}
	case UART_IER:
		if isDLABSet {
			val = s.dlmReg
			fmt.Printf("Conceptual Serial %s: Read DLM (DLAB=1) -> 0x%02X\n", s.ID, val)
		} else {
			val = s.ierReg
			fmt.Printf("Conceptual Serial %s: Read IER -> 0x%02X\n", s.ID, val)
		}
	case UART_IIR:
		val = s.iirReg
		// Reading IIR can clear THRE interrupt if it was the highest priority pending
		if (s.iirReg & 0x0F) == 0x02 { // THRE interrupt was pending
			// s.iirReg = (s.iirReg & 0xF0) | 0x01 // Set to "no interrupt pending"
		}
		fmt.Printf("Conceptual Serial %s: Read IIR -> 0x%02X\n", s.ID, val)
	case UART_LCR:
		val = s.lcrReg
		fmt.Printf("Conceptual Serial %s: Read LCR -> 0x%02X\n", s.ID, val)
	case UART_MCR:
		val = s.mcrReg
		fmt.Printf("Conceptual Serial %s: Read MCR -> 0x%02X\n", s.ID, val)
	case UART_LSR:
		s.mutex.Lock() // Lock for rxBuffer check
		if s.rxHead != s.rxTail {
			s.lsrReg |= UART_LSR_DATA_READY
		} else {
			s.lsrReg &= ^UART_LSR_DATA_READY
		}
		s.mutex.Unlock() // Unlock after rxBuffer check
		val = s.lsrReg
		fmt.Printf("Conceptual Serial %s: Read LSR -> 0x%02X\n", s.ID, val)
	case UART_MSR:
		val = s.msrReg | 0xB0 // Conceptual: DCD|DSR|CTS set, RI off
		s.msrReg &= 0xF0    // Clear delta bits (0-3) after read
		fmt.Printf("Conceptual Serial %s: Read MSR -> 0x%02X\n", s.ID, val)
	case UART_SCR:
		val = s.scrReg
		fmt.Printf("Conceptual Serial %s: Read SCR -> 0x%02X\n", s.ID, val)
	default:
		return 0, fmt.Errorf("serial port %s: read from unhandled/invalid port offset 0x%X", s.ID, offset)
	}
	return val, nil
}

// HandlePIOWrite handles a Port I/O write from the guest for this serial device.
func (s *SerialPortDevice) HandlePIOWrite(offset uint16, data uint8, size int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if size != 1 {
		return fmt.Errorf("serial port %s: write with size %d not supported at offset 0x%X", s.ID, size, offset)
	}

	isDLABSet := (s.lcrReg & 0x80) != 0

	switch offset {
	case UART_RX: // Write THR (Transmit Holding Register) or DLL (if DLAB set)
		if isDLABSet {
			s.dllReg = data
			fmt.Printf("Conceptual Serial %s: Write DLL (DLAB=1) <- 0x%02X\n", s.ID, data)
		} else {
			s.lsrReg &= ^(UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE)

			switch s.Config.GetType() {
			case pb.SerialPortConfig_STDIO:
				if writer, ok := s.HostBackend.(*os.File); ok { // Check if it's *os.File
					_, _ = writer.Write([]byte{data}) // Error handling omitted for conceptual
				} else { // Fallback if not os.File, e.g. if it's just os.Stdout directly
					fmt.Printf("[VM Serial Out - %s - STDOUT]: %c\n", s.ID, data)
				}
			case pb.SerialPortConfig_FILE:
				// if writer, ok := s.HostBackend.(io.Writer); ok { writer.Write([]byte{data}) } // More generic
				fmt.Printf("[VM Serial Out - %s - FILE %s]: %c\n", s.ID, s.Config.GetPathOrAddress(), data)
			case pb.SerialPortConfig_PTY:
				// if writer, ok := s.HostBackend.(io.Writer); ok { writer.Write([]byte{data}) }
				fmt.Printf("[VM Serial Out - %s - PTY %s]: %c\n", s.ID, s.TtyPathPlaceholder, data)
			case pb.SerialPortConfig_LOG_ONLY, pb.SerialPortConfig_SERIAL_TYPE_UNSPECIFIED, pb.SerialPortConfig_NONE:
				fallthrough
			default:
				fmt.Printf("[VM Serial Out - %s - LOG_ONLY]: Char='%c' (0x%02X)\n", s.ID, data, data)
			}
			s.lsrReg |= (UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE)
			// Conceptual: if IER enables THRE interrupt, trigger IRQ here.
		}
	case UART_IER:
		if isDLABSet {
			s.dlmReg = data
			fmt.Printf("Conceptual Serial %s: Write DLM (DLAB=1) <- 0x%02X\n", s.ID, data)
		} else {
			s.ierReg = data & 0x0F
			fmt.Printf("Conceptual Serial %s: Write IER <- 0x%02X (masked to 0x%02X)\n", s.ID, data, s.ierReg)
		}
	case UART_IIR: // Write FCR (FIFO Control Register)
		s.fcrReg = data
		fmt.Printf("Conceptual Serial %s: Write FCR <- 0x%02X\n", s.ID, data)
		if (data & 0x01) != 0 {
			s.iirReg = (s.iirReg & 0x3F) | 0xC0 // Set FIFO enabled bits (6,7) in IIR
		} else {
			s.iirReg &= ^uint8(0xC0)
		}
		if (data & 0x02) != 0 { /* s.rxBuffer clear */ s.rxHead = 0; s.rxTail = 0; s.lsrReg &= ^UART_LSR_DATA_READY; }
		if (data & 0x04) != 0 { /* TX FIFO clear */ }
	case UART_LCR:
		s.lcrReg = data
		fmt.Printf("Conceptual Serial %s: Write LCR <- 0x%02X\n", s.ID, data)
	case UART_MCR:
		s.mcrReg = data
		fmt.Printf("Conceptual Serial %s: Write MCR <- 0x%02X\n", s.ID, data)
	case UART_LSR:
		fmt.Printf("Conceptual Serial %s: Write LSR <- 0x%02X (ignored)\n", s.ID, data)
	case UART_SCR:
		s.scrReg = data
		fmt.Printf("Conceptual Serial %s: Write SCR <- 0x%02X\n", s.ID, data)
	default:
		return fmt.Errorf("serial port %s: write to unhandled/invalid port offset 0x%X with data 0x%02X", s.ID, offset, data)
	}
	return nil
}

// Close cleans up resources associated with the serial port device.
func (s *SerialPortDevice) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	fmt.Printf("Conceptual Serial: SerialPortDevice %s Close() called.\n", s.ID)
	if fileBackend, ok := s.HostBackend.(*os.File); ok {
		if fileBackend != os.Stdout && fileBackend != os.Stderr { // Don't close std streams
			fmt.Printf("Conceptual Serial: Closing backend file for %s.\n", s.ID)
			// return fileBackend.Close()
		}
	}
	s.HostBackend = nil
	return nil
}

// SimulateHostInput is a helper for tests or external input to feed the rxBuffer.
func (s *SerialPortDevice) SimulateHostInput(data []byte) {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    for _, b := range data {
        nextTail := (s.rxTail + 1) % len(s.rxBuffer)
        if nextTail == s.rxHead { // Buffer full
            fmt.Printf("Serial %s: RX buffer overrun during SimulateHostInput. Byte '%c' dropped.\n", s.ID, b)
            return
        }
        s.rxBuffer[s.rxTail] = b
        s.rxTail = nextTail
    }
    if len(data) > 0 {
        s.lsrReg |= UART_LSR_DATA_READY
        fmt.Printf("Serial %s: Simulated %d bytes of input. LSR_DATA_READY set. rxHead: %d, rxTail: %d\n", s.ID, len(data), s.rxHead, s.rxTail)
        // Conceptual: if IER enables "Received Data Available", trigger IRQ here.
    }
}
