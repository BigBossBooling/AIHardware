package devices

import (
	"bytes"
	"log" // Added import
	"strings"
	"testing"
	"unsafe"
	// "v-architect/core_engine/hypervisor" // No longer needed here
)

func TestSerialPortDevice_HandleIO_Output(t *testing.T) {
	var outputBuf bytes.Buffer
	serial := NewSerialPortDevice(COM1_BASE_ADDR, &outputBuf)

	testChar := byte('A')

	// Simulate kvm_run data area. For an OUT operation, the data is written by the guest.
	// We need to place `testChar` where HandleIO expects it.
	// HandleIO gets a pointer to the start of kvm_run, and an offset to the data.
	// For this test, let's create a small byte array that HandleIO can interpret as the data payload.
	// The actual kvm_run structure is much larger, but HandleIO only cares about the payload at dataOffset.

	// Let's make a fake kvm_run data area. HandleIO will use dataOffset to read from it.
	// If dataOffset is, say, 0 for this test (meaning data is at start of kvmRunDataPtr for payload),
	// then kvmRunDataForTest just needs to contain the byte.
	var kvmRunDataForTest [8]byte // Small buffer, assuming dataOffset will be 0 for this test
	kvmRunDataForTest[0] = testChar // Guest "wrote" 'A'

	kvmRunDataPtr := unsafe.Pointer(&kvmRunDataForTest[0])
	dataOffsetWithinFakeKvmRun := uint64(0)


	// Test writing to DATA register (THR)
	err := serial.HandleIO(COM1_BASE_ADDR+DATA_REG_OFFSET, kvmRunDataPtr, IO_OUT, 1, dataOffsetWithinFakeKvmRun)
	if err != nil {
		t.Fatalf("HandleIO (THR write) failed: %v", err)
	}

	expectedOutput := "A"
	if outputBuf.String() != expectedOutput {
		t.Errorf("Expected output %q, got %q", expectedOutput, outputBuf.String())
	}

	// Test LCR write (DLAB=0)
	outputBuf.Reset()
	newLCR := uint8(0x03) // 8N1
	kvmRunDataForTest[0] = newLCR
	err = serial.HandleIO(COM1_BASE_ADDR+LCR_REG_OFFSET, kvmRunDataPtr, IO_OUT, 1, dataOffsetWithinFakeKvmRun)
	if err != nil {
		t.Fatalf("HandleIO (LCR write) failed: %v", err)
	}
	if serial.lineControlReg != newLCR {
		t.Errorf("LCR not set correctly: expected 0x%02X, got 0x%02X", newLCR, serial.lineControlReg)
	}
	if outputBuf.Len() > 0 { // LCR write shouldn't produce output
		t.Errorf("LCR write produced unexpected output: %q", outputBuf.String())
	}
}

func TestSerialPortDevice_HandleIO_Input(t *testing.T) {
	serial := NewSerialPortDevice(COM1_BASE_ADDR, nil) // No writer needed for input tests

	// Simulate kvm_run data area. For an IN operation, HandleIO writes into this.
	var kvmRunDataForTest [8]byte
	kvmRunDataPtr := unsafe.Pointer(&kvmRunDataForTest[0])
	dataOffsetWithinFakeKvmRun := uint64(0)

	// Test reading LSR
	err := serial.HandleIO(COM1_BASE_ADDR+LSR_REG_OFFSET, kvmRunDataPtr, IO_IN, 1, dataOffsetWithinFakeKvmRun)
	if err != nil {
		t.Fatalf("HandleIO (LSR read) failed: %v", err)
	}

	lsrValue := kvmRunDataForTest[0] // Value written by HandleIO
	expectedLSR := LSR_TRANSMITTER_HOLDING_REG_EMPTY | LSR_TRANSMITTER_EMPTY
	if lsrValue != expectedLSR {
		t.Errorf("Expected LSR 0x%02X, got 0x%02X", expectedLSR, lsrValue)
	}

	// Test reading IIR
	kvmRunDataForTest[0] = 0 // Clear for next read
	err = serial.HandleIO(COM1_BASE_ADDR+IIR_REG_OFFSET, kvmRunDataPtr, IO_IN, 1, dataOffsetWithinFakeKvmRun)
	if err != nil {
		t.Fatalf("HandleIO (IIR read) failed: %v", err)
	}
	iirValue := kvmRunDataForTest[0]
	expectedIIR := IIR_NO_INTERRUPT_PENDING
	if iirValue != expectedIIR {
		t.Errorf("Expected IIR 0x%02X, got 0x%02X", expectedIIR, iirValue)
	}

	// Test reading RHR (Data Register, DLAB=0)
	serial.lineControlReg = 0x03 // Ensure DLAB is 0
	kvmRunDataForTest[0] = 0xFF // Set to non-zero to see it change
	err = serial.HandleIO(COM1_BASE_ADDR+DATA_REG_OFFSET, kvmRunDataPtr, IO_IN, 1, dataOffsetWithinFakeKvmRun)
	if err != nil {
		t.Fatalf("HandleIO (RHR read) failed: %v", err)
	}
	rhrValue := kvmRunDataForTest[0]
	if rhrValue != 0x00 { // Expecting 0 as no input is buffered
		t.Errorf("Expected RHR 0x00 (no input), got 0x%02X", rhrValue)
	}
}

func TestSerialPortDevice_HandleIO_DLAB(t *testing.T) {
	serial := NewSerialPortDevice(COM1_BASE_ADDR, nil)

	var kvmRunDataForTest [8]byte
	kvmRunDataPtr := unsafe.Pointer(&kvmRunDataForTest[0])
	dataOffset := uint64(0)

	// Set DLAB to 1 by writing to LCR
	kvmRunDataForTest[0] = LCR_DLAB | 0x03 // 8N1, DLAB=1
	err := serial.HandleIO(COM1_BASE_ADDR+LCR_REG_OFFSET, kvmRunDataPtr, IO_OUT, 1, dataOffset)
	if err != nil {
		t.Fatalf("Failed to set LCR for DLAB test: %v", err)
	}
	if !serial.isDLABSet() {
		t.Fatal("DLAB bit was not set in LCR")
	}

	// Write to DLL (offset 0 when DLAB=1)
	dllValue := uint8(0x0C) // Example for 9600 baud (115200 / 12)
	kvmRunDataForTest[0] = dllValue
	err = serial.HandleIO(COM1_BASE_ADDR+DATA_REG_OFFSET, kvmRunDataPtr, IO_OUT, 1, dataOffset)
	if err != nil {
		t.Fatalf("HandleIO (DLL write) failed: %v", err)
	}
	// TODO: If we stored DLL, check it here. For now, just ensure no error and log was printed.

	// Read from DLL
	kvmRunDataForTest[0] = 0xFF // Clear for read
	err = serial.HandleIO(COM1_BASE_ADDR+DATA_REG_OFFSET, kvmRunDataPtr, IO_IN, 1, dataOffset)
	if err != nil {
		t.Fatalf("HandleIO (DLL read) failed: %v", err)
	}
	readDll := kvmRunDataForTest[0]
	if readDll != 0x00 { // Currently returns 0x00 as it's not stored
		t.Errorf("Expected DLL read to be 0x00, got 0x%02X", readDll)
	}
}

// Test logger output (optional, more complex)
// This requires capturing log output.
func TestSerialLogging(t *testing.T) {
	var logBuf bytes.Buffer
    originalLogger := log.Writer()
    log.SetOutput(&logBuf)
    defer log.SetOutput(originalLogger) // Restore original logger

	serial := NewSerialPortDevice(COM1_BASE_ADDR, nil)
	var kvmRunDataForTest [1]byte
	kvmRunDataForTest[0] = byte('X')
	kvmRunDataPtr := unsafe.Pointer(&kvmRunDataForTest[0])

	// Write to MCR (Modem Control Register)
	_ = serial.HandleIO(COM1_BASE_ADDR+MCR_REG_OFFSET, kvmRunDataPtr, IO_OUT, 1, 0)

	if !strings.Contains(logBuf.String(), "Serial MCR write") {
		t.Errorf("Expected log output for MCR write, got: %s", logBuf.String())
	}
}
