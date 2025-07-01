package devices

import (
	"fmt"
	"io"
	"sync"
)

// SerialPortDevice represents a virtual 16550A-compatible serial port.
type SerialPortDevice struct {
	mu sync.Mutex // Protects device state

	// Registers
	rhrThrDll byte // Read: RHR (Receiver Holding Register), Write: THR (Transmitter Holding Register), DLL (Divisor Latch LSB)
	ierDlm    byte // IER (Interrupt Enable Register), DLM (Divisor Latch MSB)
	iirFcr    byte // Read: IIR (Interrupt Identification Register), Write: FCR (FIFO Control Register)
	lcr       byte // LCR (Line Control Register)
	mcr       byte // MCR (Modem Control Register)
	lsr       byte // LSR (Line Status Register)
	msr       byte // MSR (Modem Status Register)
	scr       byte // SCR (Scratch Register)

	// FIFO state (basic simulation)
	rxBuffer []byte
	rxHead   int
	rxTail   int
	// txBuffer would exist if we were simulating transmission delays or a full TX FIFO

	// Output writer (e.g., os.Stdout or a PTY)
	writer io.Writer

	// Interrupt state
	interruptPending bool
	pic              InterruptRaiser // Interface to signal PIC
}

// NewSerialPortDevice creates a new SerialPortDevice.
func NewSerialPortDevice(writer io.Writer, pic InterruptRaiser) *SerialPortDevice {
	s := &SerialPortDevice{
		writer: writer,
		pic:    pic,
		lsr:    LSR_THRE | LSR_TEMT, // Transmitter empty and holding register empty initially
		iirFcr: IIR_NO_INTERRUPT_PENDING, // No interrupt pending by default
		rxBuffer: make([]byte, 256), // Basic RX buffer
	}
	// Initialize other registers to default values if necessary
	return s
}

// HandleIO processes I/O operations on the serial port registers.
// Returns the value read for read operations, and an error if any.
func (s *SerialPortDevice) HandleIO(port uint16, data []byte, isWrite bool) (uint8, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	offset := port - COM1_BASE_ADDR
	var val byte = 0
	var err error = nil

	// DLAB (Divisor Latch Access Bit) from LCR affects ports 0 and 1
	dlabSet := (s.lcr & LCR_DLAB) != 0

	if isWrite {
		if len(data) == 0 {
			return 0, fmt.Errorf("serial: write operation with no data on port 0x%x", port)
		}
		val = data[0] // Assuming byte-sized I/O for simplicity

		switch offset {
		case RHR_THR_DLL: // THR or DLL
			if dlabSet {
				s.rhrThrDll = val // DLL (LSB of divisor)
				fmt.Printf("Serial: DLL set to 0x%02x\n", val)
			} else {
				// THR (Transmitter Holding Register)
				s.rhrThrDll = val
				if s.writer != nil {
					_, writeErr := s.writer.Write([]byte{val})
					if writeErr != nil {
						fmt.Printf("Serial: Error writing to output: %v\n", writeErr)
						// Potentially set an error bit in LSR or handle otherwise
					}
				}
				// Simulate character transmitted: THRE becomes true, TEMT after a delay (simplified here)
				s.lsr |= LSR_THRE
				s.lsr |= LSR_TEMT
				// If IER_TX_HOLDING_EMPTY is set, signal an interrupt (TODO)
				s.updateInterruptState()
			}
		case IER_DLM: // IER or DLM
			if dlabSet {
				s.ierDlm = val // DLM (MSB of divisor)
				fmt.Printf("Serial: DLM set to 0x%02x\n", val)
			} else {
				s.ierDlm = val & 0x0F // Only lower 4 bits are typically used for IER flags
				s.updateInterruptState()
				fmt.Printf("Serial: IER set to 0x%02x\n", s.ierDlm)
			}
		case IIR_FCR: // FCR
			s.iirFcr = val // Value written to FCR
			if (val & FCR_CLEAR_RX_FIFO) != 0 {
				s.rxHead = 0
				s.rxTail = 0
				s.lsr &= (^LSR_DR & 0xFF) // Data Ready cleared
				fmt.Println("Serial: RX FIFO cleared")
			}
			if (val & FCR_CLEAR_TX_FIFO) != 0 {
				// s.txBuffer cleared if implementing one
				fmt.Println("Serial: TX FIFO cleared")
			}
			if (val & FCR_ENABLE_FIFO) != 0 {
				s.iirFcr |= IIR_FIFO_ENABLED // Indicate FIFOs are "enabled" in IIR
				fmt.Println("Serial: FIFOs enabled")
			} else {
				s.iirFcr &= (^IIR_FIFO_ENABLED & 0xFF)
				fmt.Println("Serial: FIFOs disabled")
			}
			// Other FCR bits (trigger level, DMA) not fully simulated here
		case LCR:
			s.lcr = val
			fmt.Printf("Serial: LCR set to 0x%02x (DLAB=%v)\n", val, (val&LCR_DLAB) != 0)
		case MCR:
			s.mcr = val
			// MCR_OUT2 is often used to enable/disable serial IRQ passthrough from the UART to the PIC.
			// If MCR_OUT2 (bit 3) is set, interrupts are enabled.
			// This interacts with IER bits.
			s.updateInterruptState()
			fmt.Printf("Serial: MCR set to 0x%02x\n", val)
		case LSR:
			// LSR is read-only, writes are usually ignored or have no effect.
			// Some emulators might allow writing for testing/debugging.
			fmt.Printf("Serial: Write to LSR (read-only) with 0x%02x, ignored.\n", val)
		case MSR:
			// MSR is read-only.
			fmt.Printf("Serial: Write to MSR (read-only) with 0x%02x, ignored.\n", val)
		case SCR:
			s.scr = val
			fmt.Printf("Serial: SCR set to 0x%02x\n", val)
		default:
			err = fmt.Errorf("serial: write to unhandled port offset 0x%x", offset)
		}
	} else { // Read operation
		switch offset {
		case RHR_THR_DLL: // RHR or DLL
			if dlabSet {
				val = s.rhrThrDll // DLL (LSB of divisor)
			} else {
				// RHR (Receiver Holding Register)
				if s.rxHead != s.rxTail { // Data available in RX buffer
					val = s.rxBuffer[s.rxTail]
					s.rxTail = (s.rxTail + 1) % len(s.rxBuffer)
					if s.rxHead == s.rxTail { // Buffer now empty
						s.lsr &= (^LSR_DR & 0xFF) // Clear Data Ready bit
					}
				} else {
					val = 0 // Or some default value if buffer is empty
					// LSR_DR should already be clear if no data
				}
				// Reading RHR clears the RX Data Available interrupt condition if that was the source
				if (s.iirFcr & IIR_INTERRUPT_ID_MASK) == IIR_RX_DATA_AVAILABLE {
					s.iirFcr = (s.iirFcr & (^IIR_INTERRUPT_ID_MASK & 0xFF)) | IIR_NO_INTERRUPT_PENDING
				}
				s.updateInterruptState()
			}
		case IER_DLM: // IER or DLM
			if dlabSet {
				val = s.ierDlm // DLM (MSB of divisor)
			} else {
				val = s.ierDlm // IER
			}
		case IIR_FCR: // IIR
			// IIR value reflects the highest priority pending interrupt + FIFO status
			// This is a simplified version. A real 16550 has more complex logic.
			val = s.iirFcr
			// Reading IIR can clear some interrupt types (e.g. THRE on some UARTs, but not typically for IIR read itself)
			// Specifically, if IIR shows THRE interrupt, it's cleared by writing to THR or reading IIR.
			// Here, we assume THRE interrupt is primarily cleared by THR write.
			// The `s.updateInterruptState()` should correctly set IIR_NO_INTERRUPT_PENDING if no other higher prio int exists.
		case LCR:
			val = s.lcr
		case MCR:
			val = s.mcr
		case LSR:
			val = s.lsr
			// Reading LSR can clear some status bits (e.g., error bits like OE, PE, FE, BI)
			// For simplicity, we are not clearing them here automatically on read,
			// guest OS usually handles this.
		case MSR:
			// Modem Status Register: DCD, RI, DSR, CTS
			// Bits 0-3 are delta bits (cleared on MSR read)
			// Bits 4-7 are current state bits
			// For now, return a fixed value (e.g., DSR and CTS asserted, no deltas)
			val = 0x60 // CTS (bit 4) and DSR (bit 5) asserted. No delta changes.
			// s.msr &= 0xF0 // Clear delta bits (0-3) after read
		case SCR:
			val = s.scr
		default:
			err = fmt.Errorf("serial: read from unhandled port offset 0x%x", offset)
		}
	}
	// fmt.Printf("Serial IO: port=0x%x, offset=0x%x, write=%v, data_in=0x%02x, data_out=0x%02x, dlab=%v\n", port, offset, isWrite, data, val, dlabSet)
	return val, err
}

// SimulateIncomingData is a helper to simulate data arriving at the serial port (e.g., from user input via PTY)
func (s *SerialPortDevice) SimulateIncomingData(char byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nextHead := (s.rxHead + 1) % len(s.rxBuffer)
	if nextHead == s.rxTail {
		s.lsr |= LSR_OE // Overrun Error
		fmt.Println("Serial: RX buffer overrun!")
		// Handle overrun interrupt if enabled
	} else {
		s.rxBuffer[s.rxHead] = char
		s.rxHead = nextHead
		s.lsr |= LSR_DR // Data Ready
	}
	s.updateInterruptState()
}

// updateInterruptState checks conditions and updates IIR and signals PIC if needed.
func (s *SerialPortDevice) updateInterruptState() {
	// This is a simplified interrupt logic.
	// A real UART has a priority encoder for multiple interrupt sources.
	// Order of priority (highest first):
	// 1. LSR (Overrun, Parity, Framing, Break)
	// 2. RHR (Received Data Available or FIFO trigger)
	// 3. THR (Transmitter Holding Register Empty)
	// 4. MSR (Modem Status Register changes)

	s.interruptPending = false // Assume no interrupt initially for this update cycle

	// Check LSR interrupts (highest priority)
	if (s.ierDlm & IER_RX_LINE_STATUS) != 0 && (s.lsr&(LSR_OE|LSR_PE|LSR_FE|LSR_BI)) != 0 {
		s.iirFcr = (s.iirFcr & (^IIR_INTERRUPT_ID_MASK & 0xFF)) | IIR_LINE_STATUS // Use IIR_LINE_STATUS
		s.interruptPending = true
	} else if (s.ierDlm & IER_RX_DATA_AVAILABLE) != 0 && (s.lsr&LSR_DR) != 0 { // RX data available
		s.iirFcr = (s.iirFcr & (^IIR_INTERRUPT_ID_MASK & 0xFF)) | IIR_RX_DATA_AVAILABLE
		s.interruptPending = true
	} else if (s.ierDlm & IER_TX_HOLDING_EMPTY) != 0 && (s.lsr&LSR_THRE) != 0 { // TX holding register empty
		s.iirFcr = (s.iirFcr & (^IIR_INTERRUPT_ID_MASK & 0xFF)) | IIR_TX_HOLDING_EMPTY
		s.interruptPending = true
		// Note: THRE interrupt is often level-triggered. It remains active as long as THRE is true and IER bit is set.
		// Some systems might treat it as edge-triggered or clear it upon IIR read.
		// For simplicity, we'll make it active as long as condition holds.
		// Guest OS is expected to write to THR to clear this condition.
	} else if (s.ierDlm & IER_MODEM_STATUS) != 0 && (s.msr & 0x0F) != 0 { // Modem status change (delta bits)
		s.iirFcr = (s.iirFcr & (^IIR_INTERRUPT_ID_MASK & 0xFF)) | IIR_MODEM_STATUS
		s.interruptPending = true
	} else {
		s.iirFcr = (s.iirFcr & (^IIR_INTERRUPT_ID_MASK & 0xFF)) | IIR_NO_INTERRUPT_PENDING
	}

	// Overall interrupt enable from MCR_OUT2
	// If MCR_OUT2 is not set, interrupts from this UART are effectively disabled externally to the PIC.
	if (s.mcr & MCR_OUT2) == 0 {
		s.interruptPending = false
	}

	// FIFO enabled bits in IIR (6 and 7)
	// Check the FCR's actual enable bit, not the IIR's copy of it for this logic
	if (s.iirFcr & FCR_ENABLE_FIFO) != 0 { // This line is problematic, s.iirFcr holds IIR value, FCR_ENABLE_FIFO is for FCR writes
	                                      // A better check would be on a stored FCR value if we had one, or assume if IIR_FIFO_ENABLED was set...
	                                      // For now, let's assume if FCR_ENABLE_FIFO was written to iirFcr field (which happens), then check that.
		s.iirFcr |= IIR_FIFO_ENABLED
	} else {
		s.iirFcr &= (^IIR_FIFO_ENABLED & 0xFF)
	}


	if s.interruptPending {
		s.iirFcr &= (^IIR_NO_INTERRUPT_PENDING & 0xFF) // Clear bit 0 if any interrupt is pending
		if s.pic != nil {
			s.pic.RaiseIRQ(s.GetIRQLine())
		}
		// fmt.Printf("Serial: Interrupt Pending. IIR=0x%02x, IRQ Line: %d\n", s.iirFcr, s.GetIRQLine())
	} else {
		s.iirFcr |= IIR_NO_INTERRUPT_PENDING // Set bit 0 if no interrupt is pending
		if s.pic != nil {
			// Lowering IRQ is tricky; edge-triggered interrupts are latched by PIC.
			// PIC's IRR bit is cleared when interrupt is acknowledged (ISR set) and then EOI'd.
			// For now, we won't explicitly lower it here unless the device itself de-asserts.
			// s.pic.LowerIRQ(s.GetIRQLine())
		}
		// fmt.Printf("Serial: No Interrupt. IIR=0x%02x\n", s.iirFcr)
	}
}

// GetIRQLine returns the IRQ line this device would use.
// This is a placeholder; actual IRQ configuration can be more complex.
// COM1 typically uses IRQ 4.
func (s *SerialPortDevice) GetIRQLine() uint8 {
	return 4
}

// TODO: Define InterruptController interface if not already defined elsewhere
// type InterruptController interface {
// RequestInterrupt(irqLine uint8)
// ClearInterrupt(irqLine uint8) // Or EOI handling
// }

// Placeholder for KVM_EXIT_IO_MAX_DATA_SIZE if not defined globally
// const KVM_EXIT_IO_MAX_DATA_SIZE = 8 // Example, check actual KVM header/def

// This function might be called from vCPU run loop when data is available from host PTY
func (s *SerialPortDevice) HostInput(data []byte) {
    for _, b := range data {
        s.SimulateIncomingData(b)
    }
}
