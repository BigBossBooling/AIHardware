package devices

// PIT (8254 Programmable Interval Timer) I/O Ports
const (
	PIT_COUNTER0_PORT = 0x40 // Channel 0 data port (read/write)
	PIT_COUNTER1_PORT = 0x41 // Channel 1 data port (read/write)
	PIT_COUNTER2_PORT = 0x42 // Channel 2 data port (read/write)
	PIT_COMMAND_PORT  = 0x43 // Command Register (write only)
)

// PIT Command Register bits
// Bits 7-6: Select Channel
const (
	PIT_CMD_CHANNEL0    = 0x00 // 00xxxxxx
	PIT_CMD_CHANNEL1    = 0x40 // 01xxxxxx
	PIT_CMD_CHANNEL2    = 0x80 // 10xxxxxx
	PIT_CMD_READ_BACK   = 0xC0 // 11xxxxxx (Read-Back command)
	PIT_CMD_CHANNEL_MASK = 0xC0
)

// Bits 5-4: Access Mode
const (
	PIT_CMD_LATCH_COUNT = 0x00 // 0000xxxx (Latch count value command)
	PIT_CMD_ACCESS_LO   = 0x10 // xx01xxxx (RW LSB only)
	PIT_CMD_ACCESS_HI   = 0x20 // xx10xxxx (RW MSB only)
	PIT_CMD_ACCESS_LOHI = 0x30 // xx11xxxx (RW LSB then MSB)
	PIT_CMD_ACCESS_MASK = 0x30
)

// Bits 3-1: Operating Mode
const (
	PIT_CMD_MODE0 = 0x00 // xxx000xx (Interrupt on Terminal Count)
	PIT_CMD_MODE1 = 0x02 // xxx001xx (Hardware Retriggerable One-Shot)
	PIT_CMD_MODE2 = 0x04 // xxx010xx (Rate Generator)
	PIT_CMD_MODE3 = 0x06 // xxx011xx (Square Wave Generator)
	PIT_CMD_MODE4 = 0x08 // xxx100xx (Software Triggered Strobe)
	PIT_CMD_MODE5 = 0x0A // xxx101xx (Hardware Triggered Strobe)
	// Modes 6 and 7 are same as 2 and 3 for 8254
	PIT_CMD_MODE_MASK = 0x0E
)

// Bit 0: BCD/Binary Mode
const (
	PIT_CMD_BINARY = 0x00 // xxxxxxx0 (16-bit binary counter)
	PIT_CMD_BCD    = 0x01 // xxxxxxx1 (4-digit BCD counter)
	PIT_CMD_BCD_MASK = 0x01
)

// PIT Read-Back Command bits (when CMD_READ_BACK is selected)
const (
	PIT_RB_DONT_LATCH_COUNT  = 0x20 // D5: 0 = latch count, 1 = don't latch count
	PIT_RB_DONT_LATCH_STATUS = 0x10 // D4: 0 = latch status, 1 = don't latch status
	PIT_RB_CHANNEL2          = 0x08 // D3: Select channel 2
	PIT_RB_CHANNEL1          = 0x04 // D2: Select channel 1
	PIT_RB_CHANNEL0          = 0x02 // D1: Select channel 0
	// Bit 0 is reserved (must be 0)
)

// Port 0x61 - System Control Port B / NMI Status and Control Register
// Used for PIT channel 2 gate and speaker control
const (
	SYSTEM_CONTROL_PORT_B = 0x61
	PIT_GATE2_SPEAKER_ENABLE_BIT = 1 << 1 // Bit 1: PIT Channel 2 Gate to Speaker Enable
	PIT_SPEAKER_DATA_ENABLE_BIT  = 1 << 0 // Bit 0: PIT Speaker Data Enable
)


// RTC (Real-Time Clock - MC146818 compatible, often part of CMOS RAM) I/O Ports
const (
	RTC_INDEX_PORT = 0x70 // CMOS Address Register (selects register)
	                    // Bit 7 of this port is often NMI disable flag (1 = NMI disabled)
	RTC_DATA_PORT  = 0x71 // CMOS Data Register (reads/writes selected register)
)

const RTC_NMI_DISABLE_MASK = 0x80

// RTC Register Indices (selected via RTC_INDEX_PORT)
const (
	RTC_REG_SECONDS       = 0x00
	RTC_REG_SECONDS_ALARM = 0x01
	RTC_REG_MINUTES       = 0x02
	RTC_REG_MINUTES_ALARM = 0x03
	RTC_REG_HOURS         = 0x04
	RTC_REG_HOURS_ALARM   = 0x05
	RTC_REG_DAY_OF_WEEK   = 0x06
	RTC_REG_DAY_OF_MONTH  = 0x07
	RTC_REG_MONTH         = 0x08
	RTC_REG_YEAR          = 0x09

	RTC_REG_STATUS_A = 0x0A
	RTC_REG_STATUS_B = 0x0B
	RTC_REG_STATUS_C = 0x0C
	RTC_REG_STATUS_D = 0x0D

	// Other registers (e.g., for century, or specific model features)
	RTC_REG_CENTURY_DEFAULT = 0x32 // Common location for century byte in some chipsets
)

// RTC Status Register A (REG_STATUS_A - 0x0A) bits
const (
	RTC_REGA_UIP = 0x80 // Update In Progress (1 = time update in progress, access inhibited)
	// Bits 6-4: Time base divider (DV2, DV1, DV0) - usually set to 010 for 32.768 kHz crystal
	RTC_REGA_DV_32768HZ = 0x20 // 010xxxxx
	// Bits 3-0: Rate Selection for periodic interrupt and square wave output (RS3-RS0)
	RTC_REGA_RATE_MASK = 0x0F
)

// RTC Status Register B (REG_STATUS_B - 0x0B) bits
const (
	RTC_REGB_SET = 0x80 // 1 = Inhibit updates to time/date registers
	RTC_REGB_PIE = 0x40 // Periodic Interrupt Enable (1 = enabled)
	RTC_REGB_AIE = 0x20 // Alarm Interrupt Enable (1 = enabled)
	RTC_REGB_UIE = 0x10 // Update-ended Interrupt Enable (1 = enabled)
	RTC_REGB_SQWE= 0x08 // Square Wave Output Enable (1 = enabled)
	RTC_REGB_DM  = 0x04 // Data Mode: 0 = BCD, 1 = Binary. (Most systems use BCD)
	RTC_REGB_24H = 0x02 // 0 = 12-hour mode, 1 = 24-hour mode
	RTC_REGB_DSE = 0x01 // Daylight Saving Enable (1 = enabled)
)

// RTC Status Register C (REG_STATUS_C - 0x0C) bits - Read Only, flags cleared on read
const (
	RTC_REGC_IRQF = 0x80 // Interrupt Request Flag (any of PF, AF, UF is 1)
	RTC_REGC_PF   = 0x40 // Periodic Interrupt Flag
	RTC_REGC_AF   = 0x20 // Alarm Interrupt Flag
	RTC_REGC_UF   = 0x10 // Update-ended Interrupt Flag
	// Bits 3-0 are reserved (0)
)

// RTC Status Register D (REG_STATUS_D - 0x0D) bits - Read Only
const (
	RTC_REGD_VRT = 0x80 // Valid RAM and Time (1 = CMOS battery good)
	// Bits 6-0 are reserved (0)
)

// Common RTC values
const (
	RTC_IRQ = 8 // Typical IRQ line for RTC
)
