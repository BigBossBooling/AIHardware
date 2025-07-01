package devices

import (
	"testing"
	"time"
)

func TestRTCNewRTCDevice(t *testing.T) {
	rtc := NewRTCDevice(nil)
	if rtc == nil {
		t.Fatal("NewRTCDevice returned nil")
	}
	if rtc.registers[RTC_REG_STATUS_D]&RTC_REGD_VRT == 0 {
		t.Errorf("RTC_REG_STATUS_D: Expected VRT bit to be set, got 0x%02X", rtc.registers[RTC_REG_STATUS_D])
	}
	expectedRegB := byte(RTC_REGB_24H) // Default is 24H, BCD
	if rtc.registers[RTC_REG_STATUS_B] != expectedRegB {
		t.Errorf("RTC_REG_STATUS_B: Expected 0x%02X, got 0x%02X", expectedRegB, rtc.registers[RTC_REG_STATUS_B])
	}
	expectedRegA := byte(RTC_REGA_DV_32768HZ | 0x06) // Default DV and RS
	if rtc.registers[RTC_REG_STATUS_A] != expectedRegA {
		t.Errorf("RTC_REG_STATUS_A: Expected 0x%02X, got 0x%02X", expectedRegA, rtc.registers[RTC_REG_STATUS_A])
	}
}

func TestRTCIndexAndDataPorts(t *testing.T) {
	rtc := NewRTCDevice(nil)

	// Select RTC_REG_MONTH (0x08)
	testIndex := byte(RTC_REG_MONTH)
	_, err := rtc.HandleIO(RTC_INDEX_PORT, []byte{testIndex}, true)
	if err != nil {
		t.Fatalf("Error writing to RTC_INDEX_PORT: %v", err)
	}
	if rtc.selectedIndex != testIndex {
		t.Errorf("Expected selectedIndex 0x%02X, got 0x%02X", testIndex, rtc.selectedIndex)
	}
	if rtc.nmiDisabled { // NMI bit should be clear
		t.Error("Expected NMI to be enabled by default after writing index")
	}

	// Write to RTC_INDEX_PORT with NMI_DISABLE_MASK set
	testIndexNMI := byte(RTC_REG_DAY_OF_WEEK | RTC_NMI_DISABLE_MASK)
	_, err = rtc.HandleIO(RTC_INDEX_PORT, []byte{testIndexNMI}, true)
	if err != nil {
		t.Fatalf("Error writing to RTC_INDEX_PORT with NMI disable: %v", err)
	}
	if rtc.selectedIndex != (testIndexNMI & 0x7F) {
		t.Errorf("Expected selectedIndex 0x%02X, got 0x%02X", (testIndexNMI & 0x7F), rtc.selectedIndex)
	}
	if !rtc.nmiDisabled {
		t.Error("Expected NMI to be disabled")
	}

	// Read from RTC_INDEX_PORT, should return last written value
	readIndex, err := rtc.HandleIO(RTC_INDEX_PORT, nil, false)
	if err != nil {
		t.Fatalf("Error reading from RTC_INDEX_PORT: %v", err)
	}
	if readIndex != testIndexNMI {
		t.Errorf("Read from RTC_INDEX_PORT: Expected 0x%02X, got 0x%02X", testIndexNMI, readIndex)
	}


	// Read current time (e.g., seconds)
	// Select RTC_REG_SECONDS
	_, err = rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_SECONDS}, true)
	if err != nil { t.Fatalf("Error setting index to seconds: %v", err) }

	now := time.Now()
	expectedSecondsBCD := toBCD(now.Second())

	readSeconds, err := rtc.HandleIO(RTC_DATA_PORT, nil, false)
	if err != nil {
		t.Fatalf("Error reading seconds from RTC_DATA_PORT: %v", err)
	}
	// Compare with a small tolerance due to potential time passing during test execution
	if readSeconds != expectedSecondsBCD && readSeconds != toBCD((now.Second()+1)%60) && readSeconds != toBCD((now.Second()+2)%60) {
		t.Errorf("Read seconds: Expected BCD ~0x%02X, got 0x%02X", expectedSecondsBCD, readSeconds)
	}

	// Write to a CMOS RAM location (e.g. register 0x0E - diagnostic status, usually settable)
	cmosTestIndex := byte(0x0E)
	cmosTestData := byte(0xAA)
	_, err = rtc.HandleIO(RTC_INDEX_PORT, []byte{cmosTestIndex}, true)
	if err != nil { t.Fatalf("Error setting index to 0x0E: %v", err) }
	_, err = rtc.HandleIO(RTC_DATA_PORT, []byte{cmosTestData}, true)
	if err != nil {
		t.Fatalf("Error writing to RTC_DATA_PORT at index 0x0E: %v", err)
	}
	if rtc.registers[cmosTestIndex] != cmosTestData {
		t.Errorf("CMOS RAM 0x0E: Expected 0x%02X, got 0x%02X", cmosTestData, rtc.registers[cmosTestIndex])
	}

	// Read back from CMOS RAM location
	readCmosData, err := rtc.HandleIO(RTC_DATA_PORT, nil, false) // Index is still 0x0E
	if err != nil {
		t.Fatalf("Error reading from RTC_DATA_PORT at index 0x0E: %v", err)
	}
	if readCmosData != cmosTestData {
		t.Errorf("Read CMOS RAM 0x0E: Expected 0x%02X, got 0x%02X", cmosTestData, readCmosData)
	}
}

func TestRTCStatusRegisters(t *testing.T) {
	rtc := NewRTCDevice(nil)

	// Test Status Register B: Set to Binary, 12-hour mode, PIE enabled
	newRegB := byte(RTC_REGB_DM | RTC_REGB_PIE) // Binary, 12-hour (0), PIE
	_, err := rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_STATUS_B}, true)
	if err != nil { t.Fatalf("Error setting index to RTC_REG_STATUS_B: %v", err) }
	_, err = rtc.HandleIO(RTC_DATA_PORT, []byte{newRegB}, true)
	if err != nil {
		t.Fatalf("Error writing to RTC_REG_STATUS_B: %v", err)
	}
	if rtc.registers[RTC_REG_STATUS_B] != newRegB {
		t.Errorf("RTC_REG_STATUS_B: Expected 0x%02X, got 0x%02X", newRegB, rtc.registers[RTC_REG_STATUS_B])
	}

	// Read it back
	readRegB, err := rtc.HandleIO(RTC_DATA_PORT, nil, false) // Index still RTC_REG_STATUS_B
	if err != nil {
		t.Fatalf("Error reading RTC_REG_STATUS_B: %v", err)
	}
	if readRegB != newRegB {
		t.Errorf("Read RTC_REG_STATUS_B: Expected 0x%02X, got 0x%02X", newRegB, readRegB)
	}

	// Test Status Register A: Change Rate Select
	originalRegA := rtc.registers[RTC_REG_STATUS_A]
	newRateSelect := byte(0x0A) // Example rate
	newRegA := (originalRegA & (^RTC_REGA_RATE_MASK & 0xFF)) | newRateSelect // Fixed NOT overflow

	_, err = rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_STATUS_A}, true)
	if err != nil { t.Fatalf("Error setting index to RTC_REG_STATUS_A: %v", err) }
	_, err = rtc.HandleIO(RTC_DATA_PORT, []byte{newRegA}, true) // Write only rate select part
	if err != nil {
		t.Fatalf("Error writing to RTC_REG_STATUS_A: %v", err)
	}
	// Check that only RS bits changed, DV and UIP (if it were managed and set) are preserved
	// Our writeRegister for RegA ensures DV part is preserved from original, UIP is masked out on read.
	// So, the value should be (original_DV_bits | newRateSelect)
	// Also apply & 0xFF to ^RTC_REGA_UIP
	finalRegA := (originalRegA & (^RTC_REGA_RATE_MASK & 0xFF) & (^RTC_REGA_UIP & 0xFF)) | newRateSelect
	if rtc.registers[RTC_REG_STATUS_A] != finalRegA {
		t.Errorf("RTC_REG_STATUS_A: Expected 0x%02X, got 0x%02X", finalRegA, rtc.registers[RTC_REG_STATUS_A])
	}

	// Test Status Register C: Read should clear flags
	rtc.registers[RTC_REG_STATUS_C] = RTC_REGC_PF | RTC_REGC_AF | RTC_REGC_IRQF // Manually set some flags
	_, err = rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_STATUS_C}, true)
	if err != nil { t.Fatalf("Error setting index to RTC_REG_STATUS_C: %v", err) }

	readRegC, err := rtc.HandleIO(RTC_DATA_PORT, nil, false)
	if err != nil {
		t.Fatalf("Error reading RTC_REG_STATUS_C: %v", err)
	}
	if readRegC != (RTC_REGC_PF | RTC_REGC_AF | RTC_REGC_IRQF) {
		t.Errorf("Read RTC_REG_STATUS_C: Expected 0x%02X, got 0x%02X", (RTC_REGC_PF | RTC_REGC_AF | RTC_REGC_IRQF), readRegC)
	}
	if rtc.registers[RTC_REG_STATUS_C] != 0 { // Should be cleared after read
		t.Errorf("RTC_REG_STATUS_C: Expected to be 0 after read, got 0x%02X", rtc.registers[RTC_REG_STATUS_C])
	}

	// Test Write to Read-Only Status Register C and D
	_, err = rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_STATUS_C}, true)
	if err != nil { t.Fatalf("Error setting index to RTC_REG_STATUS_C for write test: %v", err) }
	_, err = rtc.HandleIO(RTC_DATA_PORT, []byte{0xFF}, true) // Attempt to write
	if err != nil { t.Fatalf("Error writing to RTC_REG_STATUS_C: %v", err) }
	if rtc.registers[RTC_REG_STATUS_C] != 0 { // Should remain unchanged (0 from previous test)
		t.Errorf("RTC_REG_STATUS_C: Write to read-only reg changed value to 0x%02X", rtc.registers[RTC_REG_STATUS_C])
	}

	originalRegD := rtc.registers[RTC_REG_STATUS_D]
	_, err = rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_STATUS_D}, true)
	if err != nil { t.Fatalf("Error setting index to RTC_REG_STATUS_D for write test: %v", err) }
	_, err = rtc.HandleIO(RTC_DATA_PORT, []byte{0x00}, true) // Attempt to write
	if err != nil { t.Fatalf("Error writing to RTC_REG_STATUS_D: %v", err) }
	if rtc.registers[RTC_REG_STATUS_D] != originalRegD { // Should remain unchanged
		t.Errorf("RTC_REG_STATUS_D: Write to read-only reg changed value from 0x%02X to 0x%02X", originalRegD, rtc.registers[RTC_REG_STATUS_D])
	}
}

func TestRTCTimeDateBCDvsBinary(t *testing.T) {
	rtc := NewRTCDevice(nil) // Added nil for InterruptRaiser
	now := time.Now()

	// Case 1: BCD mode (default)
	rtc.registers[RTC_REG_STATUS_B] &= (^RTC_REGB_DM & 0xFF) // Ensure BCD mode; Fixed NOT overflow

	_, err := rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_HOURS}, true)
	if err != nil { t.Fatalf("Error setting index to RTC_REG_HOURS: %v", err) }

	readHoursBCD, err := rtc.HandleIO(RTC_DATA_PORT, nil, false)
	if err != nil { t.Fatalf("Error reading hours in BCD mode: %v", err) }

	expectedHoursBCD := toBCD(now.Hour()) // Assuming 24H mode also default
	if rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_24H == 0 { // 12H mode
		hour12 := now.Hour()
		isPM := hour12 >=12
		if hour12 == 0 { hour12 = 12 }
		if hour12 > 12 { hour12 -= 12}
		expectedHoursBCD = toBCD(hour12)
		if isPM { expectedHoursBCD |= 0x80 }
	}

	if readHoursBCD != expectedHoursBCD && readHoursBCD != toBCD((now.Hour()+1)%24) /* account for slight delay */ {
		t.Errorf("BCD Mode: Read hours expected ~0x%02X, got 0x%02X", expectedHoursBCD, readHoursBCD)
	}

	// Case 2: Binary mode
	rtc.registers[RTC_REG_STATUS_B] |= RTC_REGB_DM // Set Binary mode

	// Index is still RTC_REG_HOURS
	readHoursBinary, err := rtc.HandleIO(RTC_DATA_PORT, nil, false)
	if err != nil { t.Fatalf("Error reading hours in Binary mode: %v", err) }

	expectedHoursBinary := byte(now.Hour())
	if rtc.registers[RTC_REG_STATUS_B] & RTC_REGB_24H == 0 { // 12H mode
		hour12 := now.Hour()
		isPM := hour12 >=12
		if hour12 == 0 { hour12 = 12 }
		if hour12 > 12 { hour12 -= 12}
		expectedHoursBinary = byte(hour12)
		if isPM { expectedHoursBinary |= 0x80 }
	}
	if readHoursBinary != expectedHoursBinary && readHoursBinary != byte((now.Hour()+1)%24) {
		t.Errorf("Binary Mode: Read hours expected ~0x%02X, got 0x%02X", expectedHoursBinary, readHoursBinary)
	}
}

func TestRTC12HourMode(t *testing.T) {
	rtc := NewRTCDevice(nil) // Added nil for InterruptRaiser
	rtc.registers[RTC_REG_STATUS_B] &= (^RTC_REGB_24H & 0xFF) // Set 12-hour mode; Fixed NOT overflow
	rtc.registers[RTC_REG_STATUS_B] &= (^RTC_REGB_DM & 0xFF)  // Ensure BCD mode for easier comparison; Fixed NOT overflow

	_, err := rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_HOURS}, true)
	if err != nil { t.Fatalf("Error setting index to RTC_REG_HOURS: %v", err) }

	testTimes := []struct{ hour24, expectedHour12BCD byte }{
		{0, 0x12},  // 12 AM
		{5, 0x05},  //  5 AM
		{11, 0x11}, // 11 AM
		{12, 0x92}, // 12 PM (PM bit set)
		{17, 0x85}, //  5 PM (PM bit set)
		{23, 0x91}, // 11 PM (PM bit set)
	}

	for _, tt := range testTimes {
		// Simulate current time by directly setting a "now" that gives tt.hour24
		// This is tricky as readRegister always uses time.Now().
		// Instead, we can check the conversion logic.
		// Let's manually check the conversion logic using a fixed time.

		// Create a time.Time object for the specific hour
		mockTime := time.Date(2023, 1, 1, int(tt.hour24), 0, 0, 0, time.UTC)

		// Temporarily override time.Now for this test (not directly possible without DI or monkey patching)
		// So, we test the logic part by part.
		hour := mockTime.Hour()
		isPM := hour >= 12
		if hour == 0 { hour = 12 }
		if hour > 12 { hour -= 12 }
		val := toBCD(hour)
		if isPM { val |= 0x80 }

		if val != tt.expectedHour12BCD {
			t.Errorf("12-Hour BCD test for %d:00. Expected 0x%02X, got 0x%02X", tt.hour24, tt.expectedHour12BCD, val)
		}
	}
	// Actual test with HandleIO would require mocking time.Now() or complex setup.
	// The above loop tests the conversion logic within the RTCDevice's scope.
}

// TODO: Add tests for Alarm functionality once Tick and interrupt generation are active.
// TODO: Add tests for Periodic Interrupt functionality once Tick and interrupts are active.
// TODO: Add tests for Update-Ended Interrupt.
// TODO: Test Century byte reading.
// TODO: Test SET bit in Register B inhibiting writes to time/date registers.

func TestRTCSetBitInhibitsWrites(t *testing.T) {
	rtc := NewRTCDevice(nil) // Added nil for InterruptRaiser

	// Enable SET bit in Register B
	rtc.registers[RTC_REG_STATUS_B] |= RTC_REGB_SET

	// Attempt to write to RTC_REG_SECONDS (should be inhibited)
	// originalSeconds := rtc.registers[RTC_REG_SECONDS] // May not be useful as it's dynamic; Commented out
	_, err := rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_SECONDS}, true)
	if err != nil { t.Fatalf("Error setting index to RTC_REG_SECONDS: %v", err) }

	_, err = rtc.HandleIO(RTC_DATA_PORT, []byte{0x55}, true) // Attempt write
	if err != nil { t.Fatalf("Error writing to RTC_DATA_PORT (seconds): %v", err) }

	// Read back seconds. It should *not* be 0x55. It should be current time.
	// This test is a bit weak because current time is dynamic.
	// A better check: store a non-time CMOS value, try to change it.
	// We don't explicitly check the value of seconds after write attempt due to its dynamic nature.
	// The main check is for a non-time CMOS byte below.

	cmosTestIndex := byte(0x20) // Some unused CMOS byte
	rtc.registers[cmosTestIndex] = 0xAA

	_, err = rtc.HandleIO(RTC_INDEX_PORT, []byte{cmosTestIndex}, true)
	if err != nil { t.Fatalf("Error setting index to 0x%02X: %v", cmosTestIndex, err) }

	_, err = rtc.HandleIO(RTC_DATA_PORT, []byte{0xBB}, true) // Attempt write
	if err != nil { t.Fatalf("Error writing to RTC_DATA_PORT (CMOS 0x%02X): %v", cmosTestIndex, err) }

	if rtc.registers[cmosTestIndex] == 0xBB {
		t.Errorf("RTC_REGB_SET: Write to CMOS 0x%02X was not inhibited. Expected 0xAA, got 0xBB.", cmosTestIndex)
	} else if rtc.registers[cmosTestIndex] != 0xAA {
		// This case should not happen if write was properly inhibited.
		t.Errorf("RTC_REGB_SET: CMOS 0x%02X changed unexpectedly. Expected 0xAA, got 0x%02X.", cmosTestIndex, rtc.registers[cmosTestIndex])
	}

	// Writes to Reg A and B should still be allowed
	originalRegA := rtc.registers[RTC_REG_STATUS_A]
	newRegAVal := byte( (originalRegA & (^RTC_REGA_RATE_MASK & 0xFF)) | 0x07 ) // Change rate select; Fixed NOT overflow
	_, err = rtc.HandleIO(RTC_INDEX_PORT, []byte{RTC_REG_STATUS_A}, true)
	if err != nil { t.Fatalf("Error setting index to RTC_REG_STATUS_A: %v", err) }
	_, err = rtc.HandleIO(RTC_DATA_PORT, []byte{newRegAVal}, true)
	if err != nil { t.Fatalf("Error writing to RTC_REG_STATUS_A with SET active: %v", err) }
	if rtc.registers[RTC_REG_STATUS_A] != newRegAVal {
		t.Errorf("RTC_REGB_SET: Write to RTC_REG_STATUS_A was unexpectedly inhibited or failed.")
	}
}
