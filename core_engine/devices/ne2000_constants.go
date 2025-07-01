package devices

// Default I/O base address for NE2000
const NE2000_IO_BASE uint16 = 0x300 // Common default, can be configurable

// Register offsets from NE2000_IO_BASE
// These are for the National Semiconductor DP8390/WD80x3/NE2000 family.
// Some registers are specific to the NE2000 ASIC part, others to the DP8390 part.
// The DP8390 registers are typically accessed when Page 0 is selected in CR.

// Offsets for DP8390 registers (Page 0, unless specified)
const (
	NE2000_REG_CR          = 0x00 // Command Register (R/W) - Controls overall operation
	// Page 0 - Read
	NE2000_REG_CLDA0       = 0x01 // Current Local DMA Address 0 (R)
	NE2000_REG_CLDA1       = 0x02 // Current Local DMA Address 1 (R)
	NE2000_REG_BNRY        = 0x03 // Boundary Pointer (R/W) - For RX ring buffer
	NE2000_REG_TSR         = 0x04 // Transmit Status Register (R)
	NE2000_REG_NCR         = 0x05 // Number of Collisions Register (R)
	NE2000_REG_FIFO        = 0x06 // FIFO (R) - For testing
	NE2000_REG_ISR         = 0x07 // Interrupt Status Register (R/W - write 1 to clear)
	NE2000_REG_CRDA0       = 0x08 // Current Remote DMA Address 0 (R) - Unused in PIO
	NE2000_REG_CRDA1       = 0x09 // Current Remote DMA Address 1 (R) - Unused in PIO
	// 0x0A, 0x0B - Reserved or specific to card vendor
	NE2000_REG_RSR         = 0x0C // Receive Status Register (R)
	NE2000_REG_CNTR0       = 0x0D // Frame Alignment Errors Counter (R)
	NE2000_REG_CNTR1       = 0x0E // CRC Errors Counter (R)
	NE2000_REG_CNTR2       = 0x0F // Missed Packets Counter (R)

	// Page 0 - Write
	NE2000_REG_PSTART      = 0x01 // Page Start Register (W) - Start of RX ring buffer
	NE2000_REG_PSTOP       = 0x02 // Page Stop Register (W) - End of RX ring buffer
	// BNRY is at 0x03 (R/W)
	NE2000_REG_TPSR        = 0x04 // Transmit Page Start Register (W) - TX buffer start
	NE2000_REG_TBCR0       = 0x05 // Transmit Byte Count Register 0 (LSB) (W)
	NE2000_REG_TBCR1       = 0x06 // Transmit Byte Count Register 1 (MSB) (W)
	// ISR is at 0x07 (R/W)
	NE2000_REG_RSAR0       = 0x08 // Remote Start Address Register 0 (LSB) (W) - For Remote DMA
	NE2000_REG_RSAR1       = 0x09 // Remote Start Address Register 1 (MSB) (W) - For Remote DMA
	NE2000_REG_RBCR0       = 0x0A // Remote Byte Count Register 0 (LSB) (W) - For Remote DMA
	NE2000_REG_RBCR1       = 0x0B // Remote Byte Count Register 1 (MSB) (W) - For Remote DMA
	NE2000_REG_RCR         = 0x0C // Receive Configuration Register (W)
	NE2000_REG_TCR         = 0x0D // Transmit Configuration Register (W)
	NE2000_REG_DCR         = 0x0E // Data Configuration Register (W)
	NE2000_REG_IMR         = 0x0F // Interrupt Mask Register (W)

	// Page 1 (CR_PS0=1, CR_PS1=0) - Read/Write
	// Physical Address Registers (PAR0-PAR5) for MAC address
	NE2000_REG_PAR0        = 0x01 // Physical Address Register 0 (MAC byte 0)
	NE2000_REG_PAR1        = 0x02 // Physical Address Register 1 (MAC byte 1)
	NE2000_REG_PAR2        = 0x03 // Physical Address Register 2 (MAC byte 2)
	NE2000_REG_PAR3        = 0x04 // Physical Address Register 3 (MAC byte 3)
	NE2000_REG_PAR4        = 0x05 // Physical Address Register 4 (MAC byte 4)
	NE2000_REG_PAR5        = 0x06 // Physical Address Register 5 (MAC byte 5)
	NE2000_REG_CURR        = 0x07 // Current Page Register (CPR) - Points to next available RX buffer page
	// MAR0-MAR7 (Multicast Address Registers) at 0x08-0x0F on Page 1

	// NE2000 ASIC specific registers (these are typically outside the DP8390 block)
	// For a generic NE2000, these are usually at base + 0x10 to base + 0x1F
	NE2000_ASIC_DATA       = 0x10 // Data port for memory R/W (PIO) or Remote DMA. Often 16-bit.
	NE2000_ASIC_RESET      = 0x1F // Reset port (read from this port to reset NIC)
)

// Command Register (CR) bits
const (
	CR_STP  = 1 << 0 // Stop: Software reset, puts NIC in reset state
	CR_STA  = 1 << 1 // Start: Activates NIC after configuration
	CR_TXP  = 1 << 2 // Transmit Packet: Initiates transmission of packet in TPSR
	CR_RD0  = 1 << 3 // Remote DMA Read
	CR_RD1  = 1 << 4 // Remote DMA Write
	CR_RD2  = 1 << 5 // Remote DMA Complete (send packet)
	CR_PS0  = 1 << 6 // Page Select bit 0
	CR_PS1  = 1 << 7 // Page Select bit 1
	// Page selection: PS1 PS0
	// 0  0  Page 0 (DP8390 registers)
	// 0  1  Page 1 (PARs, CURR, MARs)
	// 1  0  Page 2 (Reserved or specific, e.g. some docs show CRDA here too)
	// 1  1  Page 3 (Diagnostic, vendor specific)
)

// Interrupt Status Register (ISR) bits (Write 1 to clear)
const (
	ISR_PRX = 1 << 0 // Packet Received: Packet received without errors
	ISR_PTX = 1 << 1 // Packet Transmitted: Packet transmitted without error
	ISR_RXE = 1 << 2 // Receive Error: Packet received with error
	ISR_TXE = 1 << 3 // Transmit Error: Transmission aborted due to excessive collisions or FIFO underrun
	ISR_OVW = 1 << 4 // Overwrite Warning: RX buffer ring exhausted
	ISR_CNT = 1 << 5 // Counter Overflow: One or more network tally counters overflowed
	ISR_RDC = 1 << 6 // Remote DMA Complete
	ISR_RST = 1 << 7 // Reset Status: NIC has been reset (either power-on or CR_STP)
)

// Interrupt Mask Register (IMR) bits (Write 1 to enable interrupt)
const (
	IMR_PRXE = 1 << 0 // Packet Received Interrupt Enable
	IMR_PTXE = 1 << 1 // Packet Transmitted Interrupt Enable
	IMR_RXEE = 1 << 2 // Receive Error Interrupt Enable
	IMR_TXEE = 1 << 3 // Transmit Error Interrupt Enable
	IMR_OVWE = 1 << 4 // Overwrite Warning Interrupt Enable
	IMR_CNTE = 1 << 5 // Counter Overflow Interrupt Enable
	IMR_RDCE = 1 << 6 // Remote DMA Complete Interrupt Enable
)

// Data Configuration Register (DCR) bits
const (
	DCR_WTS = 1 << 0 // Word Transfer Select: 0=byte, 1=word (for 16-bit cards)
	DCR_BOS = 1 << 1 // Byte Order Select: 0=MSB first (3210), 1=LSB first (0123) - for word DMA
	DCR_LAS = 1 << 2 // Long Address Select: 0=normal (for PC/AT), 1=other
	DCR_LS  = 1 << 3 // Loopback Select: 0=normal, 1=loopback mode
	DCR_AR  = 1 << 4 // Auto-initialize Remote: 0=no auto-init, 1=auto-init
	DCR_FT0 = 1 << 5 // FIFO Threshold Select bit 0
	DCR_FT1 = 1 << 6 // FIFO Threshold Select bit 1
	// FT1 FT0: Threshold
	// 0  0   2 bytes (or 1 word)
	// 0  1   4 bytes (or 2 words)
	// 1  0   8 bytes (or 4 words)
	// 1  1  12 bytes (or 6 words)
)

// Transmit Configuration Register (TCR) bits
const (
	TCR_CRC = 1 << 0 // Inhibit CRC: 0=CRC appended by Tx, 1=CRC not appended
	TCR_LB0 = 1 << 1 // Loopback Control bit 0
	TCR_LB1 = 1 << 2 // Loopback Control bit 1
	// LB1 LB0: Mode
	//  0   0  Normal operation
	//  0   1  Internal loopback (LPBK_NIC)
	//  1   0  External loopback (LPBK_ENDEC) - PHY loopback
	//  1   1  External loopback (LPBK_TRANSCEIVER)
	TCR_ATD = 1 << 3 // Auto Transmit Disable: 0=normal, 1=auto-disable
	TCR_OFST= 1 << 4 // Collision Offset Enable (for CSMA/CD retries)
)

// Receive Configuration Register (RCR) bits
const (
	RCR_SEP  = 1 << 0 // Save Errored Packets: 0=discard, 1=save
	RCR_AR   = 1 << 1 // Accept Runt Packets (less than 64 bytes)
	RCR_AB   = 1 << 2 // Accept Broadcast Packets
	RCR_AM   = 1 << 3 // Accept Multicast Packets
	RCR_PRO  = 1 << 4 // Promiscuous Physical: Accept all physical addresses
	RCR_MON  = 1 << 5 // Monitor Mode: 0=normal, 1=monitor (no packets to host)
)

// Transmit Status Register (TSR) bits
const (
	TSR_PTX  = 1 << 0 // Packet Transmitted (OK)
	TSR_COL  = 1 << 2 // Transmit Collided
	TSR_ABT  = 1 << 3 // Transmit Aborted (excessive collisions)
	TSR_CRS  = 1 << 4 // Carrier Sense Lost
	TSR_FU   = 1 << 5 // FIFO Underrun
	TSR_CDH  = 1 << 6 // CD Heartbeat (collision detection test)
	TSR_OWC  = 1 << 7 // Out of Window Collision
)

// Receive Status Register (RSR) bits (prepended to each packet in RX buffer)
const (
	RSR_PRX  = 1 << 0 // Packet Received Intact
	RSR_CRC  = 1 << 1 // CRC Error
	RSR_FAE  = 1 << 2 // Frame Alignment Error
	RSR_FO   = 1 << 3 // FIFO Overrun
	RSR_MPA  = 1 << 4 // Missed Packet
	RSR_PHY  = 1 << 5 // Physical/Multicast Address Match
	RSR_DIS  = 1 << 6 // Receiver Disabled (while in monitor mode)
	RSR_DFR  = 1 << 7 // Deferring (collision avoidance)
)

// NE2000 Memory Layout
const (
	NE2000_MEM_PAGE_SIZE = 256    // Bytes per page
	NE2000_MEM_SIZE_8K   = 8192   // 8KB NIC RAM version (32 pages)
	NE2000_MEM_SIZE_16K  = 16384  // 16KB NIC RAM version (64 pages)

	NE2000_PROM_OFFSET   = 0x00   // PROM data (MAC address) is at the beginning of ASIC address space for PIO
	NE2000_PROM_SIZE     = 16     // Bytes for PROM data (MAC is first 6 bytes, then vendor info)
	                               // Note: Some cards have 32 bytes PROM. First 16 are often mirrored.

	// Default TX buffer start page (after PROM if ASIC and NIC RAM are shared conceptually for PIO access)
	// For DP8390, TX buffer is from PSTART to PSTOP-1.
	// But NE2000 typically has a dedicated TX buffer area.
	// Example: 8K card (0x20 pages total, 0x4000-0x5FFF memory map)
	// PROM at 0x4000-0x400F (if mapped via data port)
	// TX buffer: 6 pages (0x40-0x45 for DP8390 TPSR) -> 0x4000-0x45FF in NIC RAM
	// RX ring:   PSTART=0x46, PSTOP=0x60 (0x4600-0x5FFF in NIC RAM)
	NE2000_DEFAULT_TX_START_PAGE = 0x40 // Page number for TPSR (Transmit Page Start Register)
	NE2000_DEFAULT_RX_START_PAGE = 0x46 // Page number for PSTART (Page Start Register)
	NE2000_DEFAULT_RX_STOP_PAGE  = 0x60 // Page number for PSTOP (Page Stop Register) for an 8K card
)

// Typical IRQ for NE2000
const IRQ_NE2000 uint8 = 9 // Or 3, 5, 10, 11, 12 depending on jumper/config
