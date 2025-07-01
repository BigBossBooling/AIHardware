package devices

// Standard I/O ports for the 8259A PIC
const (
	PIC1_COMMAND_PORT = 0x20
	PIC1_DATA_PORT    = 0x21
	PIC2_COMMAND_PORT = 0xA0
	PIC2_DATA_PORT    = 0xA1
)

// PIC Initialization Command Words (ICWs)
const (
	ICW1_INIT        = 0x10 // Initialization command
	ICW1_ICW4        = 0x01 // ICW4 (not) needed
	ICW1_SINGLE      = 0x02 // Single (cascade) mode
	ICW1_INTERVAL4   = 0x04 // Call address interval 4 (8)
	ICW1_LEVEL       = 0x08 // Level triggered (edge) mode
	ICW4_8086        = 0x01 // 8086/88 (MCS-80/85) mode
	ICW4_AUTO        = 0x02 // Auto (normal) EOI
	ICW4_BUF_SLAVE   = 0x08 // Buffered mode/slave
	ICW4_BUF_MASTER  = 0x0C // Buffered mode/master
	ICW4_SFNM        = 0x10 // Special fully nested (not)
)

// PIC Operation Command Words (OCWs)
const (
	OCW2_EOI             = 0x20 // End-of-interrupt command code
	OCW2_SPECIFIC_EOI    = 0x60 // Specific EOI
	OCW2_ROTATE_SPECIFIC = 0xE0 // Rotate in specific EOI mode
	OCW3_READ_IRR        = 0x0A // Read IRR command
	OCW3_READ_ISR        = 0x0B // Read ISR command
)

// IRQ numbers
const (
	IRQ_TIMER    = 0
	IRQ_KEYBOARD = 1
	IRQ_CASCADE  = 2 // PIC2 cascade
	IRQ_SERIAL2  = 3 // COM2
	IRQ_SERIAL1  = 4 // COM1
	IRQ_LPT2     = 5
	IRQ_FLOPPY   = 6
	IRQ_LPT1     = 7
	IRQ_RTC      = 8 // Real Time Clock (PIC2)
	// IRQs 9-15 are for PIC2
)

// RTC I/O Ports
const (
	RTC_INDEX_PORT = 0x70 // Address Port
	RTC_DATA_PORT  = 0x71 // Data Port
	RTC_NMI_DISABLE_MASK = 0x80 // Bit 7 of Index Port write: 1 = NMI disabled
)

// RTC Register Indices / Selectors
const (
	RTC_REG_SECONDS         = 0x00
	RTC_REG_MINUTES         = 0x02
	RTC_REG_HOURS           = 0x04
	RTC_REG_DAY_OF_WEEK     = 0x06
	RTC_REG_DAY_OF_MONTH    = 0x07
	RTC_REG_MONTH           = 0x08
	RTC_REG_YEAR            = 0x09
	RTC_REG_STATUS_A        = 0x0A // Update in progress, divider, rate
	RTC_REG_STATUS_B        = 0x0B // Control: periodic int, alarm int, update-ended int, square wave, binary/BCD, 24/12 hour, DST
	RTC_REG_STATUS_C        = 0x0C // Interrupt flags (read-only, cleared on read)
	RTC_REG_STATUS_D        = 0x0D // Valid RAM (VRT bit, read-only)

	RTC_REG_SECONDS_ALARM   = 0x01
	RTC_REG_MINUTES_ALARM   = 0x03
	RTC_REG_HOURS_ALARM     = 0x05

	// Common non-standard registers often used by BIOS/OS
	RTC_REG_CENTURY_DEFAULT = 0x32 // Example for century byte, often OS/BIOS specific
)

// RTC Status Register A Bits
const (
	RTC_REGA_UIP          = 1 << 7 // Update In Progress (Read-Only)
	RTC_REGA_DV_MASK      = 0x70   // Divider bits (DV2, DV1, DV0)
	RTC_REGA_DV_32768HZ   = 0x20   // DV = 010 (32.768kHz crystal)
	RTC_REGA_RATE_MASK    = 0x0F   // Rate Selection bits (RS3-RS0)
)

// RTC Status Register B Bits
const (
	RTC_REGB_SET          = 1 << 7 // 0: Update cycles normal, 1: Inhibit updates to time/date registers
	RTC_REGB_PIE          = 1 << 6 // Periodic Interrupt Enable
	RTC_REGB_AIE          = 1 << 5 // Alarm Interrupt Enable
	RTC_REGB_UIE          = 1 << 4 // Update-Ended Interrupt Enable
	RTC_REGB_SQWE         = 1 << 3 // Square Wave Enable
	RTC_REGB_DM           = 1 << 2 // Data Mode: 0=BCD, 1=Binary
	RTC_REGB_24H          = 1 << 1 // Hour Mode: 0=12 hour, 1=24 hour
	RTC_REGB_DSE          = 1 << 0 // Daylight Savings Time Enable
)

// RTC Status Register C Bits (Read-Only, flags cleared on read)
const (
	RTC_REGC_IRQF         = 1 << 7 // IRQ Line Active (logical OR of PF, AF, UF if corresponding enable in Reg B is set)
	RTC_REGC_PF           = 1 << 6 // Periodic Interrupt Flag
	RTC_REGC_AF           = 1 << 5 // Alarm Interrupt Flag
	RTC_REGC_UF           = 1 << 4 // Update-Ended Interrupt Flag
	// Bits 3-0 are reserved
)

// RTC Status Register D Bits (Read-Only)
const (
	RTC_REGD_VRT          = 1 << 7 // Valid RAM and Time: 1 = CMOS battery good, 0 = CMOS power lost
	// Bits 6-0 are reserved
)


// PIT (8253/8254 Programmable Interval Timer) I/O Ports
const (
	PIT_CHANNEL0_DATA = 0x40
	PIT_CHANNEL1_DATA = 0x41 // Not typically used by OS
	PIT_CHANNEL2_DATA = 0x42 // Speaker
	PIT_COMMAND_REG   = 0x43
)

// PIT Command Register bits
const (
	PIT_CMD_BINARY_MODE    = 0    // 0 = 16-bit binary, 1 = four-digit BCD
	PIT_CMD_MODE0          = 0 << 1 // Mode 0: Interrupt on terminal count
	PIT_CMD_MODE1          = 1 << 1 // Mode 1: Hardware re-triggerable one-shot
	PIT_CMD_MODE2          = 2 << 1 // Mode 2: Rate generator
	PIT_CMD_MODE3          = 3 << 1 // Mode 3: Square wave generator
	PIT_CMD_MODE4          = 4 << 1 // Mode 4: Software triggered strobe
	PIT_CMD_MODE5          = 5 << 1 // Mode 5: Hardware triggered strobe
	PIT_CMD_RW_LATCH       = 0 << 4 // Latch count value command
	PIT_CMD_RW_LSB_ONLY    = 1 << 4 // Read/Write least significant byte only
	PIT_CMD_RW_MSB_ONLY    = 2 << 4 // Read/Write most significant byte only
	PIT_CMD_RW_LSB_MSB     = 3 << 4 // Read/Write LSB then MSB
	PIT_CMD_CHANNEL0       = 0 << 6
	PIT_CMD_CHANNEL1       = 1 << 6
	PIT_CMD_CHANNEL2       = 2 << 6
	PIT_CMD_READ_BACK      = 3 << 6 // Read-back command (8254 only)
)

// PIT default frequency (Hz)
const PIT_BASE_FREQUENCY = 1193182 // Approximately 1.193 MHz

// PIT Command Register Masks (often used in programming)
const (
	PIT_CMD_CHANNEL_MASK = 3 << 6 // Selects channel 0, 1, or 2
	PIT_CMD_ACCESS_MASK  = 3 << 4 // Selects access mode (latch, lo, hi, lo/hi)
	PIT_CMD_MODE_MASK    = 7 << 1 // Selects operating mode 0-5
	PIT_CMD_BCD_MASK     = 1      // Selects BCD (1) or binary (0) mode
)

// PIT Read-Back Command Bits (8254 only)
const (
	PIT_RB_DONT_LATCH_COUNT  = 1 << 5 // Bit 5: 0 = latch count of selected counters
	PIT_RB_DONT_LATCH_STATUS = 1 << 4 // Bit 4: 0 = latch status of selected counters
	PIT_RB_CHANNEL2          = 1 << 3 // Bit 3: Select counter 2
	PIT_RB_CHANNEL1          = 1 << 2 // Bit 2: Select counter 1
	PIT_RB_CHANNEL0          = 1 << 1 // Bit 1: Select counter 0
	// Bit 0 is reserved (must be 0)
	// Bit 7 and 6 must be 1 for read-back command (11xxxxxx)
)

// System Control Port B (often related to PIT channel 2 gate/speaker)
const SYSTEM_CONTROL_PORT_B = 0x61


// Serial Port (UART 16550A) I/O Ports (COM1)
const (
	SERIAL_COM1_DATA_PORT          = 0x3F8 // Data Register (R/W) / Baud Rate Divisor LSB (W, DLAB=1)
	SERIAL_COM1_INTERRUPT_ENABLE   = 0x3F9 // Interrupt Enable Register (R/W) / Baud Rate Divisor MSB (W, DLAB=1)
	SERIAL_COM1_INTERRUPT_IDENT    = 0x3FA // Interrupt Identification Register (R) / FIFO Control Register (W)
	SERIAL_COM1_LINE_CONTROL       = 0x3FB // Line Control Register (R/W)
	SERIAL_COM1_MODEM_CONTROL      = 0x3FC // Modem Control Register (R/W)
	SERIAL_COM1_LINE_STATUS        = 0x3FD // Line Status Register (R)
	SERIAL_COM1_MODEM_STATUS       = 0x3FE // Modem Status Register (R)
	SERIAL_COM1_SCRATCH_REGISTER   = 0x3FF // Scratch Register (R/W)
)

// Line Control Register (LCR) bits
const (
	LCR_DLAB = 1 << 7 // Divisor Latch Access Bit
	// Other LCR bits for word length, stop bits, parity are also common
)

// Interrupt Enable Register (IER) bits
const (
	IER_RX_AVAILABLE = 1 << 0 // Enable Received Data Available Interrupt
	IER_TX_EMPTY     = 1 << 1 // Enable Transmitter Holding Register Empty Interrupt
	IER_RX_LINE_STAT = 1 << 2 // Enable Receiver Line Status Interrupt
	IER_MODEM_STAT   = 1 << 3 // Enable Modem Status Interrupt
)

// Interrupt Identification Register (IIR) bits
const (
	IIR_NO_INTERRUPT_PENDING = 0x01
	IIR_MODEM_STATUS         = 0x00
	IIR_TX_HOLDING_EMPTY     = 0x02
	IIR_RX_DATA_AVAILABLE    = 0x04
	IIR_LINE_STATUS          = 0x06
	IIR_FIFO_TIMEOUT         = 0x0C // Only on 16550+
	IIR_FIFO_ENABLED_64B     = 0x20 // 16750
	IIR_FIFO_ENABLED         = 0xC0 // Both bits set if FIFO enabled (16550+)
	IIR_INTERRUPT_ID_MASK    = 0x0F // Masks the interrupt ID part of IIR (bits 0-3)
)

// Line Status Register (LSR) bits
const (
	LSR_DATA_READY           = 1 << 0 // Data Ready
	LSR_OVERRUN_ERROR        = 1 << 1 // Overrun Error
	LSR_PARITY_ERROR         = 1 << 2 // Parity Error
	LSR_FRAMING_ERROR        = 1 << 3 // Framing Error
	LSR_BREAK_INTERRUPT      = 1 << 4 // Break Interrupt
	LSR_TX_HOLDING_REG_EMPTY = 1 << 5 // Transmitter Holding Register Empty
	LSR_TX_EMPTY             = 1 << 6 // Transmitter Empty
	LSR_FIFO_ERROR           = 1 << 7 // Error in RCVR FIFO (16550+)
)

// Modem Control Register (MCR) bits
const (
	MCR_DTR    = 1 << 0 // Data Terminal Ready
	MCR_RTS    = 1 << 1 // Request To Send
	MCR_OUT1   = 1 << 2 // Auxiliary Output 1
	MCR_OUT2   = 1 << 3 // Auxiliary Output 2 / Interrupt Enable (used to enable interrupts from the UART)
	MCR_LOOP   = 1 << 4 // Loopback mode
)
