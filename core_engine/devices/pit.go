package devices

import (
	"fmt"
	"sync"
	"time" // For simulating timer behavior, if needed for more advanced features later
)

const (
	pitNumChannels = 3
	pitMaxCount    = 0xFFFF // Max 16-bit counter value
	pitFrequency   = 1193182 // PIT base frequency in Hz (approx 1.193 MHz)
)

// PITChannel represents a single channel of the 8254 PIT.
type PITChannel struct {
	mu sync.Mutex

	count          uint16 // Current counter value (counts down)
	reloadValue    uint16 // Value to reload into count when it reaches 0 (mode dependent)
	latchedValue   uint16 // Latched value for reading
	latchedStatus  byte   // Latched status for read-back

	mode         byte   // Operating mode (0-5)
	accessMode   byte   // Access mode (lo, hi, lo/hi)
	bcdMode      bool   // BCD (true) or binary (false)

	readState    byte   // For 2-byte reads (LSB then MSB)
	writeState   byte   // For 2-byte writes (LSB then MSB)

	gate          bool   // Gate input (true = active/counting enabled)
	output        bool   // Current output state (OUT pin)
	lastClockTime time.Time // For more accurate simulation if needed

	// Fields for read-back command
	countLatched  bool
	statusLatched bool
	nullCount     bool // True if count has not been loaded since mode set.
	outputPin     bool // Reflects the actual state of the OUT pin.
}

// PITDevice represents the 8254 Programmable Interval Timer.
type PITDevice struct {
	mu       sync.Mutex
	channels [pitNumChannels]PITChannel
	pic      InterruptRaiser // Reference to PIC for generating interrupts
	port61State byte // Store state of relevant bits from port 0x61
}

// NewPITDevice creates and initializes a new PITDevice.
func NewPITDevice(pic InterruptRaiser) *PITDevice {
	pit := &PITDevice{
		pic: pic,
	}
	for i := 0; i < pitNumChannels; i++ {
		pit.channels[i].mode = PIT_CMD_MODE0 // Default mode, though typically configured by BIOS
		pit.channels[i].accessMode = PIT_CMD_RW_LSB_MSB // Default access (lo/hi)
		pit.channels[i].gate = (i < 2) // Channel 0 and 1 gates are usually high by default
		pit.channels[i].output = false // Default output state
		pit.channels[i].nullCount = true
		pit.channels[i].lastClockTime = time.Now()
	}
	// Channel 2's gate is often controlled by port 0x61 bit 0.
	// Initial state of port61State can be 0.
	return pit
}

// HandleIO processes I/O operations on the PIT registers.
// Returns the value read for read operations (or 0 for writes), and an error if any.
func (pit *PITDevice) HandleIO(port uint16, data []byte, isWrite bool) (uint8, error) {
	pit.mu.Lock()
	defer pit.mu.Unlock()

	// Handle Port 0x61 (System Control Port B) separately if it's part of PIT's responsibility
	// In many systems, port 0x61 is handled by a system controller that interacts with the PIT.
	// For now, we'll assume direct control or that another device handles non-PIT aspects of 0x61.
	if port == SYSTEM_CONTROL_PORT_B {
		if isWrite {
			val := data[0]
			pit.port61State = val
			// Bit 0: Speaker Data Enable. Bit 1: Speaker Enable from PIT channel 2.
			// Update channel 2 gate based on bit 0 of port 0x61 (this is a common setup)
			// ch2Gate := (val & 0x01) != 0 // Typically bit 0 of port 0x61 controls gate of channel 2
			// pit.channels[2].setGate(ch2Gate)
			// fmt.Printf("PIT: Port 0x61 write: 0x%02X. Channel 2 gate/speaker TBD.\n", val)
			return 0, nil
		}
		// Reading from 0x61 returns its current state.
		// This might also include NMI status bits, refresh toggle, etc. from other hardware.
		// We only manage bits relevant to PIT if any.
		// For now, just return the stored PIT-relevant state.
		// A full port 0x61 emulation is complex.
		return pit.port61State, nil // Simplified
	}

	// Check if the port is one of the PIT channel data ports or the command register
	isPitPort := false
	channelIndex := -1

	switch port {
	case PIT_CHANNEL0_DATA:
		isPitPort = true
		channelIndex = 0
	case PIT_CHANNEL1_DATA:
		isPitPort = true
		channelIndex = 1
	case PIT_CHANNEL2_DATA:
		isPitPort = true
		channelIndex = 2
	case PIT_COMMAND_REG:
		isPitPort = true
		// channelIndex remains -1 for command register
	}

	if isPitPort {
		if isWrite {
			if len(data) == 0 {
				return 0, fmt.Errorf("pit: write operation with no data on port 0x%x", port)
			}
			val := data[0]

			if port == PIT_COMMAND_REG {
				pit.handleCommandWrite(val)
			} else { // Must be a channel data port
				pit.channels[channelIndex].handleCounterWrite(val)
			}
			return 0, nil // No value returned for writes to PIT data/command ports
		} else { // Read operation
			if port == PIT_COMMAND_REG {
				// Command port is write-only according to most specs. Some emulators return 0 or last value.
				// Bochs returns 0. Let's stick to that.
				return 0, nil
			} else { // Must be a channel data port
				return pit.channels[channelIndex].handleCounterRead(), nil
			}
		}
	}

	return 0, fmt.Errorf("pit: access to unhandled port 0x%x", port)
}

// handleCommandWrite processes a write to the PIT command register (0x43).
func (pit *PITDevice) handleCommandWrite(cmd byte) {
	channelSelect := (cmd & PIT_CMD_CHANNEL_MASK) >> 6
	accessMode := (cmd & PIT_CMD_ACCESS_MASK)
	opMode := (cmd & PIT_CMD_MODE_MASK)
	// In pic_constants.go: PIT_CMD_BINARY_MODE = 0 means binary, 1 means BCD.
	// PIT_CMD_BCD_MASK is 1. So if (cmd & PIT_CMD_BCD_MASK) != 0, it's BCD.
	bcd := (cmd & PIT_CMD_BCD_MASK) != 0

	// fmt.Printf("PIT CMD: 0x%02X (Ch: %d, Access: 0x%X, Mode: 0x%X, BCD: %v)\n",
	//	cmd, channelSelect, accessMode>>4, opMode>>1, bcd)

	if channelSelect == (PIT_CMD_READ_BACK >> 6) {
		// Read-back command
		pit.handleReadBackCommand(cmd)
		return
	}

	ch := &pit.channels[channelSelect]
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if accessMode == PIT_CMD_RW_LATCH { // Latch command for selected channel
		ch.latchedValue = ch.readCurrentCount()
		ch.countLatched = true
		ch.readState = 0 // Reset read state for subsequent reads from counter port
		// fmt.Printf("PIT Ch %d: Count latched (value=0x%04X)\n", channelSelect, ch.latchedValue)
	} else {
		// Mode/Access programming command
		ch.mode = opMode
		ch.accessMode = accessMode
		ch.bcdMode = bcd
		ch.writeState = 0 // Reset write state for new programming sequence
		ch.readState = 0  // Reset read state as well
		ch.nullCount = true // New mode set, count is considered "null" until loaded
		ch.output = (opMode == PIT_CMD_MODE1 || opMode == PIT_CMD_MODE5) // Modes 1 and 5 start with OUT high.
		// Other modes (0, 2, 3, 4) start with OUT low until timeout or trigger.
		// This is a simplification; initial OUT state can be complex.
		// fmt.Printf("PIT Ch %d: Programmed. Mode: %d, Access: %d, BCD: %v\n",
		//	channelSelect, ch.mode>>1, ch.accessMode>>4, ch.bcdMode)

		// If mode 3, output starts high if counter is loaded.
		// Here, just setting mode, actual output change on load.
		if ch.mode == PIT_CMD_MODE3 {
			ch.output = true // Mode 3 output generally starts high after programming, goes low after first half-period.
		}
	}
}

// handleReadBackCommand processes a PIT read-back command.
func (pit *PITDevice) handleReadBackCommand(cmd byte) {
	// fmt.Printf("PIT Read-Back CMD: 0x%02X\n", cmd)
	latchCount := (cmd&PIT_RB_DONT_LATCH_COUNT) == 0
	latchStatus := (cmd&PIT_RB_DONT_LATCH_STATUS) == 0

	for i := 0; i < pitNumChannels; i++ {
		if (cmd & (PIT_RB_CHANNEL0 << i)) != 0 { // Check if this channel is selected
			ch := &pit.channels[i]
			ch.mu.Lock()
			if latchStatus {
				// Status byte: [OUT | NULL_COUNT | RW_MODE(2) | MODE(3) | BCD]
				// OUT pin state, Null count, RW mode, Op mode, BCD mode
				status := byte(0)
				if ch.outputPin { // Actual OUT pin state
					status |= 0x80
				}
				if ch.nullCount { // True if count value is undefined
					status |= 0x40
				}
				status |= (ch.accessMode & 0x30) // Bits 5-4: RW mode (already shifted)
				status |= (ch.mode & 0x0E)       // Bits 3-1: Operating mode (already shifted)
				if ch.bcdMode {
					status |= 0x01
				}
				ch.latchedStatus = status
				ch.statusLatched = true
				// fmt.Printf("PIT Ch %d: Status latched (0x%02X)\n", i, status)
			}
			if latchCount {
				// This is different from channel-specific latch command.
				// Read-back latch does not affect channel's readState for counter port reads.
				// It makes the current count available for a read-back status structure,
				// but standard counter port reads are not affected by this type of latch.
				// For simplicity, we can reuse latchedValue, but be mindful of this distinction.
				ch.latchedValue = ch.readCurrentCount() // Read current count for read-back
				ch.countLatched = true // Indicate count is available via status read or for counter read
				// fmt.Printf("PIT Ch %d: Count latched for read-back (value=0x%04X)\n", i, ch.latchedValue)
			}
			ch.mu.Unlock()
		}
	}
}


// handleCounterWrite processes a write to a PIT counter data port.
func (ch *PITChannel) handleCounterWrite(val byte) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// fmt.Printf("PIT Counter Write: Val=0x%02X, AccessMode=0x%X, WriteState=%d\n", val, ch.accessMode>>4, ch.writeState)

	switch ch.accessMode {
	case PIT_CMD_RW_LSB_ONLY: // LSB only
		ch.reloadValue = uint16(val)
		ch.writeState = 0 // Done
	case PIT_CMD_RW_MSB_ONLY: // MSB only
		ch.reloadValue = uint16(val) << 8
		ch.writeState = 0 // Done
	case PIT_CMD_RW_LSB_MSB: // LSB then MSB
		if ch.writeState == 0 { // Expecting LSB
			ch.reloadValue = (ch.reloadValue & 0xFF00) | uint16(val)
			ch.writeState = 1 // Next is MSB
		} else { // Expecting MSB
			ch.reloadValue = (ch.reloadValue & 0x00FF) | (uint16(val) << 8)
			ch.writeState = 0 // Done with this LSB/MSB pair
		}
	default:
		// This case should not be reached if accessMode is always one of the valid ones.
		// fmt.Printf("PIT: Invalid access mode 0x%X during counter write\n", ch.accessMode)
		return
	}

	// If write sequence is complete for the current access mode, load the counter.
	// For LSB-only or MSB-only, writeState stays 0. For LSB/MSB, it becomes 0 after MSB.
	if ch.writeState == 0 {
		ch.count = ch.reloadValue
		ch.nullCount = false // Count has been loaded
		ch.lastClockTime = time.Now() // Reset timer reference
		// fmt.Printf("PIT Counter Loaded: ReloadValue=0x%04X (Mode %d)\n", ch.reloadValue, ch.mode>>1)

		// Mode-specific actions on load:
		switch ch.mode {
		case PIT_CMD_MODE0: // Interrupt on terminal count
			ch.output = false // Output goes low, then high on terminal count
		case PIT_CMD_MODE1: // Hardware retriggerable one-shot
			ch.output = false // Output goes low, then high after timeout. Retrigger sets it low again.
		case PIT_CMD_MODE2: // Rate generator
			ch.output = true  // Output high, goes low for one clock pulse on terminal count, then high again.
		case PIT_CMD_MODE3: // Square wave
			ch.output = true  // Output high. If count is even, it's high for count/2 ticks. If odd, (count+1)/2 ticks.
		case PIT_CMD_MODE4: // Software triggered strobe
			ch.output = true  // Output high, goes low for one clock pulse on terminal count.
		case PIT_CMD_MODE5: // Hardware triggered strobe
			ch.output = true  // Output high, goes low for one clock pulse on terminal count (after gate trigger).
		}
		// TODO: Signal PIC if mode change or load affects interrupt state (e.g. for channel 0)
		// This basic simulation doesn't run the counter actively yet.
	}
}

// handleCounterRead processes a read from a PIT counter data port.
func (ch *PITChannel) handleCounterRead() byte {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	var val byte

	if ch.countLatched { // If count was latched by a latch command or read-back
		// Read from latchedValue
		switch ch.accessMode {
		case PIT_CMD_RW_LSB_ONLY:
			val = byte(ch.latchedValue & 0xFF)
			ch.countLatched = false // Latch is consumed after full read sequence
		case PIT_CMD_RW_MSB_ONLY:
			val = byte((ch.latchedValue >> 8) & 0xFF)
			ch.countLatched = false
		case PIT_CMD_RW_LSB_MSB:
			if ch.readState == 0 { // Read LSB
				val = byte(ch.latchedValue & 0xFF)
				ch.readState = 1
			} else { // Read MSB
				val = byte((ch.latchedValue >> 8) & 0xFF)
				ch.readState = 0
				ch.countLatched = false // Latch consumed after MSB read
			}
		default:
			// Should not happen
			val = 0
		}
		// fmt.Printf("PIT Counter Read (Latched): Val=0x%02X, AccessMode=0x%X, ReadState=%d, LatchedVal=0x%04X\n",
		//	val, ch.accessMode>>4, ch.readState, ch.latchedValue)
		return val
	}

	// If status was latched by read-back, and this read is for status (not count)
	// This part of logic is tricky: read-back status is read via counter port if status was latched.
	// A common implementation detail: if status is latched for a channel, the *next* read
	// from that channel's data port returns status, not count.
	// This is typically only if accessMode is NOT LATCH_COUNT.
	// For simplicity, let's assume standard counter reads if not explicitly latched for count.
	// A more accurate model would check ch.statusLatched here.

	// Read current counter value on-the-fly
	currentCount := ch.readCurrentCount()
	switch ch.accessMode {
	case PIT_CMD_RW_LSB_ONLY:
		val = byte(currentCount & 0xFF)
	case PIT_CMD_RW_MSB_ONLY:
		val = byte((currentCount >> 8) & 0xFF)
	case PIT_CMD_RW_LSB_MSB:
		if ch.readState == 0 { // Read LSB
			val = byte(currentCount & 0xFF)
			ch.readState = 1
		} else { // Read MSB
			val = byte((currentCount >> 8) & 0xFF)
			ch.readState = 0
		}
	default:
		val = 0
	}
	// fmt.Printf("PIT Counter Read (Dynamic): Val=0x%02X, AccessMode=0x%X, ReadState=%d, CurrentCount=0x%04X\n",
	//	val, ch.accessMode>>4, ch.readState, currentCount)
	return val
}

// readCurrentCount simulates reading the current value of the counter.
// For now, this is a placeholder as we are not actively decrementing it.
// In a running simulation, this would calculate the decremented value based on time.
func (ch *PITChannel) readCurrentCount() uint16 {
	// Basic simulation: If a reload value is set, assume it's counting down.
	// This doesn't simulate actual decrementing over time yet.
	// For Mode 3 (Square Wave), the value read can be complex.
	// For now, just return the last loaded value or current count.
	// A more advanced simulation would estimate current count based on pitFrequency and time elapsed.
	if ch.nullCount {
		return 0 // Or some other default if count is "null"
	}

	// Simplistic: return the value as if it hasn't changed much since last load/latch.
	// A real simulation would calculate elapsed ticks and decrement.
	// For example:
	//  elapsed := time.Since(ch.lastClockTime)
	//  ticks := uint16(elapsed.Nanoseconds() * pitFrequency / 1e9) // Potential for large numbers
	//  if ticks >= ch.count { return 0 } else { return ch.count - ticks }
	// This is very simplified and needs care for different modes and frequencies.
	// For now, we'll return the stored count, assuming it's read "quickly".
	return ch.count
}

// setGate updates the gate status for a channel.
func (ch *PITChannel) setGate(gateState bool) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	// Gate transitions can affect counter operation in some modes (e.g., Mode 1, 5)
	// and can start/stop counting in others (e.g. Mode 0, 2, 3, 4 if not already running)
	if ch.gate != gateState {
		ch.gate = gateState
		// fmt.Printf("PIT Channel: Gate set to %v\n", gateState)
		// TODO: Mode-specific gate logic (e.g., trigger one-shot, start/stop counter)
		// For example, in Mode 2 or 3, a low-to-high gate transition might reload the counter.
	}
}

// Tick is a placeholder for advancing PIT state.
// In a real emulator, this might be called periodically or integrated with vCPU execution loop.
// This function would handle counter decrementing, mode logic, and interrupt generation.
func (pit *PITDevice) Tick() {
	// For each channel:
	//   Decrement counter if gate is active and mode requires counting.
	//   Handle terminal count (reload, change output, generate interrupt).
	//   This needs to be synchronized with guest execution speed.
	//   Channel 0 is typically connected to IRQ0 for timer interrupts.

	// Example for channel 0, mode 2 (Rate Generator) or 3 (Square Wave)
	// ch0 := &pit.channels[0]
	// ch0.mu.Lock()
	// defer ch0.mu.Unlock()
	// if ch0.gate && ch0.reloadValue > 0 && !ch0.nullCount {
	//    now := time.Now()
	//    elapsed := now.Sub(ch0.lastClockTime)
	//    ticksToDecrement := uint64(elapsed.Nanoseconds()) * uint64(pitFrequency) / uint64(ch0.reloadValue) / 1e9 // This formula is specific to how many full cycles pass
	//
	//    if ticksToDecrement > 0 {
	//       ch0.lastClockTime = now // Or add the accounted for duration
	//       // Simplified decrement, real PITs count clock pulses
	//       if uint64(ch0.count) > ticksToDecrement {
	//          ch0.count -= uint16(ticksToDecrement)
	//       } else {
	//          // Terminal count reached
	//          ch0.count = ch0.reloadValue // Reload
	//          // Handle output toggle for Mode 3, pulse for Mode 2
	//          // if pit.pic != nil && ch0.mode == PIT_CMD_MODE2 || ch0.mode == PIT_CMD_MODE3 { // Simplified condition
	//          //    pit.pic.RaiseIRQ(pit.GetIRQLine())
	//          // }
	//          fmt.Printf("PIT Ch0: Terminal count, reloaded to 0x%04X (simulated)\n", ch0.reloadValue)
	//       }
	//    }
	// }

	// Simulate a timer interrupt for channel 0 if it's configured and time for it.
	// This is a very basic placeholder for actual timer logic.
	// In a real system, this would be driven by the PIT's internal clock and counter.
	// For now, let's imagine we can call a method to simulate an IRQ0 pulse.
	// pit.SimulateIRQ0() // This would be called based on timing logic.
	// This is a very rough sketch. Accurate PIT simulation is complex.
	// For now, we focus on register I/O. Actual timed events are deferred for full implementation.
}

// SimulateIRQ0 is a placeholder to manually trigger IRQ0 from PIT.
// This would be called by the PIT's internal timing logic when channel 0 reaches terminal count.
func (pit *PITDevice) SimulateIRQ0() {
	if pit.pic != nil {
		// fmt.Println("PIT: Simulating IRQ0")
		pit.pic.RaiseIRQ(pit.GetIRQLine())
		// In a real scenario, the IRQ might be lowered after acknowledgment or by the PIC logic itself.
		// For edge-triggered, the PIC latches it.
	}
}

// GetIRQLine returns the IRQ line this device would use (typically IRQ0 for channel 0).
func (pit *PITDevice) GetIRQLine() uint8 {
	return 0 // PIT Channel 0 is usually IRQ0
}

// TODO: A more complete PIT would have an internal ticker or a way for the main VM loop
// to advance its state and check for timer expirations.
// For instance, the vCPU loop could periodically call a method on PITDevice
// that updates internal counters based on elapsed time and triggers interrupts.
