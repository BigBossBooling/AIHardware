package devices

// ATA Controller I/O Ports (Primary Channel)
const (
	ATA_PRIMARY_IO_BASE    uint16 = 0x1F0
	ATA_PRIMARY_CTL_BASE   uint16 = 0x3F6
)

// Register offsets from ATA_PRIMARY_IO_BASE (0x1F0)
const (
	ATA_REG_DATA        = 0 // Data Register (R/W, 16-bit)
	ATA_REG_ERROR       = 1 // Error Register (R)
	ATA_REG_FEATURES    = 1 // Features Register (W)
	ATA_REG_SECTOR_COUNT= 2 // Sector Count Register (R/W)
	ATA_REG_LBA_LOW     = 3 // LBA Low / Sector Number (R/W)
	ATA_REG_LBA_MID     = 4 // LBA Mid / Cylinder Low (R/W)
	ATA_REG_LBA_HIGH    = 5 // LBA High / Cylinder High (R/W)
	ATA_REG_DRIVE_HEAD  = 6 // Drive/Head Register (R/W)
	ATA_REG_STATUS      = 7 // Status Register (R)
	ATA_REG_COMMAND     = 7 // Command Register (W)
)

// Register offsets from ATA_PRIMARY_CTL_BASE (0x3F6)
// Note: The standard port is 0x3F6 for Alternate Status / Device Control.
// Some sources might list 0x3F7 for something, but 0x3F6 is the key control port.
const (
	ATA_REG_ALT_STATUS      = 0 // Alternate Status Register (R, same as Status but doesn't clear interrupt)
	ATA_REG_DEVICE_CONTROL  = 0 // Device Control Register (W)
	// ATA_REG_DRIVE_ADDRESS = 1 // Drive Address Register (R, specific to some chipsets, often unused for basic IDE)
)

// Status Register (ATA_REG_STATUS / ATA_REG_ALT_STATUS) bits
const (
	ATA_SR_ERR  = 1 << 0 // Error
	ATA_SR_IDX  = 1 << 1 // Index (unused)
	ATA_SR_CORR = 1 << 2 // Corrected data (unused)
	ATA_SR_DRQ  = 1 << 3 // Data Request (ready to transfer data)
	ATA_SR_DSC  = 1 << 4 // Drive Seek Complete (or Service)
	ATA_SR_DF   = 1 << 5 // Drive Fault / Device Fault
	ATA_SR_DRDY = 1 << 6 // Drive Ready
	ATA_SR_BSY  = 1 << 7 // Busy
)

// Error Register (ATA_REG_ERROR) bits
const (
	ATA_ER_AMNF = 1 << 0 // Address Mark Not Found
	ATA_ER_TK0NF= 1 << 1 // Track 0 Not Found
	ATA_ER_ABRT = 1 << 2 // Aborted Command
	ATA_ER_MCR  = 1 << 3 // Media Change Request
	ATA_ER_IDNF = 1 << 4 // ID Not Found
	ATA_ER_MC   = 1 << 5 // Media Changed
	ATA_ER_UNC  = 1 << 6 // Uncorrectable Data Error
	ATA_ER_BBK  = 1 << 7 // Bad Block Detected
)

// Drive/Head Register (ATA_REG_DRIVE_HEAD) bits
const (
	// Bits 0-3: Head Number (for CHS addressing, or LBA bits 24-27 if LBA48)
	ATA_DH_DRV_SELECT_SHIFT = 4
	ATA_DH_DRV_MASTER       = 0 << ATA_DH_DRV_SELECT_SHIFT // Select Master Drive (Drive 0)
	ATA_DH_DRV_SLAVE        = 1 << ATA_DH_DRV_SELECT_SHIFT // Select Slave Drive (Drive 1)
	ATA_DH_LBA_MODE         = 1 << 6 // LBA mode enable (must be 1 for LBA)
	// Bit 7 and 5 are fixed (101xxxxx for legacy reasons)
	ATA_DH_FIXED_BITS       = (1 << 7) | (1 << 5)
)

// Device Control Register (ATA_REG_DEVICE_CONTROL) bits
const (
	ATA_DCR_NIEN = 1 << 1 // nIEN: Interrupt Enable (0=enabled, 1=disabled)
	ATA_DCR_SRST = 1 << 2 // SRST: Software Reset
	ATA_DCR_HOB  = 1 << 7 // HOB: High Order Byte (for LBA48, read LBA48 high bytes from 0x1F3-0x1F5)
)

// ATA Commands (written to ATA_REG_COMMAND)
const (
	ATA_CMD_READ_SECTORS      = 0x20 // Read sectors with retry
	ATA_CMD_READ_SECTORS_NO_RETRY = 0x21 // Read sectors without retry
	ATA_CMD_WRITE_SECTORS     = 0x30 // Write sectors with retry
	ATA_CMD_WRITE_SECTORS_NO_RETRY = 0x31 // Write sectors without retry
	ATA_CMD_IDENTIFY_DEVICE   = 0xEC // Identify device
	ATA_CMD_SET_FEATURES      = 0xEF // Set features
	ATA_CMD_FLUSH_CACHE       = 0xE7 // Flush cache
	ATA_CMD_READ_DMA          = 0xC8 // Read DMA
	ATA_CMD_WRITE_DMA         = 0xCA // Write DMA
	// Many more commands exist...
)

// Sector size for ATA devices
const ATA_SECTOR_SIZE = 512
