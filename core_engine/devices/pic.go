package devices

import (
	"fmt"
	"log"
	"sync"
)

// InterruptRaiser defines the interface for devices that can raise IRQs on the PIC.
type InterruptRaiser interface {
	RaiseIRQ(irq uint8)
	LowerIRQ(irq uint8) // Not strictly needed for all PIC models but good for completeness
}

// PICDevice represents a single 8259A PIC chip (master or slave).
type PICDevice struct {
	sync.Mutex
	CommandPort uint16
	DataPort    uint16

	irr uint8 // Interrupt Request Register: pins that have pending requests
	isr uint8 // In-Service Register: interrupts currently being serviced
	imr uint8 // Interrupt Mask Register: masks out interrupts

	icwStep int     // Current step in ICW initialization sequence
	icw     [4]byte // ICW values storage

	readISR bool // Next read from command port should return ISR, else IRR
	autoEOI bool // Automatic End Of Interrupt

	// For Master PIC
	Slave *PICDevice // Link to slave PIC if this is a master

	// For Slave PIC
	irqOffset uint8 // IRQ base for this PIC (usually 8 for slave)
}

// NewPICDevice creates a new PIC device.
// isMaster defines if it's the master (PIC1) or slave (PIC2).
// slave is a pointer to the slave PIC, only used if isMaster is true.
func NewPICDevice(cmdPort, dataPort uint16, isMaster bool, slave *PICDevice) *PICDevice {
	pic := &PICDevice{
		CommandPort: cmdPort,
		DataPort:    dataPort,
		imr:         0xFF, // All interrupts masked initially
		Slave:       slave,
	}
	if !isMaster {
		// Slave PIC typically has IRQ offset 8
		pic.irqOffset = 8
	}
	return pic
}

// HandleIO handles I/O operations for the PIC device.
func (pic *PICDevice) HandleIO(port uint16, data []byte, isWrite bool) ([]byte, error) {
	pic.Lock()
	defer pic.Unlock()

	if len(data) > 1 {
		return nil, fmt.Errorf("pic: data size %d > 1 not supported on port 0x%X", len(data), port)
	}

	// log.Printf("PIC IO: Port 0x%X, Write: %v, Data: 0x%X (ICW Step: %d)", port, isWrite, data, pic.icwStep)

	switch port {
	case pic.CommandPort:
		if isWrite {
			pic.writeCommand(data[0])
		} else {
			// Reading command port
			log.Printf("PIC.HandleIO: Reading CMD port. readISR_flag before decision: %v, ISR_val: 0x%02X, IRR_val: 0x%02X", pic.readISR, pic.isr, pic.irr)
			if pic.readISR {
				currentIsrVal := pic.isr
				pic.readISR = false // Standard behavior: reading ISR resets the "read ISR next" state for the next read.
				log.Printf("PIC.HandleIO: Returning ISR (0x%02X). readISR_flag set to false for next op.", currentIsrVal)
				return []byte{currentIsrVal}, nil
			}
			log.Printf("PIC.HandleIO: Returning IRR (0x%02X).", pic.irr)
			return []byte{pic.irr}, nil
		}
	case pic.DataPort:
		if isWrite {
			pic.writeData(data[0])
		} else {
			// Reading data port returns IMR
			// log.Printf("PIC (0x%X) Read IMR: 0x%02X", pic.DataPort, pic.imr)
			return []byte{pic.imr}, nil
		}
	default:
		return nil, fmt.Errorf("pic: unhandled port 0x%X", port)
	}
	return nil, nil
}

func (pic *PICDevice) writeCommand(val byte) {
	log.Printf("PIC.writeCommand (Port 0x%X) received value: 0x%02X", pic.CommandPort, val)

	// log.Printf("PIC (0x%X) CMD Write: 0x%02X (ICW Step: %d)", pic.CommandPort, val, pic.icwStep)
	if val&ICW1_INIT != 0 { // ICW1
		pic.icwStep = 1
		pic.icw[0] = val
		pic.imr = 0x00 // Clear IMR as per some docs, though often set by OS later
		pic.isr = 0x00
		pic.irr = 0x00
		pic.autoEOI = false
		// log.Printf("PIC (0x%X) ICW1 received: 0x%02X. ICW4 needed: %v", pic.CommandPort, val, (val&ICW1_ICW4) != 0)
		return
	}

	// OCWs
	// Define conditions based on 'val'
	// OCW3 Read/Mask: Bit 3=1, Bit 2=0 (e.g. 0x0A for Read IRR, 0x0B for Read ISR)
	is_ocw3_read_mask := ((val & 0x08) != 0) && ((val & 0x04) == 0)

	// OCW3 Poll: Bit 3=1, Bit 2=1 (e.g. 0x0C)
	is_ocw3_poll := ((val & 0x08) != 0) && ((val & 0x04) != 0)

	is_ocw2 := (val & 0x80) == 0 // Bit 7=0 (characteristic of OCW2 if not an OCW3 form)

	// Log evaluation of conditions for debugging
	// log.Printf("PIC.writeCommand (Port 0x%X, val 0x%02X): is_ocw3_read_mask=%v; is_ocw3_poll=%v; is_ocw2_if_not_ocw3=%v",
	//	pic.CommandPort, val, is_ocw3_read_mask, is_ocw3_poll, is_ocw2)

	if is_ocw3_read_mask {
		log.Printf("PIC.writeCommand (Port 0x%X, val 0x%02X): Processing as OCW3 Read/Mask.", pic.CommandPort, val)
		if (val & 0x02) != 0 { // RR bit (Read Register) is set
			if (val & 0x01) == 0 { // RIS bit (0 for IRR)
				pic.readISR = false
				log.Printf("PIC.writeCommand (Port 0x%X, val 0x%02X): OCW3 Read IRR. Set pic.readISR to FALSE.", pic.CommandPort, val)
			} else { // RIS bit (1 for ISR)
				pic.readISR = true
				log.Printf("PIC.writeCommand (Port 0x%X, val 0x%02X): OCW3 Read ISR. Set pic.readISR to TRUE.", pic.CommandPort, val)
			}
		} else {
			// Special Mask Mode logic
			log.Printf("PIC.writeCommand (Port 0x%X, val 0x%02X): OCW3 Special Mask Mode (Not Implemented).", pic.CommandPort, val)
		}
	} else if is_ocw3_poll {
		log.Printf("PIC.writeCommand (Port 0x%X, val 0x%02X): Processing as OCW3 Poll (Not Implemented).", pic.CommandPort, val)
		// Poll command logic
	} else if is_ocw2 { // OCW2 (and not an OCW3 type already handled)
		log.Printf("PIC.writeCommand (Port 0x%X, val 0x%02X): Processing as OCW2.", pic.CommandPort, val)
		// OCW2 commands typically have bits 4 and 3 as 00 (val&0x18 == 0), and bit 7 as 0.
		// EOI commands: Non-specific (0x20), Specific (0x60 | irq)
		if val == OCW2_EOI { // Non-specific EOI (0x20 : 00100000)
			// Cleared variable was removed as it was unused.
			for i := 7; i >= 0; i-- {
				if pic.isr&(1<<uint(i)) != 0 {
					pic.isr &^= (1 << uint(i))
					break
				}
			}
		} else if (val & 0xE0) == OCW2_SPECIFIC_EOI { // Specific EOI check (011xxxxx)
                                                     // OCW2_SPECIFIC_EOI is 0x60
                                                     // This condition checks if top 3 bits are 011
			irqToEOI := val & 0x07
			pic.isr &^= (1 << uint(irqToEOI))
		} else {
			// Other OCW2 types not fully handled e.g. rotate commands.
			log.Printf("PIC (0x%X) Unhandled OCW2 variant: 0x%02X", pic.CommandPort, val)
		}
	} else {
		log.Printf("PIC (0x%X) Unhandled command (not ICW1, OCW3, or OCW2): 0x%02X", pic.CommandPort, val)
	}
}

func (pic *PICDevice) writeData(val byte) {
	// log.Printf("PIC (0x%X) Data Write: 0x%02X (ICW Step: %d)", pic.DataPort, val, pic.icwStep)
	switch pic.icwStep {
	case 0: // Not in ICW sequence, so this is an IMR write
		pic.imr = val
		// log.Printf("PIC (0x%X) IMR set to: 0x%02X", pic.DataPort, pic.imr)
	case 1: // ICW2: Interrupt vector offset
		pic.icw[1] = val
		if pic.icw[0]&ICW1_SINGLE != 0 { // Single PIC mode
			// log.Printf("PIC (0x%X) ICW2 (single): 0x%02X. Vector offset: 0x%02X", pic.DataPort, val, val)
			if pic.icw[0]&ICW1_ICW4 != 0 {
				pic.icwStep = 3 // Skip ICW3, go to ICW4
			} else {
				pic.icwStep = 0 // End of ICW sequence
				// log.Printf("PIC (0x%X) Initialization complete (no ICW4). Vector offset: 0x%02X", pic.DataPort, pic.icw[1])
			}
		} else { // Cascade mode
			// log.Printf("PIC (0x%X) ICW2 (cascade): 0x%02X. Vector offset: 0x%02X", pic.DataPort, val, val)
			pic.icwStep = 2 // Go to ICW3
		}
	case 2: // ICW3: Master/Slave configuration
		pic.icw[2] = val
		// log.Printf("PIC (0x%X) ICW3: 0x%02X (Master: slave bitmap / Slave: slave ID)", pic.DataPort, val)
		if pic.icw[0]&ICW1_ICW4 != 0 {
			pic.icwStep = 3 // Go to ICW4
		} else {
			pic.icwStep = 0 // End of ICW sequence
			// log.Printf("PIC (0x%X) Initialization complete (no ICW4). Vector offset: 0x%02X, ICW3: 0x%02X", pic.DataPort, pic.icw[1], pic.icw[2])
		}
	case 3: // ICW4: Additional mode configuration
		pic.icw[3] = val
		// log.Printf("PIC (0x%X) ICW4: 0x%02X", pic.DataPort, val)
		if val&ICW4_8086 != 0 {
			// log.Printf("PIC (0x%X) Mode: 8086/88", pic.DataPort)
		}
		if val&ICW4_AUTO != 0 {
			pic.autoEOI = true
			// log.Printf("PIC (0x%X) Auto EOI enabled", pic.DataPort)
		}
		pic.icwStep = 0 // End of ICW sequence
		// log.Printf("PIC (0x%X) Initialization complete. Vector offset: 0x%02X, ICW3: 0x%02X, ICW4: 0x%02X", pic.DataPort, pic.icw[1], pic.icw[2], pic.icw[3])
	}
}

// RaiseIRQ signals an interrupt request on the given IRQ line.
func (pic *PICDevice) RaiseIRQ(irq uint8) {
	if irq > 7 {
		log.Printf("PIC (0x%X) RaiseIRQ: IRQ %d out of range for a single PIC", pic.CommandPort, irq)
		return
	}
	pic.Lock()
	defer pic.Unlock()
	// log.Printf("PIC (0x%X) RaiseIRQ: %d. IRR before: 0x%02X", pic.CommandPort, irq, pic.irr)
	pic.irr |= (1 << irq)
	// log.Printf("PIC (0x%X) RaiseIRQ: %d. IRR after: 0x%02X, IMR: 0x%02X", pic.CommandPort, irq, pic.irr, pic.imr)
}

// LowerIRQ is less commonly explicitly managed this way for simple PICs,
// as edge-triggered interrupts are latched until acknowledged.
// For level-triggered, it would be more relevant.
func (pic *PICDevice) LowerIRQ(irq uint8) {
	if irq > 7 {
		// log.Printf("PIC (0x%X) LowerIRQ: IRQ %d out of range", pic.CommandPort, irq)
		return
	}
	pic.Lock()
	defer pic.Unlock()
	// For edge-triggered, IRR bit is cleared when ISR is set and EOI'd.
	// For level-triggered, this would clear the request if the device is no longer asserting.
	// We are mostly simulating edge-triggered.
	// pic.irr &= ^(1 << irq)
	// log.Printf("PIC (0x%X) LowerIRQ: %d (currently a NO-OP for edge simulation). IRR: 0x%02X", pic.CommandPort, irq, pic.irr)
}

// HasPendingInterrupt checks if there's an unmasked interrupt pending.
// Returns true if an interrupt is pending, false otherwise.
func (pic *PICDevice) HasPendingInterrupt() bool {
	pic.Lock()
	defer pic.Unlock()
	return (pic.irr &^ pic.imr) != 0
}

// GetInterruptVector returns the highest priority pending interrupt vector.
// It sets the corresponding ISR bit and clears the IRR bit.
// Returns (vector, true) if an interrupt is available, or (0, false) otherwise.
func (pic *PICDevice) GetInterruptVector() (uint8, bool) {
	pic.Lock()
	defer pic.Unlock()

	pendingAndUnmasked := pic.irr &^ pic.imr
	if pendingAndUnmasked == 0 {
		return 0, false // No pending unmasked interrupt
	}

	var chosenIRQ uint8 = 255 // Invalid IRQ initial value

	for irqLoopVar := 0; irqLoopVar < 8; irqLoopVar++ { // Check IRQ 0-7 in order of priority
		if pendingAndUnmasked&(1<<uint(irqLoopVar)) != 0 {
			chosenIRQ = uint8(irqLoopVar) // Store the chosen IRQ
			break                         // Exit loop once highest priority IRQ is found
		}
	}

	if chosenIRQ > 7 { // Should not happen if pendingAndUnmasked was non-zero
		log.Printf("PIC (0x%X) GetInterruptVector: No IRQ found by loop despite pendingAndUnmasked=0x%02X. IRR=0x%02X, ISR=0x%02X, IMR=0x%02X",
			pic.CommandPort, pendingAndUnmasked, pic.irr, pic.isr, pic.imr)
		return 0, false
	}

	// Now use chosenIRQ consistently
	pic.irr &^= (1 << chosenIRQ)
	pic.isr |= (1 << chosenIRQ)

	vector := pic.icw[1] + chosenIRQ // Base vector + IRQ number

	// log.Printf("PIC (0x%X) GetInterruptVector: Servicing IRQ %d. IRR becomes 0x%02X, ISR becomes 0x%02X. Vector 0x%02X",
	//	pic.CommandPort, chosenIRQ, pic.irr, pic.isr, vector)


	if pic.autoEOI {
		pic.isr &^= (1 << chosenIRQ) // Auto EOI clears ISR bit immediately
		// log.Printf("PIC (0x%X) Auto EOI for IRQ %d. ISR now 0x%02X", pic.CommandPort, chosenIRQ, pic.isr)
	}
	return vector, true
}

// PICController manages a pair of PICs (master and slave).
type PICController struct {
	Master *PICDevice
	Slave  *PICDevice
}

// NewPICController creates and initializes a master/slave PIC setup.
func NewPICController() *PICController {
	slave := NewPICDevice(PIC2_COMMAND_PORT, PIC2_DATA_PORT, false, nil)
	master := NewPICDevice(PIC1_COMMAND_PORT, PIC1_DATA_PORT, true, slave)
	slave.irqOffset = 8 // Standard slave PIC vector starts after master's 8
	return &PICController{
		Master: master,
		Slave:  slave,
	}
}

// HandleIO routes I/O to the correct PIC (master or slave).
func (pc *PICController) HandleIO(port uint16, data []byte, isWrite bool) ([]byte, error) {
	switch port {
	case PIC1_COMMAND_PORT, PIC1_DATA_PORT:
		return pc.Master.HandleIO(port, data, isWrite)
	case PIC2_COMMAND_PORT, PIC2_DATA_PORT:
		return pc.Slave.HandleIO(port, data, isWrite)
	default:
		// This check should ideally be done by the VM before calling this device.
		// If this device is registered only for its specific ports, this case won't be hit.
		return nil, fmt.Errorf("piccontroller: unhandled port 0x%X", port)
	}
}

// RaiseIRQ allows external devices to raise an interrupt.
// IRQs 0-7 are for master, 8-15 for slave.
func (pc *PICController) RaiseIRQ(irq uint8) {
	// log.Printf("PICController: RaiseIRQ %d", irq)
	if irq <= 7 { // Master PIC
		pc.Master.RaiseIRQ(irq)
	} else if irq <= 15 { // Slave PIC
		// Slave handles IRQs 0-7 internally, which correspond to system IRQs 8-15.
		pc.Slave.RaiseIRQ(irq - 8)
		// Also signal the master on its cascade IRQ line (typically IRQ 2)
		pc.Master.RaiseIRQ(IRQ_CASCADE) // IRQ_CASCADE is usually 2
	} else {
		log.Printf("PICController: IRQ %d out of range (0-15)", irq)
	}
}

func (pc *PICController) LowerIRQ(irq uint8) {
	// log.Printf("PICController: LowerIRQ %d", irq)
	if irq <= 7 {
		pc.Master.LowerIRQ(irq)
	} else if irq <= 15 {
		pc.Slave.LowerIRQ(irq - 8)
		// If all slave IRQs are clear, we might lower IRQ_CASCADE on master.
		// This logic can be complex depending on PIC configuration (level vs edge).
		// For now, simple pass-through.
		if !pc.Slave.HasPendingInterrupt() { // A simple check
			pc.Master.LowerIRQ(IRQ_CASCADE)
		}
	} else {
		log.Printf("PICController: IRQ %d out of range for LowerIRQ (0-15)", irq)
	}
}

// GetInterruptVector checks both PICs and returns the highest priority interrupt vector.
// It handles cascading from slave to master.
func (pc *PICController) GetInterruptVector() (uint8, bool) {
	// Check master PIC first (IRQs 0-7, excluding cascade line if slave has interrupt)
	vector, ok := pc.Master.GetInterruptVector()
	if ok {
		// If the interrupt is from the slave (via cascade IRQ on master),
		// we need to get the actual vector from the slave.
		if vector == pc.Master.icw[1]+IRQ_CASCADE { // Check if it's the cascade IRQ
			slaveVector, slaveOk := pc.Slave.GetInterruptVector()
			if slaveOk {
				// log.Printf("PICController: Interrupt from Slave (IRQ %d on master), Slave vector 0x%02X", IRQ_CASCADE, slaveVector)
				return slaveVector, true
			}
			// If slave had no interrupt, something is wrong, or cascade was spurious.
			// For now, we'll assume if cascade triggered, slave must provide vector.
			// If not, acknowledge the cascade on master and return no interrupt.
			// This part might need refinement based on real PIC behavior.
			// log.Printf("PICController: Cascade IRQ from master, but slave had no vector. Master ISR for cascade was 0x%02X", pc.Master.isr)
			// If autoEOI is off for master, the ISR bit for cascade (IRQ2) would still be set.
			// It needs an EOI. This simplified GetInterruptVector doesn't handle the EOI for master's cascade IRQ separately.
			// For now, we assume if slaveOk is false, then the cascade was spurious or already handled.
			return 0, false
		}
		// log.Printf("PICController: Interrupt from Master, vector 0x%02X", vector)
		return vector, true // Genuine master interrupt
	}

	// If master had no interrupt, it's possible the cascade line was asserted
	// but already processed by master.GetInterruptVector(), and now we check slave directly.
	// This logic is tricky. The standard way is:
	// 1. Master PIC signals CPU.
	// 2. CPU acks, reads vector from Master.
	// 3. If vector is cascade IRQ, CPU then reads vector from Slave.

	// The current structure of GetInterruptVector for a single PIC already sets ISR
	// and clears IRR. If master's GetInterruptVector() returned the cascade IRQ,
	// it means it was the highest priority on master. Then the slave's vector is fetched.

	// This path (master had no interrupt, or its interrupt was not cascade)
	// implies we should not be checking slave independently unless master directed us.
	// However, if master's GetInterruptVector() handled a non-cascade IRQ,
	// the slave wouldn't be checked.

	// The logic should be:
	// 1. Check master. If it has an interrupt:
	//    a. If it's the cascade line, get vector from slave. If slave has one, return it.
	//       If slave doesn't (spurious), clear master's cascade ISR bit and return no interrupt.
	//    b. If it's not cascade, return master's vector.
	// 2. If master has no interrupt, then there's no interrupt for the CPU.

	// The current combined GetInterruptVector in PICDevice already handles IRR->ISR transition.
	// So, the logic above inside the `if ok` block for master is mostly correct.
	// If master.GetInterruptVector() returned `false`, it means no *unmasked* irq was pending on master,
	// or one was pending but it was the cascade line and slave had no vector.

	return 0, false // No interrupt from master (or handled cascade that was empty)
}

// HasPendingInterrupt checks if any PIC has a pending interrupt.
func (pc *PICController) HasPendingInterrupt() bool {
	// An interrupt is pending if the master has one, OR if the slave has one AND
	// the cascade line on the master (IRQ_CASCADE) is not masked on the master.
	// Simpler: check if master would signal an interrupt.
	// Master signals if (master.irr & ^master.imr) != 0.
	// If the only thing in master.irr is IRQ_CASCADE, then we also need slave to have an interrupt.

	masterPending := pc.Master.irr &^ pc.Master.imr
	if masterPending == 0 {
		return false // Master has nothing, so no CPU interrupt
	}

	// If master has something pending, is it ONLY the cascade line?
	if masterPending == (1 << IRQ_CASCADE) {
		// Only cascade line is pending on master. Check if slave has anything.
		slavePending := pc.Slave.irr &^ pc.Slave.imr
		return slavePending != 0
	}

	return true // Master has a non-cascade interrupt pending, or multiple including cascade.
}

// Ports returns the I/O ports managed by this device.
func (pc *PICController) Ports() []uint16 {
	return []uint16{
		PIC1_COMMAND_PORT, PIC1_DATA_PORT,
		PIC2_COMMAND_PORT, PIC2_DATA_PORT,
	}
}

// DebugState returns a string representation of the PICs' state.
func (pc *PICController) DebugState() string {
	mStr := fmt.Sprintf("Master (0x%X,0x%X): IRR:%02x ISR:%02x IMR:%02x ICWs:[%02x,%02x,%02x,%02x] autoEOI:%t, nextReadISR:%t\n",
		pc.Master.CommandPort, pc.Master.DataPort, pc.Master.irr, pc.Master.isr, pc.Master.imr,
		pc.Master.icw[0], pc.Master.icw[1], pc.Master.icw[2], pc.Master.icw[3], pc.Master.autoEOI, pc.Master.readISR)
	sStr := fmt.Sprintf("Slave  (0x%X,0x%X): IRR:%02x ISR:%02x IMR:%02x ICWs:[%02x,%02x,%02x,%02x] autoEOI:%t, nextReadISR:%t, offset:%d",
		pc.Slave.CommandPort, pc.Slave.DataPort, pc.Slave.irr, pc.Slave.isr, pc.Slave.imr,
		pc.Slave.icw[0], pc.Slave.icw[1], pc.Slave.icw[2], pc.Slave.icw[3], pc.Slave.autoEOI, pc.Slave.readISR, pc.Slave.irqOffset)
	return mStr + sStr
}
