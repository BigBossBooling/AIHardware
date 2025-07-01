package devices

import (
	"fmt"
	"log"
	"net" // For MAC address parsing/representation
	"sync"
	"v-architect/core_engine/network" // For TapDevice
)

const (
	ne2000RamSize   = NE2000_MEM_SIZE_16K // 16KB on-card RAM
	ne2000PageSize  = NE2000_MEM_PAGE_SIZE
	ne2000NumPages  = ne2000RamSize / ne2000PageSize
	ne2000TxBufSize = 6 // Number of pages for TX buffer (6 * 256 = 1536 bytes)
)

// NE2000Device represents an NE2000 compatible network interface card.
type NE2000Device struct {
	mu sync.Mutex

	ioBase uint16
	tap    *network.TapDevice
	irq    InterruptRaiser
	irqNum uint8

	macAddress [6]byte
	ram        [ne2000RamSize]byte

	// DP8390 Registers (values depend on selected page in CR)
	cr     byte // Command Register
	isr    byte // Interrupt Status Register
	imr    byte // Interrupt Mask Register
	dcr    byte // Data Configuration Register
	tcr    byte // Transmit Configuration Register
	rcr    byte // Receive Configuration Register
	tsr    byte // Transmit Status Register
	rsr    byte // Receive Status Register (last packet's status)
	pstart byte // Page Start Register (for RX ring)
	pstop  byte // Page Stop Register (for RX ring)
	bnry   byte // Boundary Pointer (for RX ring)
	tpsr   byte // Transmit Page Start Address (for TX buffer)
	tbcr0  byte // Transmit Byte Count LSB
	tbcr1  byte // Transmit Byte Count MSB
	curr   byte // Current Page Register (for RX ring management by NIC)

	// Remote DMA registers (less critical for basic PIO)
	rsar0 byte // Remote Start Address LSB
	rsar1 byte // Remote Start Address MSB
	rbcr0 byte // Remote Byte Count LSB
	rbcr1 byte // Remote Byte Count MSB

	// Counters (read-only, cleared on read by some models)
	cntr0 byte // Frame Alignment Errors
	cntr1 byte // CRC Errors
	cntr2 byte // Missed Packets

	// Internal state
	currentPageSelect byte // Value of CR_PS1:CR_PS0 (0, 1, or 2 typically)
	rxRingStartPage   byte
	rxRingEndPage     byte
	nextPacketPtr     byte // Pointer to next unread packet in RX ring

	// Packet buffers (conceptual, data is in dev.ram)
	// txBuffer []byte (points into dev.ram based on tpsr)
	// rxBuffer *list.List // or a circular buffer structure for received packets
}

// NewNE2000Device creates and initializes a new NE2000 device.
func NewNE2000Device(ioBaseAddr uint16, tapDevice *network.TapDevice, macAddrString string, irqHandler InterruptRaiser, nicIrqNum uint8) (*NE2000Device, error) {
	if tapDevice == nil {
		return nil, fmt.Errorf("NE2000Device requires a TapDevice")
	}

	hwAddr, err := net.ParseMAC(macAddrString)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address string '%s': %w", macAddrString, err)
	}
	if len(hwAddr) != 6 {
		return nil, fmt.Errorf("parsed MAC address '%s' is not 6 bytes long", macAddrString)
	}

	dev := &NE2000Device{
		ioBase:          ioBaseAddr,
		tap:             tapDevice,
		irq:             irqHandler,
		irqNum:          nicIrqNum,
		// Registers are initialized to power-on defaults or specific values
		cr:     CR_STP | CR_RD2, // Stopped, Page 0 selected, DMA complete
		isr:    ISR_RST,          // Reset status initially set
		imr:    0x00,             // All interrupts masked
		dcr:    DCR_LS | DCR_FT0, // Byte-wide DMA, normal ops, 8-byte FIFO thresh (or DCR_WTS for word)
		tcr:    TCR_LB0,          // Normal operation (loopback bits = 01 internal loopback for DP8390) -> Should be 0 for normal.
		rcr:    0x00,             // Promiscuous off, accept only own MAC + broadcast initially
		pstart: NE2000_DEFAULT_RX_START_PAGE, // Example: Page 0x46 for RX start
		pstop:  NE2000_DEFAULT_RX_STOP_PAGE,  // Example: Page 0x60 for RX stop (for 8K RAM card)
		bnry:   NE2000_DEFAULT_RX_START_PAGE, // Boundary pointer starts at PSTART
		tpsr:   NE2000_DEFAULT_TX_START_PAGE, // Example: Page 0x40 for TX start
		curr:   NE2000_DEFAULT_RX_START_PAGE + 1, // Current page for NIC's next RX write
		nextPacketPtr: NE2000_DEFAULT_RX_START_PAGE + 1, // For software to know where to read next packet
	}
	copy(dev.macAddress[:], hwAddr)

	// Initialize TCR to normal operation (no loopback)
	dev.tcr = 0x00

	// Initialize DCR for 16-bit card (Word Transfer Select) and standard FIFO threshold
	// Assuming 16-bit card, so WTS should be 1. FT default often 8 bytes.
	// BOS (Byte Order Select) is usually 0 for x86 (Intel byte order for DMA words).
	dev.dcr = DCR_WTS | DCR_FT0 | DCR_FT1 // Word transfer, 12-byte FIFO threshold (or 8-byte: DCR_WTS | DCR_FT1)
	                                     // For PIO, WTS might not be strictly needed for data port but good for consistency.

	// Copy MAC address to the "PROM" area (first 16 bytes of NIC RAM, byte-swapped for word access)
	// NE2000 drivers read this typically via PIO after reset.
	// PROM is 0x00-0x0F within the NIC's address space, but often a data port for PIO.
	// Here, we simulate it by putting it at the start of our RAM buffer.
	// The NE2000_ASIC_DATA port is used to read/write this RAM.
	// The MAC is bytes 0-5 of the PROM data.
	for i := 0; i < 6; i++ {
		dev.ram[NE2000_PROM_OFFSET+i] = dev.macAddress[i]
		// Some cards store it byte-swapped per word in the first few words of RAM for PIO access.
		// E.g., dev.ram[i*2] = mac[i*2+1], dev.ram[i*2+1] = mac[i*2] for the first 3 words.
		// For simplicity, direct mapping here. Driver usually knows how to read it.
		// A common way is that the first 16 bytes of the ASIC data port access map to the SA PROM.
		// We will put the MAC directly at the start of dev.ram, and then handle reads from NE2000_ASIC_DATA
		// to provide these bytes, possibly byte-swapped if read as words.
	}
	// Fill rest of PROM area with some pattern if needed, e.g. checksum or vendor ID.
	// For now, just MAC. The first 16 bytes are accessible via the Data port after reset
	// by some drivers.

	// Setup RX ring buffer parameters based on PSTART/PSTOP
	dev.rxRingStartPage = dev.pstart
	dev.rxRingEndPage = dev.pstop

	log.Printf("NE2000 Device initialized. IO Base: 0x%X, MAC: %s, IRQ: %d",
		dev.ioBase, macAddrString, dev.irqNum)
	log.Printf("  RAM: %dKB, TX Start Page: 0x%02X, RX Ring: 0x%02X-0x%02X",
		len(dev.ram)/1024, dev.tpsr, dev.rxRingStartPage, dev.rxRingEndPage)

	return dev, nil
}

// HandleIO processes PIO reads and writes to the NE2000 controller's registers.
func (dev *NE2000Device) HandleIO(port uint16, data []byte, isWrite bool) (uint8, error) {
	dev.mu.Lock()
	defer dev.mu.Unlock()

	offset := port - dev.ioBase
	var valOut uint8
	var err error

	// log.Printf("NE2000 HandleIO: port=0x%X (offset 0x%X), isWrite=%v, data=%#v, CR=0x%02X, PageSel=%d",
	//	port, offset, isWrite, data, dev.cr, dev.currentPageSelect)

	// Registers 0x00-0x0F are DP8390 registers (page sensitive)
	// Registers 0x10-0x1F are NE2000 ASIC specific (not page sensitive)
	if offset <= 0x0F { // DP8390 Registers
		if offset == NE2000_REG_CR { // Command Register (0x00)
			if isWrite {
				dev.writeCR(data[0])
			} else {
				valOut = dev.cr
			}
		} else { // Other Page-Sensitive Registers
			page := dev.cr >> 6 // Get page from CR_PS1:CR_PS0
			switch page {
			case 0: // Page 0 Registers
				valOut, err = dev.handlePage0IO(offset, data, isWrite)
			case 1: // Page 1 Registers
				valOut, err = dev.handlePage1IO(offset, data, isWrite)
			case 2: // Page 2 Registers (often some overlap or specific use)
				valOut, err = dev.handlePage2IO(offset, data, isWrite)
			default: // Page 3 is usually vendor-specific / diagnostic, treat as unhandled for now
				err = fmt.Errorf("ne2000: unhandled I/O to page %d, offset 0x%X", page, offset)
			}
		}
	} else if offset >= NE2000_ASIC_DATA && offset <= NE2000_ASIC_RESET { // ASIC Registers
		switch offset {
		case NE2000_ASIC_DATA: // 0x10 - Data Port (for PIO access to NIC RAM)
			// This port is used for reading/writing packet data or PROM data after setting up
			// remote DMA registers (RSAR0/1, RBCR0/1) and issuing a remote DMA command (CR_RD0/RD1).
			// For simplicity in PROM read, we'll assume it allows direct byte access based on RSAR.
			// A full PIO implementation would handle word access and auto-increment RSAR.
			if isWrite {
				// Write to NIC RAM via data port (part of Remote DMA Write or manual PIO)
				// dev.writeNicRam(dev.getRemoteDMAAddress(), data[0]) // Example
				// dev.incrementRemoteDMAAddress()
				log.Printf("NE2000: Write to ASIC Data Port (0x%X) - Not fully implemented for general write", offset)
			} else {
				// Read from NIC RAM via data port
				// This path is critical for reading PROM (MAC address) after reset.
				// Driver sets up RSAR to point to PROM (0x0000 in NIC RAM) and RBCR for 16/32 bytes.
				// Then reads words from this port.
				// We'll simplify to byte reads for now, incrementing RSAR.
				dmaAddr := (uint16(dev.rsar1) << 8) | uint16(dev.rsar0)
				if dmaAddr < ne2000RamSize {
					valOut = dev.ram[dmaAddr]
					// log.Printf("NE2000: Read from ASIC Data Port (0x%X), Addr: 0x%04X, Val: 0x%02X", offset, dmaAddr, valOut)
					dmaAddr++ // Auto-increment address for next read
					dev.rsar0 = byte(dmaAddr & 0xFF)
					dev.rsar1 = byte((dmaAddr >> 8) & 0xFF)
					// Decrement RBCR, handle DMA complete interrupt (later)
					// if dev.decrementRemoteDMACount() { /* signal RDC interrupt */ }
				} else {
					log.Printf("NE2000: Read from ASIC Data Port (0x%X) - Remote DMA Addr 0x%04X out of bounds", offset, dmaAddr)
					valOut = 0xFF // Error
				}
			}
		// 0x11 - 0x1E: Typically unused or vendor-specific on NE2000 clones for PIO.
		// Some cards might have specific functions here.
		case NE2000_ASIC_RESET: // 0x1F - Reset Port
			if !isWrite { // Read from Reset Port resets the NIC
				log.Println("NE2000: Reset triggered by read from RESET port.")
				dev.reset() // Call internal reset function
				valOut = 0 // Value read is often undefined or 0
			} else {
				// Writing to reset port is usually a NOP.
				log.Printf("NE2000: Write to ASIC Reset Port (0x%X) - No action", offset)
			}
		default:
			err = fmt.Errorf("ne2000: unhandled I/O to ASIC offset 0x%X", offset)
		}
	} else {
		err = fmt.Errorf("ne2000: access to unhandled port offset 0x%X", offset)
	}

	if err != nil {
		log.Printf("NE2000 HandleIO Error: %v (port 0x%X, offset 0x%X, write %v)", err, port, offset, isWrite)
	}
	return valOut, err
}

// writeCR handles writes to the Command Register (CR)
func (dev *NE2000Device) writeCR(value byte) {
	// log.Printf("NE2000: CR write: 0x%02X", value)
	oldPageSelect := dev.cr >> 6
	dev.cr = value
	dev.currentPageSelect = dev.cr >> 6

	if dev.currentPageSelect != oldPageSelect {
		// log.Printf("NE2000: Page selected: %d", dev.currentPageSelect)
	}

	if (value & CR_STP) != 0 { // Stop command
		dev.reset() // STP bit puts card in reset state
		dev.isr |= ISR_RST // Set reset status flag
		return // Most other bits are ignored if STP is set
	}
	if (value & CR_STA) != 0 { // Start command
		// log.Println("NE2000: Start command (CR_STA)")
		dev.isr &= (^ISR_RST & 0xFF) // Clear reset status flag
		// Initialize receiver, transmitter as per RCR, TCR if not already running.
		// This is where the card would actually start listening/transmitting.
	}

	// Handle Remote DMA commands (RD0, RD1, RD2)
	if (value & CR_RD2) != 0 { // DMA Complete / Send Packet
		if (value & CR_RD0) != 0 || (value & CR_RD1) != 0 { // Is it a DMA read or write?
			// This bit combination (RD2 with RD0 or RD1) means "Send Packet" for DMA write,
			// or implies completion for DMA read.
			// For PIO, RD2 is often just set with STP or STA.
			// A full DMA engine would use these. For PIO, we might just clear them.
			// log.Printf("NE2000: CR Remote DMA command: 0x%02X", value & (CR_RD0|CR_RD1|CR_RD2))
		}
	}

	// Handle Transmit Packet command (TXP)
	if (value & CR_TXP) != 0 {
		// log.Println("NE2000: Transmit Packet command (CR_TXP)")
		// dev.startTransmit() // To be implemented
		dev.cr &= (^CR_TXP & 0xFF) // TXP is self-clearing (or should be cleared by ISR handler)
	}
}

// handlePage0IO handles I/O for DP8390 Page 0 registers
func (dev *NE2000Device) handlePage0IO(offset uint16, data []byte, isWrite bool) (uint8, error) { // Changed offset to uint16
	var valOut uint8
	// log.Printf("NE2000 Page 0 IO: offset=0x%02X, isWrite=%v", offset, isWrite)
	switch uint8(offset) { // Switch on uint8 version of offset
	case NE2000_REG_CLDA0: // Also NE2000_REG_PSTART (0x01)
		if isWrite { // PSTART
			dev.pstart = data[0]; dev.rxRingStartPage = data[0]
		} else { // CLDA0
			valOut = 0 // Placeholder for CLDA0 read if needed
		}
	case NE2000_REG_CLDA1: // Also NE2000_REG_PSTOP (0x02)
		if isWrite { // PSTOP
			dev.pstop = data[0]; dev.rxRingEndPage = data[0]
		} else { // CLDA1
			valOut = 0 // Placeholder for CLDA1 read if needed
		}
	case NE2000_REG_BNRY: // 0x03 (R/W)
		if isWrite { dev.bnry = data[0]; dev.rxRingEndPage = data[0] /*BNRY write updates PSTOP effectively for RX ring*/ } else { valOut = dev.bnry }
	case NE2000_REG_TSR: // Also NE2000_REG_TPSR (0x04)
		if isWrite { // TPSR
			dev.tpsr = data[0]
		} else { // TSR
			valOut = dev.tsr
			// dev.tsr = 0 // TSR often clears on read, or specific bits do
		}
	case NE2000_REG_NCR: // Also NE2000_REG_TBCR0 (0x05)
		if isWrite { // TBCR0
			dev.tbcr0 = data[0]
		} else { // NCR
			valOut = 0 // Placeholder for collision counter
		}
	case NE2000_REG_FIFO: // Also NE2000_REG_TBCR1 (0x06)
		if isWrite { // TBCR1
			dev.tbcr1 = data[0]
		} else { // FIFO (diagnostic read)
			valOut = 0 // Placeholder
		}
	case NE2000_REG_ISR: // 0x07 (R/W)
		if isWrite { dev.isr &= ^data[0] } else { valOut = dev.isr } // Write 1 to clear bits
	case NE2000_REG_CRDA0: // Also NE2000_REG_RSAR0 (0x08)
		if isWrite { // RSAR0
			dev.rsar0 = data[0]
		} else { // CRDA0
			valOut = 0 // Placeholder for Current Remote DMA Addr read
		}
	case NE2000_REG_CRDA1: // Also NE2000_REG_RSAR1 (0x09)
		if isWrite { // RSAR1
			dev.rsar1 = data[0]
		} else { // CRDA1
			valOut = 0 // Placeholder
		}
	case 0x0A: // Reserved (R), NE2000_REG_RBCR0 (W)
		if isWrite { // RBCR0
			dev.rbcr0 = data[0]
		} else {
			valOut = 0 // Reserved on read
		}
	case 0x0B: // Reserved (R), NE2000_REG_RBCR1 (W)
		if isWrite { // RBCR1
			dev.rbcr1 = data[0]
		} else {
			valOut = 0 // Reserved on read
		}
	case NE2000_REG_RSR: // Also NE2000_REG_RCR (0x0C)
		if isWrite { // RCR
			dev.rcr = data[0]
		} else { // RSR
			valOut = dev.rsr
			// dev.rsr = 0 // RSR may clear on read
		}
	case NE2000_REG_CNTR0: // Also NE2000_REG_TCR (0x0D)
		if isWrite { // TCR
			dev.tcr = data[0]
		} else { // CNTR0
			valOut = dev.cntr0; dev.cntr0 = 0;
		}
	case NE2000_REG_CNTR1: // Also NE2000_REG_DCR (0x0E)
		if isWrite { // DCR
			dev.dcr = data[0]
		} else { // CNTR1
			valOut = dev.cntr1; dev.cntr1 = 0;
		}
	case NE2000_REG_CNTR2: // Also NE2000_REG_IMR (0x0F)
		if isWrite { // IMR
			dev.imr = data[0]
		} else { // CNTR2
			valOut = dev.cntr2; dev.cntr2 = 0;
		}
	default:
		return 0, fmt.Errorf("ne2000: unhandled Page 0 I/O to offset 0x%X", uint8(offset))
	}
	return valOut, nil
}

// handlePage1IO handles I/O for DP8390 Page 1 registers
func (dev *NE2000Device) handlePage1IO(offset uint16, data []byte, isWrite bool) (uint8, error) { // Changed offset to uint16
	var valOut uint8
	// log.Printf("NE2000 Page 1 IO: offset=0x%02X, isWrite=%v", offset, isWrite)
	switch uint8(offset) { // Switch on uint8 version of offset
	case NE2000_REG_PAR0, NE2000_REG_PAR1, NE2000_REG_PAR2, NE2000_REG_PAR3, NE2000_REG_PAR4, NE2000_REG_PAR5:
		macIndex := uint8(offset) - NE2000_REG_PAR0 // Cast offset
		if isWrite { dev.macAddress[macIndex] = data[0] } else { valOut = dev.macAddress[macIndex] }
	case NE2000_REG_CURR: // Current Page Register
		if isWrite { dev.curr = data[0] } else { valOut = dev.curr }
	// MAR0-MAR7 (Multicast Address Registers) at 0x08-0x0F
	case 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F:
		// marIndex := offset - 0x08
		// if isWrite { dev.mar[marIndex] = data[0] } else { valOut = dev.mar[marIndex] }
		log.Printf("NE2000 Page 1: MAR%d access (offset 0x%02X) - Not Implemented", offset-0x08, offset)
	default:
		return 0, fmt.Errorf("ne2000: unhandled Page 1 I/O to offset 0x%X", offset)
	}
	return valOut, nil
}

// handlePage2IO handles I/O for DP8390 Page 2 registers
func (dev *NE2000Device) handlePage2IO(offset uint16, data []byte, isWrite bool) (uint8, error) { // Changed offset to uint16
	// Page 2 is often for remote DMA current address or other specific functions.
	// For basic PIO, it might not be heavily used.
	log.Printf("NE2000 Page 2 IO: offset=0x%02X, isWrite=%v - Not Implemented", offset, isWrite)
	return 0, fmt.Errorf("ne2000: Page 2 I/O not implemented (offset 0x%X)", uint8(offset)) // Cast for error message
}

// Removing commented out helper functions that might be causing parsing issues.
// These can be re-added later if needed and syntax verified.

// Ports returns the I/O port ranges this device handles.
// NE2000 typically uses 32 I/O ports.
// 0x00-0x0F are DP8390 registers (selected by page bits in CR).
// 0x10-0x1F are NE2000 ASIC specific (Data port, Reset).
func (dev *NE2000Device) Ports() []uint16 {
	ports := make([]uint16, 32)
	for i := 0; i < 32; i++ {
		ports[i] = dev.ioBase + uint16(i)
	}
	return ports
}

// Other helper methods for DMA, packet processing, interrupt management will go here.
// e.g., startTx, processRx, updateIsr, etc.
// For PIO data transfer via NE2000_ASIC_DATA, a separate state machine is needed.
// This usually involves setting up remote DMA registers (RSAR0/1, RBCR0/1) and then
// reading/writing words from/to NE2000_ASIC_DATA.

// reset simulates a hardware reset or CR_STP=1.
func (dev *NE2000Device) reset() {
	log.Println("NE2000: Resetting card state.")
	dev.cr = CR_STP | CR_RD2 // Stopped, Page 0, DMA complete
	dev.isr = ISR_RST        // Set Reset status
	dev.imr = 0x00           // Mask all interrupts
	dev.rcr = 0x00           // Standard receive config
	dev.tcr = 0x00           // Normal transmit config (no loopback)
	// DCR typically retains WTS, BOS, LS settings across STP, but FT might reset.
	// For a full reset, it might go to 0x0A (byte, normal, 8-byte FIFO) or similar.
	// Let's keep DCR as initialized for now, or set to a known safe default like:
	// dev.dcr = DCR_WTS | DCR_FT0 | DCR_FT1 (word, 12-byte FIFO)

	// RX ring pointers
	dev.bnry = dev.pstart
	dev.curr = dev.pstart + 1 // Or pstart if NIC clears it. Let's assume pstart+1.
	dev.nextPacketPtr = dev.pstart + 1

	// Clear counters (optional, depends on model)
	dev.cntr0, dev.cntr1, dev.cntr2 = 0,0,0

	// Clear TSR, RSR (these are status of last op, so reset makes sense)
	dev.tsr = 0
	dev.rsr = 0

	// TODO: Abort any ongoing DMA, clear FIFOs (conceptually).
}
