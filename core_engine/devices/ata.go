package devices

import (
	"fmt"
	"log"
	"sync"
	// "time" // May be needed later for timing BSY flag, etc.
)

const (
	ataMaxTransferSectors = 256 // Max sectors in one PIO command (typically 256 for 8-bit sector count, 65536 for 16-bit)
	ataIdentifyDeviceSize = 512 // Size of IDENTIFY DEVICE data
)

// ATADrive represents a single ATA drive (master or slave).
// For simplicity, we'll only model one drive (master) for now.
type ATADrive struct {
	image         VirtualDiskImage
	exists        bool
	isLBA48       bool   // Supports LBA48 addressing
	identifyData  [ataIdentifyDeviceSize]byte
	// More drive-specific parameters can be added here (cylinders, heads, sectors per track for CHS)
}


// ATADevice represents an ATA controller (IDE channel).
type ATADevice struct {
	mu sync.Mutex

	// I/O Ports this device handles
	ioBase uint16
	ctlBase uint16

	// Registers (Primary channel, master drive perspective)
	// Note: ATA registers are often 16-bit for data, but commands/status are 8-bit.
	// We'll model them based on their typical access size by the CPU.
	errorReg      byte   // Read: Error Register (0x1F1)
	featuresReg   byte   // Write: Features Register (0x1F1)
	sectorCountReg byte  // Sector Count (0x1F2)
	lbaLowReg     byte   // Sector Number / LBA bits 0-7 (0x1F3)
	lbaMidReg     byte   // Cylinder Low / LBA bits 8-15 (0x1F4)
	lbaHighReg    byte   // Cylinder High / LBA bits 16-23 (0x1F5)
	driveHeadReg  byte   // Drive/Head Register (0x1F6)
	statusReg     byte   // Read: Status Register (0x1F7)
	commandReg    byte   // Write: Command Register (0x1F7)

	altStatusReg   byte   // Read: Alternate Status Register (0x3F6)
	devControlReg  byte   // Write: Device Control Register (0x3F6)

	// Data buffer for PIO transfers (typically one sector, 512 bytes for ATA_SECTOR_SIZE)
	// Data is read/written word by word (16-bit) through the Data Register (0x1F0).
	dataBuffer    []uint16 // Stores words, ready for CPU to read/write
	dataBufferPtr int      // Current position in dataBuffer for R/W operations

	// Drive information (only one master drive supported for now)
	masterDrive   ATADrive
	// slaveDrive ATADrive // Placeholder for future slave drive support

	selectedDrive uint8 // 0 for master, 1 for slave (derived from driveHeadReg)
	isLBA         bool  // Current addressing mode (LBA or CHS)

	// Interrupt handling
	interruptRaiser InterruptRaiser // To signal IRQs to the PIC
	irqNumber       uint8           // Typically IRQ 14 for primary ATA
	nIEN            bool            // Interrupt disable from Device Control Register

	// Internal state for command processing
	currentCommand    byte
	sectorsToTransfer uint16
	// More state vars for LBA48, multi-sector transfers, etc.
}

// NewATADevice creates a new ATA device emulator.
// For now, it assumes a single master drive on the primary channel.
func NewATADevice(image VirtualDiskImage, irqRaiser InterruptRaiser, irqNum uint8) (*ATADevice, error) {
	if image == nil {
		return nil, fmt.Errorf("ATA device requires a disk image")
	}
	if image.SectorSize() != ATA_SECTOR_SIZE {
		return nil, fmt.Errorf("disk image sector size (%d) not compatible with ATA default (%d)", image.SectorSize(), ATA_SECTOR_SIZE)
	}

	dev := &ATADevice{
		ioBase:          ATA_PRIMARY_IO_BASE,
		ctlBase:         ATA_PRIMARY_CTL_BASE,
		interruptRaiser: irqRaiser,
		irqNumber:       irqNum,
		masterDrive: ATADrive{
			image:  image,
			exists: true,
			// isLBA48: false, // Depends on image size / features
		},
		// Initial register states
		// Status: Not BSY, DRDY (if disk exists and ready), DRQ cleared, ERR cleared.
		// For a freshly powered on drive, DRDY might take time or depend on IDENTIFY.
		// Let's assume DRDY is initially set if a disk is present.
		statusReg: ATA_SR_DRDY | ATA_SR_DSC, // DSC (Seek Complete) often set too.
		driveHeadReg: ATA_DH_FIXED_BITS | ATA_DH_DRV_MASTER, // Master selected, LBA mode off by default.
		devControlReg: ATA_DCR_NIEN, // Interrupts disabled by default via nIEN=1
		nIEN: true,
	}

	dev.generateIdentifyData()

	log.Printf("ATA Device initialized for Primary Channel (IRQ %d). Master drive size: %d MB",
		dev.irqNumber, image.Size()/(1024*1024))

	return dev, nil
}

// generateIdentifyData creates the 512-byte buffer returned by IDENTIFY DEVICE.
// This is a simplified version.
func (dev *ATADevice) generateIdentifyData() {
	// For simplicity, we'll use a fixed buffer. A real implementation would query the VirtualDiskImage.
	// Word 0: General configuration (e.g., 0x0040 for ATA device, non-removable)
	dev.masterDrive.identifyData[0*2+1] = 0x00 // word 0 high byte
	dev.masterDrive.identifyData[0*2+0] = 0x40 // word 0 low byte (bit 6 indicates non-removable, bit 15 is 0 for ATA)

	// Words 10-19: Serial Number (20 ASCII Chars)
	serial := "VA-SN012345678901234" // Exactly 20 chars
	baseSerialOffset := 10 * 2      // Word 10 starts at byte 20
	for i := 0; i < len(serial); i += 2 {
		dev.masterDrive.identifyData[baseSerialOffset+i+0] = serial[i+1] // ATA strings are byte-swapped pairs
		dev.masterDrive.identifyData[baseSerialOffset+i+1] = serial[i+0]
	}

	// Words 23-26: Firmware Revision (8 ASCII Chars)
	firmware := "VER1.0  " // Exactly 8 chars
	baseFirmwareOffset := 23 * 2 // Word 23 starts at byte 46
	for i := 0; i < len(firmware); i+=2 {
		dev.masterDrive.identifyData[baseFirmwareOffset+i+0] = firmware[i+1]
		dev.masterDrive.identifyData[baseFirmwareOffset+i+1] = firmware[i+0]
	}

	// Words 27-46: Model Number (40 ASCII Chars)
	model := "V-ARCHITECT VIRTUAL DISK              " // Exactly 40 chars
	baseModelOffset := 27 * 2 // Word 27 starts at byte 54
	for i := 0; i < len(model); i += 2 {
		dev.masterDrive.identifyData[baseModelOffset+i+0] = model[i+1]
		dev.masterDrive.identifyData[baseModelOffset+i+1] = model[i+0]
	}

	// Word 49: Capabilities (e.g., LBA supported 0x0200)
	dev.masterDrive.identifyData[49*2+1] = 0x02 // word 49 high byte (bit 9: LBA supported)
	dev.masterDrive.identifyData[49*2+0] = 0x00 // word 49 low byte
	dev.masterDrive.identifyData[49*2+1] = 0x02 // High byte -> LBA supported

	// Word 53: Fields valid (e.g., 0x0001 words 54-58 valid, 0x0002 words 64-70 valid)
	// Let's say words 54-58 (CHS info) and 60-61 (LBA28 sectors) are valid.
	dev.masterDrive.identifyData[53*2+0] = 0x03 // low byte: bit0 and bit1 set
	dev.masterDrive.identifyData[53*2+1] = 0x00 // high byte

	// Words 60-61: Total number of user addressable sectors (LBA28)
	numSectorsLBA28 := dev.masterDrive.image.Size() / uint64(dev.masterDrive.image.SectorSize())
	if numSectorsLBA28 > 0xFFFFFFFF { // Max for LBA28
		numSectorsLBA28 = 0xFFFFFFFF
	}
	dev.masterDrive.identifyData[60*2+0] = byte(numSectorsLBA28 & 0xFF)
	dev.masterDrive.identifyData[60*2+1] = byte((numSectorsLBA28 >> 8) & 0xFF)
	dev.masterDrive.identifyData[61*2+0] = byte((numSectorsLBA28 >> 16) & 0xFF)
	dev.masterDrive.identifyData[61*2+1] = byte((numSectorsLBA28 >> 24) & 0xFF)

	// Word 83: Command set supported (e.g., LBA48 if supported)
	// Bit 10: LBA48 support. If true, words 100-103 are valid.
	// For now, assume no LBA48 explicitly in identify, but can be enabled via SET FEATURES.
	// dev.masterDrive.identifyData[83*2+1] |= (1 << (10-8)) // Bit 10 is in high byte, shifted by 8

	// TODO: Populate more fields for better compatibility (CHS geometry, LBA48 size if applicable, etc.)
}


// HandleIO processes PIO reads and writes to the ATA controller's registers.
func (dev *ATADevice) HandleIO(port uint16, data []byte, isWrite bool) (uint8, error) {
	dev.mu.Lock()
	defer dev.mu.Unlock()

	// log.Printf("ATA HandleIO: port=0x%X, isWrite=%v, data=%#v, BSY=%v, DRQ=%v, DRDY=%v",
	//	port, isWrite, data, (dev.statusReg&ATA_SR_BSY) != 0, (dev.statusReg&ATA_SR_DRQ) != 0, (dev.statusReg&ATA_SR_DRDY) != 0)

	var valueRead uint8
	var err error

	// Calculate offset from base I/O or control base
	if port >= dev.ioBase && port < dev.ioBase+8 {
		regOffset := port - dev.ioBase
		switch regOffset {
		case ATA_REG_DATA: // 0x1F0
			if isWrite {
				// Write to data buffer (handle 16-bit PIO)
				if len(data) < 2 {
					return 0, fmt.Errorf("ata: data write to 0x%X requires 2 bytes, got %d", port, len(data))
				}
				if dev.dataBufferPtr >= len(dev.dataBuffer) {
					dev.statusReg |= ATA_SR_ERR // Should not happen if DRQ logic is correct
					return 0, fmt.Errorf("ata: data write buffer overflow (ptr %d, len %d)", dev.dataBufferPtr, len(dev.dataBuffer))
				}
				word := uint16(data[0]) | (uint16(data[1]) << 8)
				dev.dataBuffer[dev.dataBufferPtr] = word
				dev.dataBufferPtr++

				// Compare total bytes written from buffer vs total bytes for command
				if uint64(dev.dataBufferPtr*2) >= uint64(dev.sectorsToTransfer)*uint64(ATA_SECTOR_SIZE) {
					// All data written for current command
					dev.statusReg &= (^ (ATA_SR_BSY | ATA_SR_DRQ) & 0xFF)
					dev.statusReg |= ATA_SR_DRDY
					// Potentially signal interrupt here if command complete
					log.Printf("ATA: Finished writing data for command 0x%02X", dev.currentCommand)
				}

			} else {
				// Read from data buffer (handle 16-bit PIO)
				if (dev.statusReg & ATA_SR_DRQ) == 0 && dev.currentCommand != ATA_CMD_IDENTIFY_DEVICE { // IDENTIFY might not have DRQ for first word
					// Do not return error, but value could be garbage if read when not DRQ
					log.Printf("ATA: Read from data port 0x%X when DRQ is not set. Status: 0x%02X", port, dev.statusReg)
					// Return last value or 0xFFFF? For now, let it proceed but it's unusual.
				}

				if dev.currentCommand == ATA_CMD_IDENTIFY_DEVICE {
					if dev.dataBufferPtr >= ataIdentifyDeviceSize {
						log.Printf("ATA: Read from data port 0x%X for IDENTIFY past end of buffer. Ptr: %d. Status was 0x%02X", port, dev.dataBufferPtr, dev.statusReg)
						// This case should ideally not be hit if DRQ is managed correctly.
						// If DRQ is clear, the guest shouldn't be reading.
						valueRead = 0xFF // Error or unexpected data
					} else {
						valueRead = dev.masterDrive.identifyData[dev.dataBufferPtr]
						dev.dataBufferPtr++
						if dev.dataBufferPtr >= ataIdentifyDeviceSize {
							dev.statusReg &= (^ (ATA_SR_BSY | ATA_SR_DRQ) & 0xFF) // Clear BSY and DRQ
							dev.statusReg |= ATA_SR_DRDY                          // Set DRDY, ready for next cmd
							log.Println("ATA: IDENTIFY_DEVICE data transfer complete.")
						}
					}
				} else {
					// Logic for reading actual sector data from dev.dataBuffer (for READ SECTORS)
					// This part is not yet fully implemented.
					// The old `if dev.dataBufferPtr >= len(dev.dataBuffer)` and `word := dev.dataBuffer[dev.dataBufferPtr]`
					// would go here, adapted for byte-wise return if HandleIO must return uint8.
					log.Printf("ATA: Read from data port 0x%X for non-IDENTIFY command (Cmd: 0x%02X) - NOT FULLY IMPLEMENTED", port, dev.currentCommand)
					valueRead = 0xFF // Placeholder for other read commands
				}
			}
		case ATA_REG_ERROR: // 0x1F1 - Read Error Register
			if isWrite { // Write to Features Register
				dev.featuresReg = data[0]
				// log.Printf("ATA: Features set to 0x%02X", dev.featuresReg)
			} else {
				valueRead = dev.errorReg
			}
		case ATA_REG_SECTOR_COUNT: // 0x1F2
			if isWrite {
				dev.sectorCountReg = data[0]
			} else {
				valueRead = dev.sectorCountReg
			}
		case ATA_REG_LBA_LOW: // 0x1F3
			if isWrite {
				dev.lbaLowReg = data[0]
			} else {
				valueRead = dev.lbaLowReg
			}
		case ATA_REG_LBA_MID: // 0x1F4
			if isWrite {
				dev.lbaMidReg = data[0]
			} else {
				valueRead = dev.lbaMidReg
			}
		case ATA_REG_LBA_HIGH: // 0x1F5
			if isWrite {
				dev.lbaHighReg = data[0]
			} else {
				valueRead = dev.lbaHighReg
			}
		case ATA_REG_DRIVE_HEAD: // 0x1F6
			if isWrite {
				dev.driveHeadReg = data[0]
				// Update selectedDrive and isLBA based on this write
				if (data[0] & (1 << ATA_DH_DRV_SELECT_SHIFT)) == ATA_DH_DRV_SLAVE {
					dev.selectedDrive = 1
				} else {
					dev.selectedDrive = 0
				}
				dev.isLBA = (data[0] & ATA_DH_LBA_MODE) != 0
				// log.Printf("ATA: Drive/Head set to 0x%02X. Selected Drive: %d, LBA Mode: %v", data[0], dev.selectedDrive, dev.isLBA)
			} else {
				valueRead = dev.driveHeadReg
			}
		case ATA_REG_STATUS: // 0x1F7 - Read Status / Write Command
			if isWrite { // Write to Command Register
				dev.commandReg = data[0]
				dev.processCommand(dev.commandReg)
			} else { // Read Status Register
				valueRead = dev.statusReg
				// Reading status register clears a pending IRQ if nIEN is 0
				if !dev.nIEN {
					// log.Printf("ATA: Status read, clearing IRQ %d (placeholder)", dev.irqNumber)
					// dev.interruptRaiser.LowerIRQ(dev.irqNumber) // Actual IRQ lowering is complex
				}
			}
		default:
			err = fmt.Errorf("ata: unhandled I/O offset 0x%X in primary block", regOffset)
		}
	} else if port >= dev.ctlBase && port < dev.ctlBase+2 { // Typically 0x3F6, 0x3F7
		regOffset := port - dev.ctlBase
		switch regOffset {
		case ATA_REG_ALT_STATUS: // 0x3F6 (offset 0 from ctlBase) - Read Alt Status / Write Dev Control
			if isWrite { // Device Control Register
				dev.devControlReg = data[0]
				oldNIEN := dev.nIEN
				dev.nIEN = (dev.devControlReg & ATA_DCR_NIEN) != 0
				if dev.nIEN && !oldNIEN {
					// log.Printf("ATA: Interrupts disabled via DCR (nIEN set)")
				} else if !dev.nIEN && oldNIEN {
					// log.Printf("ATA: Interrupts enabled via DCR (nIEN cleared)")
				}
				if (dev.devControlReg & ATA_DCR_SRST) != 0 {
					log.Println("ATA: Software Reset (SRST) requested.")
					// dev.resetController() // Implement reset logic
					// SRST is self-clearing. For now, just log.
				}
			} else { // Alternate Status Register
				valueRead = dev.statusReg // AltStatus mirrors Status but read doesn't affect IRQ
			}
		// case ATA_REG_DRIVE_ADDRESS: // 0x3F7 (offset 1 from ctlBase) - Often unused or for diagnostics
		// if isWrite { /* usually no effect */ } else { valueRead = 0 /* or some diagnostic byte */ }
		default:
			err = fmt.Errorf("ata: unhandled I/O offset 0x%X in control block", regOffset)
		}
	} else {
		err = fmt.Errorf("ata: access to unhandled port 0x%X", port)
	}

	// log.Printf("ATA HandleIO Result: port=0x%X, isWrite=%v, valueRead=0x%02X, err=%v", port, isWrite, valueRead, err)
	return valueRead, err
}

// processCommand handles ATA commands written to the command register.
func (dev *ATADevice) processCommand(cmd byte) {
	// log.Printf("ATA: Received command 0x%02X", cmd)
	dev.currentCommand = cmd
	dev.errorReg = 0 // Clear error register on new command

	// Check if drive is ready (not BSY, DRDY is set) - except for IDENTIFY or RESET
	if cmd != ATA_CMD_IDENTIFY_DEVICE && (dev.devControlReg & ATA_DCR_SRST) == 0 {
		if (dev.statusReg & ATA_SR_BSY) != 0 {
			log.Printf("ATA Warning: Command 0x%02X received while BSY set.", cmd)
			dev.errorReg = ATA_ER_ABRT // Or just ignore command
			dev.statusReg |= ATA_SR_ERR
			return
		}
		if (dev.statusReg & ATA_SR_DRDY) == 0 && cmd != 0x00 /*NOP?*/ {
			log.Printf("ATA Warning: Command 0x%02X received when DRDY not set.", cmd)
			dev.errorReg = ATA_ER_ABRT
			dev.statusReg |= ATA_SR_ERR
			return
		}
	}


	switch cmd {
	case ATA_CMD_IDENTIFY_DEVICE:
		log.Println("ATA: IDENTIFY_DEVICE command")
		if dev.selectedDrive == 0 && dev.masterDrive.exists { // Master drive
			dev.statusReg |= ATA_SR_BSY  // Set BSY
			dev.statusReg &= (^ (ATA_SR_ERR | ATA_SR_DRQ) & 0xFF) // Clear ERR, DRQ

			// Simulate command processing time (very short for IDENTIFY)
			// In a real emu, this might schedule an event. Here, immediate prep.

			// Data buffer for PIO transfers is usually for sector data.
			// For IDENTIFY, data is directly from a pre-generated buffer.
			// We need to set up dataBufferPtr for reading this identifyData.
			dev.dataBufferPtr = 0 // Reset pointer for reading identify data

			dev.statusReg &= (^ATA_SR_BSY & 0xFF) // Clear BSY
			dev.statusReg |= ATA_SR_DRQ  // Set DRQ, data ready
			dev.statusReg |= ATA_SR_DRDY // Ensure DRDY is set

			// Generate interrupt (if nIEN is clear)
			if !dev.nIEN {
				log.Printf("ATA: IDENTIFY_DEVICE complete, DRQ set. Signaling IRQ %d (placeholder)", dev.irqNumber)
				// dev.interruptRaiser.RaiseIRQ(dev.irqNumber)
			}
		} else { // Slave or non-existent drive
			dev.statusReg |= ATA_SR_ERR
			dev.errorReg = ATA_ER_ABRT // No device or slave not supported
			log.Printf("ATA: IDENTIFY_DEVICE failed - drive %d not found/supported.", dev.selectedDrive)
		}

	case ATA_CMD_READ_SECTORS, ATA_CMD_READ_SECTORS_NO_RETRY:
		log.Printf("ATA: READ_SECTORS command (LBA: %02X%02X%02X, Count: %d) - NOT FULLY IMPLEMENTED",
			dev.lbaHighReg, dev.lbaMidReg, dev.lbaLowReg, dev.sectorCountReg)
		// 1. Set BSY
		// 2. Read sectors from disk image into internal buffer (dev.dataBuffer)
		// 3. Clear BSY, Set DRQ
		// 4. Signal IRQ
		// Data will be read by CPU from Data Port word by word.
		// Each word read decrements internal count, when buffer empty & more sectors, repeat.
		dev.statusReg |= ATA_SR_BSY
		dev.statusReg &= (^ (ATA_SR_ERR | ATA_SR_DRQ) & 0xFF)
		// Placeholder:
		dev.sectorsToTransfer = uint16(dev.sectorCountReg)
		if dev.sectorsToTransfer == 0 { dev.sectorsToTransfer = 256 } // 0 means 256 sectors
		// For now, just set DRQ as if one sector is ready.
		// dev.prepareReadBuffer(lba, 1) // Internal helper
		dev.statusReg &= (^ATA_SR_BSY & 0xFF)
		dev.statusReg |= ATA_SR_DRQ
		if !dev.nIEN { log.Printf("ATA: READ_SECTORS ready for data. Signaling IRQ %d (placeholder)", dev.irqNumber) }


	case ATA_CMD_WRITE_SECTORS, ATA_CMD_WRITE_SECTORS_NO_RETRY:
		log.Printf("ATA: WRITE_SECTORS command (LBA: %02X%02X%02X, Count: %d) - NOT FULLY IMPLEMENTED",
			dev.lbaHighReg, dev.lbaMidReg, dev.lbaLowReg, dev.sectorCountReg)
		// 1. Set BSY (briefly)
		// 2. Set DRQ, Clear BSY (ready to receive data from CPU)
		// 3. CPU writes data to Data Port word by word.
		// 4. When buffer full, write to disk image. If more sectors, repeat DRQ.
		// 5. When all sectors written, signal IRQ.
		dev.statusReg |= ATA_SR_BSY
		dev.statusReg &= (^ATA_SR_ERR & 0xFF)
		dev.sectorsToTransfer = uint16(dev.sectorCountReg)
		if dev.sectorsToTransfer == 0 { dev.sectorsToTransfer = 256 }

		// Allocate buffer for one sector for now
		// dev.dataBuffer = make([]uint16, ATA_SECTOR_SIZE/2) // Handled by PIO state machine
		dev.dataBufferPtr = 0

		dev.statusReg &= (^ATA_SR_BSY & 0xFF)
		dev.statusReg |= ATA_SR_DRQ // Ready for CPU to write data
		// No IRQ yet until data is received and written.


	case ATA_CMD_SET_FEATURES:
		log.Printf("ATA: SET_FEATURES command (Feature: 0x%02X) - NOT IMPLEMENTED", dev.featuresReg)
		// Common use: enable/disable write cache, set transfer mode.
		// For now, just acknowledge command by clearing BSY and setting DRDY.
		dev.statusReg &= (^ATA_SR_BSY & 0xFF)
		dev.statusReg |= ATA_SR_DRDY

	case ATA_CMD_FLUSH_CACHE, 0xE0 /*STANDBYIMMEDIATE*/, 0x90 /*EXECUTE DEVICE DIAGNOSTIC*/:
		log.Printf("ATA: Command 0x%02X received (treated as NOP for now).", cmd)
		// These commands complete quickly without data transfer.
		dev.statusReg &= (^ATA_SR_BSY & 0xFF) // Clear BSY
		dev.statusReg |= ATA_SR_DRDY // Ensure DRDY
		if !dev.nIEN {
			log.Printf("ATA: NOP-like command 0x%02X complete. Signaling IRQ %d (placeholder)", cmd, dev.irqNumber)
			// dev.interruptRaiser.RaiseIRQ(dev.irqNumber)
		}

	default:
		log.Printf("ATA: Unhandled command: 0x%02X", cmd)
		dev.statusReg |= ATA_SR_ERR
		dev.errorReg = ATA_ER_ABRT // Aborted command
		if !dev.nIEN {
			log.Printf("ATA: Unhandled command 0x%02X. Signaling IRQ %d (placeholder)", cmd, dev.irqNumber)
			// dev.interruptRaiser.RaiseIRQ(dev.irqNumber)
		}
	}
} // This closing brace ends processCommand

// Ports returns the I/O port ranges this device handles.
func (dev *ATADevice) Ports() []uint16 {
	return []uint16{
		// Primary channel command block
		ATA_PRIMARY_IO_BASE + ATA_REG_DATA,
		ATA_PRIMARY_IO_BASE + ATA_REG_ERROR, // Also Features
		ATA_PRIMARY_IO_BASE + ATA_REG_SECTOR_COUNT,
		ATA_PRIMARY_IO_BASE + ATA_REG_LBA_LOW,
		ATA_PRIMARY_IO_BASE + ATA_REG_LBA_MID,
		ATA_PRIMARY_IO_BASE + ATA_REG_LBA_HIGH,
		ATA_PRIMARY_IO_BASE + ATA_REG_DRIVE_HEAD,
		ATA_PRIMARY_IO_BASE + ATA_REG_STATUS,   // Also Command

		// Primary channel control block
		ATA_PRIMARY_CTL_BASE + ATA_REG_ALT_STATUS, // Also Device Control
		// ATA_PRIMARY_CTL_BASE + ATA_REG_DRIVE_ADDRESS, // If used
	}
}
