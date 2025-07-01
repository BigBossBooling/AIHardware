package devices

import (
	"testing"
)

func TestPITNewPITDevice(t *testing.T) {
	pit := NewPITDevice(nil) // Added nil for InterruptRaiser
	if pit == nil {
		t.Fatal("NewPITDevice returned nil")
	}
	for i, ch := range pit.channels {
		if ch.mode != PIT_CMD_MODE0 {
			t.Errorf("Channel %d: Expected mode %X, got %X", i, PIT_CMD_MODE0, ch.mode)
		}
		if ch.accessMode != PIT_CMD_RW_LSB_MSB { // Renamed PIT_CMD_ACCESS_LOHI
			t.Errorf("Channel %d: Expected accessMode %X, got %X", i, PIT_CMD_RW_LSB_MSB, ch.accessMode)
		}
		expectedGate := i < 2
		if ch.gate != expectedGate {
			t.Errorf("Channel %d: Expected gate %v, got %v", i, expectedGate, ch.gate)
		}
	}
}

func TestPITCommandRegisterWrite(t *testing.T) {
	pit := NewPITDevice(nil) // Added nil for InterruptRaiser

	// Program Channel 0, Mode 3, LSB/MSB access, Binary
	// Binary mode is implicit if PIT_CMD_BCD_MASK is not set in the command byte.
	cmdCh0Mode3 := byte(PIT_CMD_CHANNEL0 | PIT_CMD_RW_LSB_MSB | PIT_CMD_MODE3) // Removed PIT_CMD_BINARY
	_, err := pit.HandleIO(PIT_COMMAND_REG, []byte{cmdCh0Mode3}, true) // Renamed PIT_COMMAND_PORT
	if err != nil {
		t.Fatalf("Error writing command to PIT: %v", err)
	}

	ch0 := &pit.channels[0]
	if ch0.mode != PIT_CMD_MODE3 {
		t.Errorf("Ch0: Expected mode %X, got %X", PIT_CMD_MODE3, ch0.mode)
	}
	if ch0.accessMode != PIT_CMD_RW_LSB_MSB { // Renamed PIT_CMD_ACCESS_LOHI
		t.Errorf("Ch0: Expected accessMode %X, got %X", PIT_CMD_RW_LSB_MSB, ch0.accessMode)
	}
	if ch0.bcdMode != false { // This check is correct as bcdMode is false for binary
		t.Errorf("Ch0: Expected binary mode, got BCD")
	}
	if !ch0.output { // Mode 3 output should be high after programming
		t.Errorf("Ch0: Expected output to be high after Mode 3 programming, got low")
	}

	// Latch Channel 1 counter
	cmdLatchCh1 := byte(PIT_CMD_CHANNEL1 | PIT_CMD_RW_LATCH) // Renamed PIT_CMD_LATCH_COUNT
	_, err = pit.HandleIO(PIT_COMMAND_REG, []byte{cmdLatchCh1}, true) // Renamed PIT_COMMAND_PORT
	if err != nil {
		t.Fatalf("Error writing latch command to PIT: %v", err)
	}
	ch1 := &pit.channels[1]
	if !ch1.countLatched {
		t.Errorf("Ch1: Expected countLatched to be true, got false")
	}
}

func TestPITCounterReadWrite(t *testing.T) {
	pit := NewPITDevice(nil) // Added nil for InterruptRaiser
	ch0 := &pit.channels[0]

	// Program Channel 0, Mode 2, LSB/MSB access, Binary
	cmdCh0Mode2 := byte(PIT_CMD_CHANNEL0 | PIT_CMD_RW_LSB_MSB | PIT_CMD_MODE2) // Implicitly binary
	_, err := pit.HandleIO(PIT_COMMAND_REG, []byte{cmdCh0Mode2}, true) // Renamed constants
	if err != nil {
		t.Fatalf("Error programming Ch0 for read/write test: %v", err)
	}

	// Write 0x1234 to Channel 0 counter
	valLSB := byte(0x34)
	valMSB := byte(0x12)

	_, err = pit.HandleIO(PIT_CHANNEL0_DATA, []byte{valLSB}, true) // Write LSB; Renamed constant
	if err != nil {
		t.Fatalf("Error writing LSB to Ch0: %v", err)
	}
	if ch0.writeState != 1 {
		t.Errorf("Ch0: Expected writeState 1 after LSB write, got %d", ch0.writeState)
	}
	if ch0.reloadValue != 0x0034 {
		t.Errorf("Ch0: Expected reloadValue 0x0034 after LSB, got 0x%04X", ch0.reloadValue)
	}


	_, err = pit.HandleIO(PIT_CHANNEL0_DATA, []byte{valMSB}, true) // Write MSB; Renamed constant
	if err != nil {
		t.Fatalf("Error writing MSB to Ch0: %v", err)
	}
	if ch0.writeState != 0 {
		t.Errorf("Ch0: Expected writeState 0 after MSB write, got %d", ch0.writeState)
	}
	if ch0.reloadValue != 0x1234 {
		t.Errorf("Ch0: Expected reloadValue 0x1234 after MSB, got 0x%04X", ch0.reloadValue)
	}
	if ch0.count != 0x1234 {
		t.Errorf("Ch0: Expected count 0x1234 after load, got 0x%04X", ch0.count)
	}
	if ch0.nullCount {
		t.Error("Ch0: Expected nullCount to be false after load")
	}


	// Latch Channel 0 counter before reading
	cmdLatchCh0 := byte(PIT_CMD_CHANNEL0 | PIT_CMD_RW_LATCH) // Renamed constant
	_, err = pit.HandleIO(PIT_COMMAND_REG, []byte{cmdLatchCh0}, true) // Renamed constant
	if err != nil {
		t.Fatalf("Error latching Ch0 for read: %v", err)
	}
	if !ch0.countLatched {
		t.Errorf("Ch0: Expected countLatched true after latch cmd, got false")
	}
	if ch0.latchedValue != ch0.count { // Assuming count hasn't changed
		t.Errorf("Ch0: Latched value 0x%04X does not match current count 0x%04X", ch0.latchedValue, ch0.count)
	}


	// Read back LSB
	readLSB, err := pit.HandleIO(PIT_CHANNEL0_DATA, nil, false) // Renamed constant
	if err != nil {
		t.Fatalf("Error reading LSB from Ch0: %v", err)
	}
	if readLSB != valLSB {
		t.Errorf("Ch0: Read LSB expected 0x%02X, got 0x%02X", valLSB, readLSB)
	}
	if ch0.readState != 1 {
		t.Errorf("Ch0: Expected readState 1 after LSB read, got %d", ch0.readState)
	}
	if !ch0.countLatched { // Latch should still be active for MSB read
		t.Errorf("Ch0: Expected countLatched true after LSB read, got false")
	}

	// Read back MSB
	readMSB, err := pit.HandleIO(PIT_CHANNEL0_DATA, nil, false) // Renamed constant
	if err != nil {
		t.Fatalf("Error reading MSB from Ch0: %v", err)
	}
	if readMSB != valMSB {
		t.Errorf("Ch0: Read MSB expected 0x%02X, got 0x%02X", valMSB, readMSB)
	}
	if ch0.readState != 0 {
		t.Errorf("Ch0: Expected readState 0 after MSB read, got %d", ch0.readState)
	}
	if ch0.countLatched { // Latch should be consumed after full LSB/MSB read
		t.Errorf("Ch0: Expected countLatched false after MSB read, got true")
	}
}

func TestPITReadBackCommand(t *testing.T) {
	pit := NewPITDevice(nil) // Added nil for InterruptRaiser

	// Program Channel 0, Mode 3, LSB/MSB access, BCD
	cmdCh0Prog := byte(PIT_CMD_CHANNEL0 | PIT_CMD_RW_LSB_MSB | PIT_CMD_MODE3 | PIT_CMD_BCD_MASK) // Use BCD_MASK
	_, err := pit.HandleIO(PIT_COMMAND_REG, []byte{cmdCh0Prog}, true) // Renamed constants
	if err != nil { t.Fatalf("Cmd write failed: %v", err) }

	// Load count 0x1234 (BCD representation if BCD mode was fully working for value, but count is stored binary)
	// For this test, actual value doesn't matter as much as status bits.
	// PIT stores reloadValue and count as binary even if BCD mode is set. BCD affects interpretation at I/O.
	_, err = pit.HandleIO(PIT_CHANNEL0_DATA, []byte{0x34}, true) // LSB; Renamed constant
	if err != nil { t.Fatalf("LSB write failed: %v", err) }
	_, err = pit.HandleIO(PIT_CHANNEL0_DATA, []byte{0x12}, true) // MSB; Renamed constant
	if err != nil { t.Fatalf("MSB write failed: %v", err) }

	// Set output pin state for testing read-back (not dynamically changed by Tick in this test)
	pit.channels[0].outputPin = true
	pit.channels[0].nullCount = false // Explicitly set as count is loaded


	// Read-back command for Channel 0, latch count and status
	// To latch count and status, PIT_RB_DONT_LATCH_COUNT and PIT_RB_DONT_LATCH_STATUS should be 0.
	// The command is 11SC élus, where S are channel selects, C is count latch bit (0=latch), U is status latch bit (0=latch)
	// So, if PIT_RB_DONT_LATCH_COUNT is bit 5 (1<<5), to latch count, this bit in cmd must be 0.
	// The command structure is: 11 <CH2> <CH1> <CH0> <!COUNT> <!STATUS> <0>
	// Our constants are defined as the bit itself. So, for latching, these bits should NOT be set in cmd.
	rbCmd := byte( (0x03 << 6) | PIT_RB_CHANNEL0) // This means 11000010 - Latch count & status for Ch0.
	// Bits 7,6 are 1,1 for readback. Bit 0 is reserved (0).
	// Bit 1 selects CH0. Bit 4 (status) and Bit 5 (count) are 0 to latch.

	_, err = pit.HandleIO(PIT_COMMAND_REG, []byte{rbCmd}, true) // Renamed constant
	if err != nil {
		t.Fatalf("Error writing read-back command: %v", err)
	}

	ch0 := &pit.channels[0]
	if !ch0.statusLatched {
		t.Errorf("Ch0: Expected statusLatched true after read-back, got false")
	}
	if !ch0.countLatched { // Count should also be latched by this read-back command
		t.Errorf("Ch0: Expected countLatched true after read-back, got false")
	}

	// Expected status byte: [OUT | NULL_COUNT | RW_MODE(2) | MODE(3) | BCD]
	// OUT = 1 (set manually), NULL_COUNT = 0 (loaded), RW_MODE = LOHI (0x30), MODE = 3 (0x06), BCD = 1 (0x01)
	expectedStatus := byte(0x00)
	if ch0.outputPin { expectedStatus |= 0x80 }
	if ch0.nullCount { expectedStatus |= 0x40 } // Should be false
	expectedStatus |= (ch0.accessMode & 0x30) // Use actual programmed access mode
	expectedStatus |= (ch0.mode & 0x0E)       // Use actual programmed op mode
	if ch0.bcdMode { expectedStatus |= PIT_CMD_BCD_MASK } // Use BCD_MASK


	if ch0.latchedStatus != expectedStatus {
		t.Errorf("Ch0: Latched status error. Expected 0x%02X, got 0x%02X", expectedStatus, ch0.latchedStatus)
		t.Logf("Breakdown: OUT=%v, NULL=%v, RW_MODE=0x%X, MODE=0x%X, BCD=%v",
			(expectedStatus&0x80) != 0, (expectedStatus&0x40) != 0,
			(expectedStatus&0x30), (expectedStatus&0x0E), (expectedStatus&0x01) != 0)
		t.Logf("Actual ch0 state: outputPin=%v, nullCount=%v, accessMode=0x%X, mode=0x%X, bcdMode=%v",
			ch0.outputPin, ch0.nullCount, ch0.accessMode, ch0.mode, ch0.bcdMode)
	}

	// Reading the counter port after a read-back that latches status might return status.
	// This behavior is complex and varies. For now, our HandleCounterRead prioritizes latched count.
	// A more accurate test would verify if status is read via counter port under specific conditions.
}

func TestPITPort61Handling(t *testing.T) {
	pit := NewPITDevice(nil) // Added nil for InterruptRaiser

	// Write to Port 0x61
	writeData := byte(0x03) // Example: Speaker data enable, PIT Channel 2 Gate to Speaker enable
	_, err := pit.HandleIO(SYSTEM_CONTROL_PORT_B, []byte{writeData}, true)
	if err != nil {
		t.Fatalf("Error writing to Port 0x61: %v", err)
	}
	if pit.port61State != writeData {
		t.Errorf("Expected port61State 0x%02X, got 0x%02X", writeData, pit.port61State)
	}

	// Read from Port 0x61
	readData, err := pit.HandleIO(SYSTEM_CONTROL_PORT_B, nil, false)
	if err != nil {
		t.Fatalf("Error reading from Port 0x61: %v", err)
	}
	if readData != writeData {
		t.Errorf("Read from Port 0x61: Expected 0x%02X, got 0x%02X", writeData, readData)
	}
}

// TODO: Add tests for BCD mode counting behavior if fully implemented.
// TODO: Add tests for gate functionality once PIT Tick simulation is active.
// TODO: Add tests for output states in different modes once PIT Tick is active.
// TODO: Add tests for read-back of status when status is latched and read via counter port.
//       This requires HandleCounterRead to prioritize status read if statusLatched is true
//       under certain conditions (e.g., if count was not also latched by the same RB command).
//       The current HandleCounterRead always prioritizes latched count.
//       A typical sequence: RB latches status -> read counter port -> returns status byte.
//       Then subsequent reads return counter value (if count also latched or read normally).

// Helper to check specific bits in status for TestPITReadBackCommand breakdown
// t.Logf("Details: outputPin=%v, nullCountRead=%v, accessModeRead=0x%X, modeRead=0x%X, bcdModeRead=%v",
// (ch0.latchedStatus & 0x80) != 0,
// (ch0.latchedStatus & 0x40) != 0,
// (ch0.latchedStatus & 0x30),
// (ch0.latchedStatus & 0x0E),
// (ch0.latchedStatus & 0x01) != 0,
// )
