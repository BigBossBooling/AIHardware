package devices

import (
	"fmt"
	"sync"
	"time"
)

const (
	rtcNumRegisters = 128 // Standard CMOS RAM size, though RTC uses fewer for timekeeping
	// RTC_REG_CENTURY_OS_DEPENDENT = 0x32 // Example: Linux often uses 0x32 for century
)

// RTCDevice represents the Real-Time Clock (MC146818 compatible).
type RTCDevice struct {
	mu sync.Mutex

	selectedIndex byte   // Currently selected register index (via port 0x70)
	registers     [rtcNumRegisters]byte // CMOS RAM, including RTC registers

	pic InterruptRaiser // For signaling RTC interrupts (IRQ 8)
	nmiDisabled bool
}

// NewRTCDevice creates and initializes a new RTCDevice.
func NewRTCDevice(pic InterruptRaiser) *RTCDevice {
	rtc := &RTCDevice{
		pic: pic,
	}
	rtc.initDefaultRegisters()
	return rtc
}

// initDefaultRegisters sets up initial values for RTC registers.
func (rtc *RTCDevice) initDefaultRegisters() {
	// Initialize Status Register D: VRT (Valid RAM and Time) should be set
	rtc.registers[RTC_REG_STATUS_D] = RTC_REGD_VRT

	// Initialize Status Register B: 24-hour mode, Binary mode (or BCD if preferred by default)
	// For simplicity, let's default to 24-hour and BCD mode as it's common.
	// RTC_REGB_DM = 0 for BCD.
	rtc.registers[RTC_REG_STATUS_B] = RTC_REGB_24H //  | RTC_REGB_DM (if binary)

	// Initialize Status Register A: Default DV (divider) and RS (rate select)
	// DV = 010 (32.768kHz crystal), RS = 0110 (1.024ms period for periodic int, ~976Hz)
	rtc.registers[RTC_REG_STATUS_A] = RTC_REGA_DV_32768HZ | 0x06

	// Other registers (like time/date) will be populated on demand from host time
	// or can be set to a default fixed time for testing.
	// Alarm registers default to 0.
	// Century byte (e.g. at 0x32 or ACPI defined) could be set here if known.
	// For now, we will update time from host when read.
}

// HandleIO processes I/O operations on the RTC registers.
// Returns the value read for read operations, and an error if any.
func (rtc *RTCDevice) HandleIO(port uint16, data []byte, isWrite bool) (uint8, error) {
	rtc.mu.Lock()
	defer rtc.mu.Unlock()

	var val byte = 0
	var err error = nil

	if isWrite {
		if len(data) == 0 {
			return 0, fmt.Errorf("rtc: write operation with no data on port 0x%x", port)
		}
		writeData := data[0]

		switch port {
		case RTC_INDEX_PORT:
			rtc.selectedIndex = writeData & 0x7F // Lower 7 bits select register index
			rtc.nmiDisabled = (writeData & RTC_NMI_DISABLE_MASK) != 0
			// fmt.Printf("RTC: Index set to 0x%02X, NMI Disabled: %v\n", rtc.selectedIndex, rtc.nmiDisabled)
		case RTC_DATA_PORT:
			// fmt.Printf("RTC: Write to Data Port (Index 0x%02X): Value 0x%02X\n", rtc.selectedIndex, writeData)
			rtc.writeRegister(rtc.selectedIndex, writeData)
		default:
			err = fmt.Errorf("rtc: write to unhandled port 0x%x", port)
		}
	} else { // Read operation
		switch port {
		case RTC_INDEX_PORT:
			// Reading from index port usually returns the last written value, including NMI bit.
			val = rtc.selectedIndex
			if rtc.nmiDisabled {
				val |= RTC_NMI_DISABLE_MASK
			}
		case RTC_DATA_PORT:
			val = rtc.readRegister(rtc.selectedIndex)
			// fmt.Printf("RTC: Read from Data Port (Index 0x%02X): Value 0x%02X\n", rtc.selectedIndex, val)
		default:
			err = fmt.Errorf("rtc: read from unhandled port 0x%x", port)
		}
	}
	return val, err
}

// toBCD converts a binary number to BCD.
func toBCD(val int) byte {
	return byte(((val / 10) << 4) | (val % 10))
}

// fromBCD converts a BCD number to binary.
func fromBCD(bcd byte) int {
	return int(((bcd>>4)&0x0F)*10 + (bcd & 0x0F))
}

// readRegister handles reading from a specific RTC/CMOS register index.
func (rtc *RTCDevice) readRegister(index byte) byte {
	// Check for Update In Progress (UIP) bit in Status Register A
	// If UIP is set, reads to time/date registers might return undefined values or last known good.
	// For simplicity, we'll allow reads but a real RTC might stall or return specific values.
	if (rtc.registers[RTC_REG_STATUS_A] & RTC_REGA_UIP) != 0 && index <= RTC_REG_YEAR {
		// Handle as per hardware spec, e.g., return 0xFF or last latched value.
		// For now, we'll proceed to read current time, which is a common behavior for emulators.
		// fmt.Println("RTC: Read while UIP active.")
	}

	isBCD := (rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_DM) == 0
	is12Hour := (rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_24H) == 0 // 0 means 12-hour mode

	now := time.Now() // Get current host time

	switch index {
	case RTC_REG_SECONDS:
		sec := now.Second()
		return ifThen(isBCD, toBCD(sec), byte(sec))
	case RTC_REG_MINUTES:
		min := now.Minute()
		return ifThen(isBCD, toBCD(min), byte(min))
	case RTC_REG_HOURS:
		hour := now.Hour()
		if is12Hour {
			isPM := hour >= 12
			if hour == 0 { // Midnight
				hour = 12
			} else if hour > 12 {
				hour -= 12
			}
			val := ifThen(isBCD, toBCD(hour), byte(hour))
			if isPM {
				val |= 0x80 // PM bit for 12-hour mode
			}
			return val
		}
		return ifThen(isBCD, toBCD(hour), byte(hour))
	case RTC_REG_DAY_OF_WEEK: // Sunday=1, ..., Saturday=7 (some BIOS might use 0-6)
		day := int(now.Weekday()) + 1 // time.Weekday() is Sunday=0 ... Saturday=6
		return ifThen(isBCD, toBCD(day), byte(day))
	case RTC_REG_DAY_OF_MONTH:
		day := now.Day()
		return ifThen(isBCD, toBCD(day), byte(day))
	case RTC_REG_MONTH:
		month := int(now.Month())
		return ifThen(isBCD, toBCD(month), byte(month))
	case RTC_REG_YEAR: // Last two digits of the year
		year := now.Year() % 100
		return ifThen(isBCD, toBCD(year), byte(year))

	// Status Registers
	case RTC_REG_STATUS_A:
		// UIP bit should be 0 if not actively updating. For reads, assume update is quick.
		// Real hardware sets UIP for ~244us during update cycle.
		// We can simulate it by briefly setting it if a write to time occurs, or assume reads are fast enough.
		// For now, return stored value, ensuring UIP is normally 0 unless we explicitly manage update cycles.
		return rtc.registers[RTC_REG_STATUS_A] & (^RTC_REGA_UIP & 0xFF) // Assume UIP is not active for reads generally
	case RTC_REG_STATUS_B:
		return rtc.registers[RTC_REG_STATUS_B]
	case RTC_REG_STATUS_C:
		// Reading Status Register C clears the interrupt flags (PF, AF, UF) and IRQF.
		val := rtc.registers[RTC_REG_STATUS_C]
		rtc.registers[RTC_REG_STATUS_C] = 0 // Clear flags after read
		// Reading Status Register C clears the interrupt flags. If IRQF was set, it means an interrupt was pending.
		// The PIC's corresponding IRQ line (IRQ8) would have been raised.
		// Clearing IRQF here implies the source of interrupt is acknowledged at the RTC level.
		// The CPU would still need to EOI the PIC.
		if (val & RTC_REGC_IRQF) != 0 && rtc.pic != nil {
			// This is a simplification. The IRQ is typically lowered by the PIC after EOI,
			// or if the RTC de-asserts its interrupt line.
			// For edge-triggered interrupts, simply clearing flags in RTC might not lower line immediately.
			// However, preventing re-assertion until a new event is reasonable.
			// rtc.pic.LowerIRQ(rtc.GetIRQLine()) // Tentative, depends on how LowerIRQ is implemented for PIC
		}
		return val
	case RTC_REG_STATUS_D:
		// VRT bit (CMOS battery status)
		return rtc.registers[RTC_REG_STATUS_D] // Should be RTC_REGD_VRT if battery is good

	// Alarm registers - just return stored values
	case RTC_REG_SECONDS_ALARM, RTC_REG_MINUTES_ALARM, RTC_REG_HOURS_ALARM:
		// TODO: Handle "don't care" bits (C0-FF) for alarm values if isBCD
		return rtc.registers[index]

	// Century register (example, common location)
	// Some BIOSes use 0x32, others use ACPI FADT.
	case RTC_REG_CENTURY_DEFAULT: // Or other configured century byte location
		year := now.Year()
		century := year / 100
		return ifThen(isBCD, toBCD(century), byte(century))

	default:
		// For other CMOS RAM locations, return the stored byte.
		if index < rtcNumRegisters {
			return rtc.registers[index]
		}
		// fmt.Printf("RTC: Read from unhandled or out-of-bounds CMOS index 0x%02X\n", index)
		return 0xFF // Or some other default for out-of-bounds
	}
}

// writeRegister handles writing to a specific RTC/CMOS register index.
func (rtc *RTCDevice) writeRegister(index byte, value byte) {
	// Check if updates are inhibited by SET bit in Register B or UIP in Register A.
	if (rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_SET) != 0 {
		// fmt.Printf("RTC: Write to index 0x%02X inhibited by REGB_SET.\n", index)
		// Typically, only Status Register A and B can be written when SET is true.
		if index != RTC_REG_STATUS_A && index != RTC_REG_STATUS_B {
			return
		}
	}
	// TODO: Simulate UIP (Update In Progress) for writes to time/date registers.
	// This would involve setting UIP, performing write, then clearing UIP.
	// During UIP, reads to time/date are unstable.

	switch index {
	// Time and Date registers (generally read-only from host perspective, but guest can write)
	// Writing to these registers means the guest is trying to set the time.
	// We can either ignore these writes, or attempt to set host time (requires permissions and care),
	// or just store them and let the emulated RTC diverge from host time.
	// For now, we'll store them in our registers array.
	case RTC_REG_SECONDS, RTC_REG_MINUTES, RTC_REG_HOURS,
		RTC_REG_DAY_OF_WEEK, RTC_REG_DAY_OF_MONTH, RTC_REG_MONTH, RTC_REG_YEAR:
		// fmt.Printf("RTC: Guest writing 0x%02X to time/date register 0x%02X.\n", value, index)
		rtc.registers[index] = value
		// If guest writes to time, should probably clear Status Reg C flags related to Update-ended interrupt.

	// Alarm registers
	case RTC_REG_SECONDS_ALARM, RTC_REG_MINUTES_ALARM, RTC_REG_HOURS_ALARM:
		// TODO: Handle "don't care" bits if BCD mode (e.g. C0-FF) for alarm.
		// For now, store directly.
		rtc.registers[index] = value
		rtc.registers[RTC_REG_STATUS_C] &= (^RTC_REGC_AF & 0xFF) // Clear Alarm Flag if alarm is reprogrammed
		// fmt.Printf("RTC: Alarm register 0x%02X set to 0x%02X.\n", index, value)
		// checkAlarm() might be called here.

	case RTC_REG_STATUS_A:
		// RS3-RS0 (Rate Select) are writable. DV2-DV0 (Divider) are usually fixed. UIP is read-only.
		// Allow writing to RS part. Mask off read-only parts.
		currentRS := rtc.registers[RTC_REG_STATUS_A] & RTC_REGA_RATE_MASK
		newRS := value & RTC_REGA_RATE_MASK
		rtc.registers[RTC_REG_STATUS_A] = (rtc.registers[RTC_REG_STATUS_A] & (^RTC_REGA_RATE_MASK & 0xFF)) | newRS
		if newRS != currentRS {
			// TODO: Periodic timer rate changed. Reset periodic interrupt logic.
			// fmt.Printf("RTC: Status A Rate Select changed to 0x%X.\n", newRS)
		}
		// UIP bit should not be settable by guest.
		// DV bits are often read-only reflecting crystal, but some RTCs allow modification. Assume read-only for now.

	case RTC_REG_STATUS_B:
		// Writable bits: PIE, AIE, UIE, SQWE, DM, 24H, DSE. SET bit is also writable.
		// Preserve read-only bits if any (none in standard RTC Reg B).
		rtc.registers[RTC_REG_STATUS_B] = value
		// fmt.Printf("RTC: Status B set to 0x%02X.\n", value)
		// If PIE, AIE, UIE changed, update interrupt logic.
		// If DM or 24H changed, affects how time is read/written.
		// If SET bit changed, affects writability of time/date registers.
		// rtc.updateInterrupts()

	// Status Register C is Read-Only, writes should be ignored.
	case RTC_REG_STATUS_C:
		// fmt.Printf("RTC: Write to Status C (read-only) with 0x%02X, ignored.\n", value)
		return

	// Status Register D: VRT is Read-Only. Other bits reserved. Writes should be ignored.
	case RTC_REG_STATUS_D:
		// fmt.Printf("RTC: Write to Status D (read-only) with 0x%02X, ignored.\n", value)
		return

	// Century register
	case RTC_REG_CENTURY_DEFAULT: // Or other configured century byte
		rtc.registers[index] = value
		// fmt.Printf("RTC: Century register 0x%02X set to 0x%02X.\n", index, value)

	default:
		// For other CMOS RAM locations:
		if index < rtcNumRegisters {
			// fmt.Printf("RTC: CMOS RAM Index 0x%02X set to 0x%02X.\n", index, value)
			rtc.registers[index] = value
		} else {
			// fmt.Printf("RTC: Write to unhandled or out-of-bounds CMOS index 0x%02X with 0x%02X, ignored.\n", index, value)
		}
	}
}

// ifThen is a simple generic helper for conditional value assignment.
func ifThen[T any](condition bool, trueVal T, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}

// Tick function for RTC to handle periodic interrupts, alarms.
// This would be called by the main VM loop or a timer mechanism.
func (rtc *RTCDevice) Tick(elapsedTime time.Duration) {
	rtc.mu.Lock()
	defer rtc.mu.Unlock()

	// --- Update In Progress (UIP) Simulation ---
	// Real RTCs have an update cycle (~2s) where time registers are copied.
	// UIP is set in RegA during this copy (~244us).
	// This is complex to simulate perfectly with discrete ticks.
	// A simpler model: assume updates are atomic or very fast.
	// Or, if guest is sensitive, manage UIP state explicitly around time reads.

	// --- Periodic Interrupt ---
	if (rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_PIE) != 0 {
		// Check if periodic interrupt is enabled
		rateSelect := rtc.registers[RTC_REG_STATUS_A] & RTC_REGA_RATE_MASK
		if rateSelect >= 3 && rateSelect <= 15 { // Valid rates for periodic interrupt
			// Calculate period based on rateSelect (32768Hz / 2^(RS-1))
			// e.g. RS=6 -> 32768 / 2^5 = 1024 Hz (0.9765625 ms period)
			// This part needs a robust timer mechanism to check if period has elapsed.
			// For now, this is a placeholder for when such a timer is available.
			// If period elapsed:
			//   rtc.registers[RTC_REG_STATUS_C] |= RTC_REGC_PF | RTC_REGC_IRQF
			//   if rtc.pic != nil { rtc.pic.RaiseIRQ(rtc.GetIRQLine()) }
		}
	}

	// --- Alarm Interrupt ---
	if (rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_AIE) != 0 {
		// Check if alarm interrupt is enabled
		// Compare current time (from host or internal RTC model) with alarm registers.
		// This requires converting current time to BCD/binary based on RTC_REGB_DM
		// and handling 12/24 hour mode from RTC_REGB_24H.
		// Also handle "don't care" fields in alarm registers.
		// If alarm matches:
		//   rtc.registers[RTC_REG_STATUS_C] |= RTC_REGC_AF | RTC_REGC_IRQF
		//   if rtc.pic != nil { rtc.pic.RaiseIRQ(rtc.GetIRQLine()) }
	}

	// --- Update-Ended Interrupt ---
	// This interrupt occurs after each update cycle (typically once per second).
	// if (rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_UIE) != 0 {
	//   If one second has passed since last update-ended interrupt:
	//     rtc.registers[RTC_REG_STATUS_C] |= RTC_REGC_UF | RTC_REGC_IRQF
	//     if rtc.pic != nil { rtc.pic.RaiseIRQ(rtc.GetIRQLine()) }
	// }

	// Note: The actual interrupt generation (calling rtc.pic.RaiseIRQ)
	// depends on the PIC being implemented and integrated.
	// If any of the flags (PF, AF, UF) got set and their respective enable bits (PIE, AIE, UIE) are active,
	// and the overall interrupt output from RTC is enabled (e.g. not masked by some other condition),
	// then IRQF in Reg C should be set and PIC signaled.

	// Simplified: if any of PF, AF, UF is set AND corresponding enable is set
	shouldInterrupt := false
	if (rtc.registers[RTC_REG_STATUS_C] & RTC_REGC_PF) != 0 && (rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_PIE) != 0 {
		shouldInterrupt = true
	}
	if (rtc.registers[RTC_REG_STATUS_C] & RTC_REGC_AF) != 0 && (rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_AIE) != 0 {
		shouldInterrupt = true
	}
	if (rtc.registers[RTC_REG_STATUS_C] & RTC_REGC_UF) != 0 && (rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_UIE) != 0 {
		shouldInterrupt = true
	}

	if shouldInterrupt {
		rtc.registers[RTC_REG_STATUS_C] |= RTC_REGC_IRQF // Set master interrupt flag
		if rtc.pic != nil {
			// fmt.Printf("RTC: Raising IRQ %d due to event. Reg C: 0x%02X\n", rtc.GetIRQLine(), rtc.registers[RTC_REG_STATUS_C])
			rtc.pic.RaiseIRQ(rtc.GetIRQLine())
		}
	}
}

// GetIRQLine returns the IRQ line this device would use.
func (rtc *RTCDevice) GetIRQLine() uint8 {
	return IRQ_RTC // Standard RTC IRQ is 8, defined as IRQ_RTC in pic_constants.go
}
