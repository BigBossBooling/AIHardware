package devices

// Base I/O port for COM1
const COM1_BASE_ADDR uint16 = 0x3F8

// Register offsets from the base address
const (
	RHR_THR_DLL = 0 // Receiver Holding Register (read) / Transmitter Holding Register (write) / Divisor Latch LSB
	IER_DLM     = 1 // Interrupt Enable Register / Divisor Latch MSB
	IIR_FCR     = 2 // Interrupt Identification Register (read) / FIFO Control Register (write)
	LCR         = 3 // Line Control Register
	MCR         = 4 // Modem Control Register
	LSR         = 5 // Line Status Register
	MSR         = 6 // Modem Status Register
	SCR         = 7 // Scratch Register
)

// Line Control Register (LCR) bits
// LCR_DLAB is defined in pic_constants.go
// const (
// 	LCR_DLAB = 1 << 7 // Divisor Latch Access Bit
// 	// Add other LCR bits as needed (e.g., word length, stop bits, parity)
// )

// Line Status Register (LSR) bits
const (
	LSR_DR   = 1 << 0 // Data Ready
	LSR_OE   = 1 << 1 // Overrun Error
	LSR_PE   = 1 << 2 // Parity Error
	LSR_FE   = 1 << 3 // Framing Error
	LSR_BI   = 1 << 4 // Break Interrupt
	LSR_THRE = 1 << 5 // Transmitter Holding Register Empty
	LSR_TEMT = 1 << 6 // Transmitter Empty
	// LSR_ERFIFO = 1 << 7 // Error in RCVR FIFO (16750)
)

// Interrupt Enable Register (IER) bits
const (
	IER_RX_DATA_AVAILABLE = 1 << 0 // Enable Received Data Available Interrupt
	IER_TX_HOLDING_EMPTY  = 1 << 1 // Enable Transmitter Holding Register Empty Interrupt
	IER_RX_LINE_STATUS    = 1 << 2 // Enable Receiver Line Status Interrupt
	IER_MODEM_STATUS      = 1 << 3 // Enable Modem Status Interrupt
	// Add other IER bits if supporting sleep mode, low power mode etc.
)

// Interrupt Identification Register (IIR) bits
// These are defined in pic_constants.go
// const (
// 	IIR_NO_INTERRUPT_PENDING = 0x01
// 	IIR_MODEM_STATUS         = 0x00
// 	IIR_TX_HOLDING_EMPTY     = 0x02
// 	IIR_RX_DATA_AVAILABLE    = 0x04
// 	IIR_RX_LINE_STATUS       = 0x06
// 	IIR_CHAR_TIMEOUT         = 0x0C // (16550) Character timeout indication
// 	// Bits 6 & 7 indicate FIFO enabled status on 16550+
// 	IIR_FIFO_ENABLED      = 0xC0 // Both bits set if FIFOs are enabled
// 	IIR_FIFO_STATUS_MASK  = 0xC0
// 	IIR_INTERRUPT_ID_MASK = 0x0F // Lower 4 bits give the interrupt type
// )

// FIFO Control Register (FCR) bits (for 16550 UARTs)
const (
	FCR_ENABLE_FIFO    = 1 << 0 // Enable FIFOs
	FCR_CLEAR_RX_FIFO  = 1 << 1 // Clear Receiver FIFO
	FCR_CLEAR_TX_FIFO  = 1 << 2 // Clear Transmitter FIFO
	FCR_DMA_MODE_SELECT= 1 << 3 // DMA Mode Select
	// Bits 6 & 7 control RX FIFO trigger level
	FCR_TRIGGER_LEVEL_1  = 0 << 6
	FCR_TRIGGER_LEVEL_4  = 1 << 6
	FCR_TRIGGER_LEVEL_8  = 2 << 6
	FCR_TRIGGER_LEVEL_14 = 3 << 6
)

// Modem Control Register (MCR) bits
// These are defined in pic_constants.go
// const (
// 	MCR_DTR    = 1 << 0 // Data Terminal Ready
// 	MCR_RTS    = 1 << 1 // Request To Send
// 	MCR_OUT1   = 1 << 2 // Auxiliary Output 1
// 	MCR_OUT2   = 1 << 3 // Auxiliary Output 2 (used to enable interrupts)
// 	MCR_LOOP   = 1 << 4 // Loopback Mode
// )
