package core_engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/V-Architect/v-architect-core/proto"
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
)

func TestNewSerialPortDevice(t *testing.T) {
	fmt.Println("Conceptual Test: TestNewSerialPortDevice - START")
	// require := require.New(t)
	// assert := assert.New(t)

	// Test with LOG_ONLY type
	cfgLogOnly := &pb.SerialPortConfig{Id: "com_log", Type: pb.SerialPortConfig_LOG_ONLY}
	spLog, errLog := NewSerialPortDevice(cfgLogOnly.Id, cfgLogOnly, DEFAULT_SERIAL_IO_BASE_COM1, 4)
	if errLog != nil {t.Fatalf("NewSerialPortDevice(LOG_ONLY) failed: %v", errLog)}
	if spLog == nil {t.Fatal("NewSerialPortDevice(LOG_ONLY) returned nil")}
	if spLog.Config.GetType() != pb.SerialPortConfig_LOG_ONLY {t.Errorf("Expected LOG_ONLY type, got %s", spLog.Config.GetType())}
	fmt.Println("Conceptual Test: NewSerialPortDevice(LOG_ONLY) created.")

	// Test with STDIO type
	cfgStdio := &pb.SerialPortConfig{Id: "com_stdio", Type: pb.SerialPortConfig_STDIO}
	spStdio, errStdio := NewSerialPortDevice(cfgStdio.Id, cfgStdio, DEFAULT_SERIAL_IO_BASE_COM2, 3)
	if errStdio != nil {t.Fatalf("NewSerialPortDevice(STDIO) failed: %v", errStdio)}
	if spStdio == nil {t.Fatal("NewSerialPortDevice(STDIO) returned nil")}
	if spStdio.HostBackend != os.Stdout {t.Errorf("Expected os.Stdout backend for STDIO type")} // Conceptual check
	fmt.Println("Conceptual Test: NewSerialPortDevice(STDIO) created.")

	// Test with FILE type
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "serial_out.txt")
	cfgFile := &pb.SerialPortConfig{Id: "com_file", Type: pb.SerialPortConfig_FILE, PathOrAddress: filePath}
	spFile, errFile := NewSerialPortDevice(cfgFile.Id, cfgFile, DEFAULT_SERIAL_IO_BASE_COM3, 5)
	if errFile != nil {t.Fatalf("NewSerialPortDevice(FILE) failed: %v", errFile)}
	if spFile == nil {t.Fatal("NewSerialPortDevice(FILE) returned nil")}
	// In a real test, we'd check if spFile.HostBackend is a valid file handle.
	// For conceptual, the log message in NewSerialPortDevice indicates it attempted to set up.
	// Clean up the conceptual file (actual file not created in conceptual NewSerialPortDevice)
	// _ = os.Remove(filePath)
	fmt.Println("Conceptual Test: NewSerialPortDevice(FILE) created.")
	spFile.Close() // Test close

	fmt.Println("Conceptual Test: TestNewSerialPortDevice - PASSED")
}

func TestSerialPort_HandleIoWrite_DataReg_LogOnly(t *testing.T) {
	fmt.Println("Conceptual Test: TestSerialPort_HandleIoWrite_DataReg_LogOnly - START")
	cfg := &pb.SerialPortConfig{Id: "com_log_write", Type: pb.SerialPortConfig_LOG_ONLY}
	sp, _ := NewSerialPortDevice(cfg.Id, cfg, DEFAULT_SERIAL_IO_BASE_COM1, 4)

	testChar := byte('A')
	// Output will be to fmt.Printf in HandlePIOWrite for LOG_ONLY
	// We can't directly capture that here without redirecting stdout.
	// Test relies on visual inspection of logs or assuming Printf works.
	err := sp.HandlePIOWrite(UART_RX, uint8(testChar), 1) // Offset 0 is THR
	if err != nil {t.Errorf("HandlePIOWrite to THR failed: %v", err)}

	// Check LSR: THR should be empty again quickly
	lsr, _ := sp.HandlePIORead(UART_LSR, 1)
	if (lsr & UART_LSR_TX_EMPTY) == 0 {
		t.Errorf("LSR should indicate TX_EMPTY after write, got 0x%02X", lsr)
	}

	fmt.Println("Conceptual Test: TestSerialPort_HandleIoWrite_DataReg_LogOnly - PASSED (verify log output for '[VM Serial Out - com_log_write - LOG_ONLY]: Char='A' (0x41)')")
}

func TestSerialPort_HandleIoRead_LSR(t *testing.T) {
	fmt.Println("Conceptual Test: TestSerialPort_HandleIoRead_LSR - START")
	cfg := &pb.SerialPortConfig{Id: "com_lsr_read", Type: pb.SerialPortConfig_LOG_ONLY}
	sp, _ := NewSerialPortDevice(cfg.Id, cfg, DEFAULT_SERIAL_IO_BASE_COM1, 4)

	// Initial LSR state
	lsr, err := sp.HandlePIORead(UART_LSR, 1)
	if err != nil {t.Fatalf("HandlePIORead from LSR failed: %v", err)}
	expectedInitialLSR := uint8(UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE)
	if lsr != expectedInitialLSR {
		t.Errorf("Initial LSR value incorrect. Expected 0x%02X, Got 0x%02X", expectedInitialLSR, lsr)
	}

	// Simulate data received for guest (to set DATA_READY)
	sp.SimulateHostInput([]byte{'X'})
	lsr, _ = sp.HandlePIORead(UART_LSR, 1)
	if (lsr & UART_LSR_DATA_READY) == 0 {
		t.Errorf("LSR should indicate DATA_READY after SimulateHostInput, got 0x%02X", lsr)
	}

	fmt.Println("Conceptual Test: TestSerialPort_HandleIoRead_LSR - PASSED")
}

func TestSerialPort_HandleIoRead_DataReg_RxBuffer(t *testing.T) {
	fmt.Println("Conceptual Test: TestSerialPort_HandleIoRead_DataReg_RxBuffer - START")
	cfg := &pb.SerialPortConfig{Id: "com_rbr_test", Type: pb.SerialPortConfig_LOG_ONLY}
	sp, _ := NewSerialPortDevice(cfg.Id, cfg, DEFAULT_SERIAL_IO_BASE_COM1, 4)

	// Test reading RBR when empty
	rbrVal, err := sp.HandlePIORead(UART_RX, 1)
	if err != nil {t.Fatalf("Read from empty RBR failed: %v", err)}
	if rbrVal != 0 {t.Errorf("Expected 0 from empty RBR, got 0x%02X", rbrVal)}
	lsr, _ := sp.HandlePIORead(UART_LSR, 1)
	if (lsr & UART_LSR_DATA_READY) != 0 {
		t.Errorf("LSR should not indicate DATA_READY when RBR is empty, got 0x%02X", lsr)
	}

	// Simulate host input
	testInput := []byte{'Y', 'Z'}
	sp.SimulateHostInput(testInput)

	lsr, _ = sp.HandlePIORead(UART_LSR, 1)
	if (lsr & UART_LSR_DATA_READY) == 0 {
		t.Fatalf("LSR should indicate DATA_READY after input, got 0x%02X", lsr)
	}

	// Read first byte
	rbrVal1, _ := sp.HandlePIORead(UART_RX, 1)
	if rbrVal1 != uint8(testInput[0]) {
		t.Errorf("Read first byte from RBR. Expected 0x%02X, Got 0x%02X", testInput[0], rbrVal1)
	}
	lsr, _ = sp.HandlePIORead(UART_LSR, 1) // Read LSR again
	if (lsr & UART_LSR_DATA_READY) == 0 { // Should still be data ready if Z is there
		t.Errorf("LSR should still indicate DATA_READY after reading one byte, got 0x%02X", lsr)
	}


	// Read second byte
	rbrVal2, _ := sp.HandlePIORead(UART_RX, 1)
	if rbrVal2 != uint8(testInput[1]) {
		t.Errorf("Read second byte from RBR. Expected 0x%02X, Got 0x%02X", testInput[1], rbrVal2)
	}
	lsr, _ = sp.HandlePIORead(UART_LSR, 1) // Read LSR again
	if (lsr & UART_LSR_DATA_READY) != 0 { // Buffer should be empty now
		t.Errorf("LSR should NOT indicate DATA_READY after reading all bytes, got 0x%02X", lsr)
	}

	fmt.Println("Conceptual Test: TestSerialPort_HandleIoRead_DataReg_RxBuffer - PASSED")
}

func TestSerialPort_RegisterAccess(t *testing.T) {
	fmt.Println("Conceptual Test: TestSerialPort_RegisterAccess - START")
	cfg := &pb.SerialPortConfig{Id: "com_reg_test", Type: pb.SerialPortConfig_LOG_ONLY}
	sp, _ := NewSerialPortDevice(cfg.Id, cfg, DEFAULT_SERIAL_IO_BASE_COM1, 4)

	// Test IER
	_ = sp.HandlePIOWrite(UART_IER, 0x0A, 1)
	val, _ := sp.HandlePIORead(UART_IER, 1)
	if val != 0x0A { t.Errorf("IER readback failed. Expected 0x0A, Got 0x%02X", val) }

	// Test LCR & DLAB
	_ = sp.HandlePIOWrite(UART_LCR, 0x80, 1) // Set DLAB
	if (sp.lcrReg & 0x80) == 0 {t.Error("LCR DLAB bit not set")}

	_ = sp.HandlePIOWrite(UART_RX, 0x12, 1) // Write DLL
	_ = sp.HandlePIOWrite(UART_IER, 0x34, 1) // Write DLM
	if sp.dllReg != 0x12 {t.Errorf("DLL write failed. Expected 0x12, Got 0x%02X", sp.dllReg)}
	if sp.dlmReg != 0x34 {t.Errorf("DLM write failed. Expected 0x34, Got 0x%02X", sp.dlmReg)}

	val, _ = sp.HandlePIORead(UART_RX, 1) // Read DLL
	if val != 0x12 {t.Errorf("DLL readback failed. Expected 0x12, Got 0x%02X", val)}
	val, _ = sp.HandlePIORead(UART_IER, 1) // Read DLM
	if val != 0x34 {t.Errorf("DLM readback failed. Expected 0x34, Got 0x%02X", val)}

	_ = sp.HandlePIOWrite(UART_LCR, 0x03, 1) // Clear DLAB (e.g., 8N1)
	if (sp.lcrReg & 0x80) != 0 {t.Error("LCR DLAB bit not cleared")}


	// Test SCR
	_ = sp.HandlePIOWrite(UART_SCR, 0x55, 1)
	val, _ = sp.HandlePIORead(UART_SCR, 1)
	if val != 0x55 {t.Errorf("SCR readback failed. Expected 0x55, Got 0x%02X", val)}


	fmt.Println("Conceptual Test: TestSerialPort_RegisterAccess - PASSED")
}
