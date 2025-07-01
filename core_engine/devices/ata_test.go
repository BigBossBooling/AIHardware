package devices

import (
	"bytes"
	"testing"
	// "v-architect/core_engine/hypervisor" // If any KVM specific types were needed, but not for device logic tests
)

// Helper to create a new ATADevice with a default MemoryDiskImage for testing.
func newTestATADevice(t *testing.T) *ATADevice {
	diskSize := uint64(16 * 1024 * 1024) // 16MB
	memDisk, err := NewMemoryDiskImage(diskSize, ATA_SECTOR_SIZE)
	if err != nil {
		t.Fatalf("Failed to create memory disk for test ATA device: %v", err)
	}
	// Using nil for InterruptRaiser and a common IRQ number for primary ATA.
	// Actual interrupt functionality is not tested in these specific unit tests.
	ataDev, err := NewATADevice(memDisk, nil, IRQ_PRIMARY_ATA)
	if err != nil {
		t.Fatalf("Failed to create ATA device for test: %v", err)
	}
	return ataDev
}

func TestATADeviceInitialization(t *testing.T) {
	dev := newTestATADevice(t)
	if dev == nil {
		t.Fatal("NewATADevice returned nil")
	}

	// Check initial status: DRDY and DSC should be set, BSY and DRQ clear.
	expectedStatus := byte(ATA_SR_DRDY | ATA_SR_DSC)
	if dev.statusReg != expectedStatus {
		t.Errorf("Initial status register: expected 0x%02X, got 0x%02X", expectedStatus, dev.statusReg)
	}

	// Check Drive/Head register: Master selected, LBA mode off by default.
	// ATA_DH_FIXED_BITS are 10100000 (0xA0)
	expectedDriveHead := byte(ATA_DH_FIXED_BITS | ATA_DH_DRV_MASTER)
	if dev.driveHeadReg != expectedDriveHead {
		t.Errorf("Initial Drive/Head register: expected 0x%02X, got 0x%02X", expectedDriveHead, dev.driveHeadReg)
	}
	if dev.selectedDrive != 0 {
		t.Errorf("Initial selected drive should be 0 (master), got %d", dev.selectedDrive)
	}
	if dev.isLBA {
		t.Error("Initial LBA mode should be false")
	}

	// Check nIEN is set (interrupts disabled)
	if !dev.nIEN || (dev.devControlReg & ATA_DCR_NIEN) == 0 {
		t.Errorf("Initial nIEN should be true (interrupts disabled), nIEN flag: %v, DCR: 0x%02X", dev.nIEN, dev.devControlReg)
	}
}

func TestATARegisterAccess(t *testing.T) {
	dev := newTestATADevice(t)

	// Test Sector Count Register
	_, err := dev.HandleIO(dev.ioBase+ATA_REG_SECTOR_COUNT, []byte{0x2A}, true) // Write
	if err != nil { t.Fatalf("Write to SectorCount failed: %v", err) }
	if dev.sectorCountReg != 0x2A {
		t.Errorf("SectorCountReg: expected 0x2A, got 0x%02X", dev.sectorCountReg)
	}
	val, err := dev.HandleIO(dev.ioBase+ATA_REG_SECTOR_COUNT, nil, false) // Read
	if err != nil { t.Fatalf("Read from SectorCount failed: %v", err) }
	if val != 0x2A {
		t.Errorf("Read SectorCount: expected 0x2A, got 0x%02X", val)
	}

	// Test LBA Low Register
	_, err = dev.HandleIO(dev.ioBase+ATA_REG_LBA_LOW, []byte{0x12}, true)
	if err != nil { t.Fatalf("Write to LBALow failed: %v", err) }
	if dev.lbaLowReg != 0x12 { t.Errorf("LBALow: expected 0x12, got 0x%02X", dev.lbaLowReg) }
	val, _ = dev.HandleIO(dev.ioBase+ATA_REG_LBA_LOW, nil, false)
	if val != 0x12 { t.Errorf("Read LBALow: expected 0x12, got 0x%02X", val) }

	// Test Drive/Head Register (writing also affects selectedDrive and isLBA)
	// Select Slave, LBA mode
	driveHeadCmd := byte(ATA_DH_FIXED_BITS | ATA_DH_DRV_SLAVE | ATA_DH_LBA_MODE)
	_, err = dev.HandleIO(dev.ioBase+ATA_REG_DRIVE_HEAD, []byte{driveHeadCmd}, true)
	if err != nil { t.Fatalf("Write to DriveHead failed: %v", err) }
	if dev.driveHeadReg != driveHeadCmd { t.Errorf("DriveHeadReg: expected 0x%02X, got 0x%02X", driveHeadCmd, dev.driveHeadReg) }
	if dev.selectedDrive != 1 { t.Errorf("SelectedDrive: expected 1 (slave), got %d", dev.selectedDrive) }
	if !dev.isLBA { t.Error("isLBA: expected true, got false") }
	val, _ = dev.HandleIO(dev.ioBase+ATA_REG_DRIVE_HEAD, nil, false)
	if val != driveHeadCmd { t.Errorf("Read DriveHead: expected 0x%02X, got 0x%02X", driveHeadCmd, val) }

	// Test Device Control Register (nIEN bit)
	// Disable interrupts (nIEN = 0)
	_, err = dev.HandleIO(dev.ctlBase+ATA_REG_DEVICE_CONTROL, []byte{0x00 & ^byte(ATA_DCR_NIEN)}, true)
	if err != nil { t.Fatalf("Write to DeviceControl (clear nIEN) failed: %v", err) }
	if dev.nIEN { t.Error("nIEN should be false after clearing DCR nIEN bit") }

	// Enable interrupts (nIEN = 1)
	_, err = dev.HandleIO(dev.ctlBase+ATA_REG_DEVICE_CONTROL, []byte{ATA_DCR_NIEN}, true)
	if err != nil { t.Fatalf("Write to DeviceControl (set nIEN) failed: %v", err) }
	if !dev.nIEN { t.Error("nIEN should be true after setting DCR nIEN bit") }
}

func TestATAIdentifyDeviceCommand(t *testing.T) {
	dev := newTestATADevice(t)

	// Send IDENTIFY_DEVICE command
	_, err := dev.HandleIO(dev.ioBase+ATA_REG_COMMAND, []byte{ATA_CMD_IDENTIFY_DEVICE}, true)
	if err != nil {
		t.Fatalf("Failed to send IDENTIFY_DEVICE command: %v", err)
	}

	// Check status: BSY should be clear, DRQ should be set (or was briefly BSY then DRQ)
	// Our current implementation sets BSY, then clears BSY and sets DRQ.
	status, _ := dev.HandleIO(dev.ioBase+ATA_REG_STATUS, nil, false)
	if (status & ATA_SR_BSY) != 0 {
		t.Errorf("Status after IDENTIFY_DEVICE: BSY should be clear, got 0x%02X", status)
	}
	if (status & ATA_SR_DRQ) == 0 {
		t.Errorf("Status after IDENTIFY_DEVICE: DRQ should be set, got 0x%02X", status)
	}

	// Read the 512 bytes of IDENTIFY data
	identifyResult := make([]byte, ataIdentifyDeviceSize)
	currentByte := 0
	// Simulate word reads (two byte reads) from Data Port
	// Note: Current HandleIO for Data Port reads byte-by-byte using dev.dataBufferPtr.
	// This test simulates the guest reading byte-by-byte.
	for i := 0; i < ataIdentifyDeviceSize; i++ {
		// KVM_EXIT_IO data slice (passed as `data` to HandleIO) has size 1 for single byte reads.
		// The value read by HandleIO is returned as its first uint8 result.
		readByte, err := dev.HandleIO(dev.ioBase+ATA_REG_DATA, make([]byte, 1), false)
		if err != nil {
			t.Fatalf("Error reading byte %d from Data Port for IDENTIFY: %v", i, err)
		}
		identifyResult[currentByte] = readByte
		currentByte++
	}

	// Verify some parts of the identify data
	// Word 0: General configuration (0x0040)
	// identifyData stores words in little-endian byte order. identifyData[0] is LSB, identifyData[1] is MSB.
	if identifyResult[0] != 0x40 || identifyResult[1] != 0x00 { // Byte 0, Byte 1
		t.Errorf("IDENTIFY Word 0 (General Config): Expected 0x0040, got 0x%02X%02X", identifyResult[1], identifyResult[0])
	}

	// Serial number (Words 10-19, 20 chars) "VA-SN012345678901234"
	// Bytes 20-39.
	// Expected: A V - S N   0 1 2 3 4 ... (after byte swapping)
	// serial[0]='V', serial[1]='A'. identifyData[20]=serial[1]='A', identifyData[21]=serial[0]='V'.
	expectedSerialString := "VA-SN012345678901234"
	expectedSerialBytes := make([]byte, 20)
	for i:=0; i < len(expectedSerialString); i+=2 {
		expectedSerialBytes[i] = expectedSerialString[i+1]
		expectedSerialBytes[i+1] = expectedSerialString[i]
	}

	serialStartInIdentifyResult := identifyResult[10*2 : 10*2+len(expectedSerialString)]
	if !bytes.Equal(serialStartInIdentifyResult, expectedSerialBytes) {
		t.Errorf("IDENTIFY Serial Number (Words 10-19) mismatch.\nExpected: %v (%s)\nGot:      %v (%s)",
			expectedSerialBytes, string(expectedSerialBytes), serialStartInIdentifyResult, string(serialStartInIdentifyResult))
	}

	// Total sectors (LBA28, Words 60-61)
	diskSectors := dev.masterDrive.image.Size() / uint64(dev.masterDrive.image.SectorSize())
	if diskSectors > 0xFFFFFFFF { diskSectors = 0xFFFFFFFF }
	expectedLBA28_0 := byte(diskSectors & 0xFF)
	expectedLBA28_1 := byte((diskSectors >> 8) & 0xFF)
	expectedLBA28_2 := byte((diskSectors >> 16) & 0xFF)
	expectedLBA28_3 := byte((diskSectors >> 24) & 0xFF)

	if identifyResult[60*2+0] != expectedLBA28_0 || identifyResult[60*2+1] != expectedLBA28_1 ||
		identifyResult[61*2+0] != expectedLBA28_2 || identifyResult[61*2+1] != expectedLBA28_3 {
		t.Errorf("IDENTIFY LBA28 Sectors (Words 60-61) mismatch. Expected 0x%08X, got 0x%02X%02X%02X%02X",
			uint32(diskSectors), identifyResult[61*2+1], identifyResult[61*2+0], identifyResult[60*2+1], identifyResult[60*2+0])
	}


	// After reading all data, DRQ should be clear
	statusAfterRead, _ := dev.HandleIO(dev.ioBase+ATA_REG_STATUS, nil, false)
	if (statusAfterRead & ATA_SR_DRQ) != 0 {
		t.Errorf("Status after reading all IDENTIFY data: DRQ should be clear, got 0x%02X", statusAfterRead)
	}
}

// TODO: Add tests for READ_SECTORS command
// TODO: Add tests for WRITE_SECTORS command
// TODO: Add tests for error conditions (e.g., drive not ready, bad LBA)
// TODO: Add tests for interrupt generation (will require mock InterruptRaiser)
