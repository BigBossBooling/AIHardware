package devices

// Standard I/O port addresses for a COM1-like 16550A UART.
const (
	COM1_BASE_ADDR uint16 = 0x3F8
	COM2_BASE_ADDR uint16 = 0x2F8
	// Add COM3, COM4 if needed
)

// Register offsets from the base address.
const (
	// When LCR.DLAB = 0
	DATA_REG_OFFSET          uint16 = 0 // RHR (Read Holding Register) / THR (Transmit Holding Register)
	IER_REG_OFFSET           uint16 = 1 // Interrupt Enable Register
	// When LCR.DLAB = 1 (for setting baud rate)
	DLL_REG_OFFSET           uint16 = 0 // Divisor Latch LSB
	DLM_REG_OFFSET           uint16 = 1 // Divisor Latch MSB
)

// Registers accessible regardless of LCR.DLAB
const (
	IIR_REG_OFFSET           uint16 = 2 // Interrupt Identification Register (Read-Only)
	FCR_REG_OFFSET           uint16 = 2 // FIFO Control Register (Write-Only)
	LCR_REG_OFFSET           uint16 = 3 // Line Control Register
	MCR_REG_OFFSET           uint16 = 4 // Modem Control Register
	LSR_REG_OFFSET           uint16 = 5 // Line Status Register (Read-Only)
	MSR_REG_OFFSET           uint16 = 6 // Modem Status Register (Read-Only)
	SCR_REG_OFFSET           uint16 = 7 // Scratch Register
)

// Line Control Register (LCR) bits
const (
	LCR_DLAB uint8 = 1 << 7 // Divisor Latch Access Bit
	// Other bits define word length, stop bits, parity.
)

// Line Status Register (LSR) bits
const (
	LSR_DATA_READY                  uint8 = 1 << 0 // Data Ready (DR) - Set when data is available in RHR
	LSR_OVERRUN_ERROR               uint8 = 1 << 1 // Overrun Error (OE)
	LSR_PARITY_ERROR                uint8 = 1 << 2 // Parity Error (PE)
	LSR_FRAMING_ERROR               uint8 = 1 << 3 // Framing Error (FE)
	LSR_BREAK_INTERRUPT             uint8 = 1 << 4 // Break Interrupt (BI)
	LSR_TRANSMITTER_HOLDING_REG_EMPTY uint8 = 1 << 5 // Transmitter Holding Register Empty (THRE) - THR is empty
	LSR_TRANSMITTER_EMPTY           uint8 = 1 << 6 // Transmitter Empty (TEMT) - THR and shift register are empty
	LSR_FIFO_ERROR                  uint8 = 1 << 7 // Error in RCVR FIFO (FIFOE)
)

// Interrupt Enable Register (IER) bits
const (
	IER_RECEIVED_DATA_AVAILABLE uint8 = 1 << 0
	IER_TRANSMITTER_HOLDING_EMPTY uint8 = 1 << 1
	IER_RECEIVER_LINE_STATUS    uint8 = 1 << 2
	IER_MODEM_STATUS            uint8 = 1 << 3
	// Other bits for sleep mode, low power etc.
)

// Interrupt Identification Register (IIR) bits
const (
	IIR_NO_INTERRUPT_PENDING uint8 = 0x01
	IIR_MODEM_STATUS         uint8 = 0x00
	IIR_TX_HOLDING_REG_EMPTY uint8 = 0x02
	IIR_RX_DATA_AVAILABLE    uint8 = 0x04
	IIR_LINE_STATUS          uint8 = 0x06
	IIR_CHAR_TIMEOUT         uint8 = 0x0C // (16550)
	IIR_FIFO_ENABLED_MASK    uint8 = 0xC0 // Bits 6 and 7 indicate FIFO status
)

// Modem Control Register (MCR) bits
const (
	MCR_DTR   uint8 = 1 << 0 // Data Terminal Ready
	MCR_RTS   uint8 = 1 << 1 // Request To Send
	MCR_OUT1  uint8 = 1 << 2 // Auxiliary Output 1
	MCR_OUT2  uint8 = 1 << 3 // Auxiliary Output 2 (used to enable interrupts)
	MCR_LOOP  uint8 = 1 << 4 // Loopback mode
)

// Number of I/O ports used by a standard serial device
const NUM_SERIAL_REGISTERS = 8
