package devices

import (
	"testing"
	"fmt"
)

// mockLogger can be used if detailed logging within tests is needed.
type mockLogger struct{}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format, v...) // Or log to a buffer for assertion
}

// TestPICInitialization checks basic PIC initialization.
func TestPICInitialization(t *testing.T) {
	pic := NewPICController()
	if pic.Master == nil || pic.Slave == nil {
		t.Fatal("PIC master or slave not initialized")
	}

	// Check default IMR values (all masked)
	if pic.Master.imr != 0xFF {
		t.Errorf("Master IMR should be 0xFF initially, got 0x%X", pic.Master.imr)
	}
	if pic.Slave.imr != 0xFF {
		t.Errorf("Slave IMR should be 0xFF initially, got 0x%X", pic.Slave.imr)
	}
	// log.Println("TestPICInitialization Passed")
}

// TestPIC_ICW_Sequence tests the Initialization Command Word sequence.
func TestPIC_ICW_Sequence(t *testing.T) {
	pic := NewPICController()
	master := pic.Master
	slave := pic.Slave

	// Master ICW1: Edge triggered, Cascade mode, ICW4 needed
	master.HandleIO(PIC1_COMMAND_PORT, []byte{ICW1_INIT | ICW1_ICW4}, true)
	if master.icwStep != 1 || master.icw[0] != (ICW1_INIT|ICW1_ICW4) {
		t.Errorf("Master ICW1 incorrect. Step: %d, Val: 0x%X", master.icwStep, master.icw[0])
	}

	// Master ICW2: Vector offset 0x20
	master.HandleIO(PIC1_DATA_PORT, []byte{0x20}, true)
	if master.icwStep != 2 || master.icw[1] != 0x20 {
		t.Errorf("Master ICW2 incorrect. Step: %d, Val: 0x%X", master.icwStep, master.icw[1])
	}

	// Master ICW3: Slave on IRQ2 (bit 2 = 1 -> 0x04)
	master.HandleIO(PIC1_DATA_PORT, []byte{1 << IRQ_CASCADE}, true) // Cascade on IRQ 2
	if master.icwStep != 3 || master.icw[2] != (1 << IRQ_CASCADE) {
		t.Errorf("Master ICW3 incorrect. Step: %d, Val: 0x%X", master.icwStep, master.icw[2])
	}

	// Master ICW4: 8086 mode, Auto EOI (optional for testing)
	master.HandleIO(PIC1_DATA_PORT, []byte{ICW4_8086 /*| ICW4_AUTO*/}, true)
	if master.icwStep != 0 || master.icw[3] != ICW4_8086 {
		t.Errorf("Master ICW4 incorrect. Step: %d, Val: 0x%X", master.icwStep, master.icw[3])
	}
	// if !master.autoEOI { t.Errorf("Master Auto EOI not set by ICW4") }


	// Slave ICW1: Edge triggered, Cascade mode, ICW4 needed
	slave.HandleIO(PIC2_COMMAND_PORT, []byte{ICW1_INIT | ICW1_ICW4}, true)
	if slave.icwStep != 1 || slave.icw[0] != (ICW1_INIT|ICW1_ICW4) {
		t.Errorf("Slave ICW1 incorrect. Step: %d, Val: 0x%X", slave.icwStep, slave.icw[0])
	}

	// Slave ICW2: Vector offset 0x28
	slave.HandleIO(PIC2_DATA_PORT, []byte{0x28}, true)
	if slave.icwStep != 2 || slave.icw[1] != 0x28 {
		t.Errorf("Slave ICW2 incorrect. Step: %d, Val: 0x%X", slave.icwStep, slave.icw[1])
	}

	// Slave ICW3: Slave ID is IRQ2
	slave.HandleIO(PIC2_DATA_PORT, []byte{IRQ_CASCADE}, true) // Slave is on IRQ2 of master
	if slave.icwStep != 3 || slave.icw[2] != IRQ_CASCADE {
		t.Errorf("Slave ICW3 incorrect. Step: %d, Val: 0x%X", slave.icwStep, slave.icw[2])
	}

	// Slave ICW4: 8086 mode
	slave.HandleIO(PIC2_DATA_PORT, []byte{ICW4_8086}, true)
	if slave.icwStep != 0 || slave.icw[3] != ICW4_8086 {
		t.Errorf("Slave ICW4 incorrect. Step: %d, Val: 0x%X", slave.icwStep, slave.icw[3])
	}
	// log.Println("TestPIC_ICW_Sequence Passed")
}

// TestPIC_IRQ_Handling tests raising IRQs, masking, and vector retrieval.
func TestPIC_IRQ_Handling(t *testing.T) {
	pic := NewPICController()
	// Initialize PICs (vector offsets 0x20 for master, 0x28 for slave)
	// Master: ICW1(init, icw4), ICW2(0x20), ICW3(1<<IRQ_CASCADE), ICW4(8086)
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{ICW1_INIT | ICW1_ICW4}, true)
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{0x20}, true)
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{1 << IRQ_CASCADE}, true)
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{ICW4_8086}, true)
	// Slave: ICW1(init, icw4), ICW2(0x28), ICW3(IRQ_CASCADE), ICW4(8086)
	pic.Slave.HandleIO(PIC2_COMMAND_PORT, []byte{ICW1_INIT | ICW1_ICW4}, true)
	pic.Slave.HandleIO(PIC2_DATA_PORT, []byte{0x28}, true)
	pic.Slave.HandleIO(PIC2_DATA_PORT, []byte{IRQ_CASCADE}, true)
	pic.Slave.HandleIO(PIC2_DATA_PORT, []byte{ICW4_8086}, true)

	// Unmask all interrupts on master and slave for testing
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{0x00}, true) // Write 0 to IMR
	if pic.Master.imr != 0x00 { t.Fatalf("Master IMR not cleared, got 0x%X", pic.Master.imr) }
	pic.Slave.HandleIO(PIC2_DATA_PORT, []byte{0x00}, true)  // Write 0 to IMR
	if pic.Slave.imr != 0x00 { t.Fatalf("Slave IMR not cleared, got 0x%X", pic.Slave.imr) }


	// 1. Raise IRQ 1 on master
	pic.RaiseIRQ(1)
	if (pic.Master.irr & (1 << 1)) == 0 {
		t.Errorf("Master IRR bit 1 not set. IRR: 0x%X", pic.Master.irr)
	}
	if !pic.HasPendingInterrupt() {
		t.Errorf("PICController shows no pending interrupt after IRQ1 raise.")
	}
	vec, ok := pic.GetInterruptVector()
	if !ok || vec != 0x20+1 {
		t.Errorf("Expected vector 0x21 for IRQ 1, got 0x%X (ok: %v)", vec, ok)
	}
	if (pic.Master.isr & (1 << 1)) == 0 {
		t.Errorf("Master ISR bit 1 not set after GetInterruptVector. ISR: 0x%X", pic.Master.isr)
	}
	if (pic.Master.irr & (1 << 1)) != 0 {
		t.Errorf("Master IRR bit 1 not cleared after GetInterruptVector. IRR: 0x%X", pic.Master.irr)
	}

	// EOI for IRQ 1 (non-specific EOI)
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{OCW2_EOI}, true)
	if (pic.Master.isr & (1 << 1)) != 0 {
		t.Errorf("Master ISR bit 1 not cleared after EOI. ISR: 0x%X", pic.Master.isr)
	}

	// 2. Raise IRQ 8 (slave IRQ 0)
	pic.RaiseIRQ(8) // This is IRQ 0 on slave, should also raise IRQ_CASCADE (2) on master
	if (pic.Slave.irr & (1 << 0)) == 0 {
		t.Errorf("Slave IRR bit 0 not set for IRQ 8. Slave IRR: 0x%X", pic.Slave.irr)
	}
	if (pic.Master.irr & (1 << IRQ_CASCADE)) == 0 {
		t.Errorf("Master IRR bit for cascade (IRQ %d) not set for IRQ 8. Master IRR: 0x%X", IRQ_CASCADE, pic.Master.irr)
	}
	if !pic.HasPendingInterrupt() {
		t.Errorf("PICController shows no pending interrupt after IRQ8 raise.")
	}

	vec, ok = pic.GetInterruptVector() // This should fetch from master, then slave
	expectedVec := uint8(0x28 + 0) // Slave base + slave IRQ 0
	if !ok || vec != expectedVec {
		t.Errorf("Expected vector 0x%X for IRQ 8, got 0x%X (ok: %v)", expectedVec, vec, ok)
	}
	if (pic.Slave.isr & (1 << 0)) == 0 {
		t.Errorf("Slave ISR bit 0 not set after GetInterruptVector for IRQ 8. Slave ISR: 0x%X", pic.Slave.isr)
	}
	// Master's cascade IRQ (IRQ2) should also be in ISR
	if (pic.Master.isr & (1 << IRQ_CASCADE)) == 0 {
		t.Errorf("Master ISR bit for cascade (IRQ %d) not set. Master ISR: 0x%X", IRQ_CASCADE, pic.Master.isr)
	}

	// EOI for IRQ 8: Master first (for cascade line), then Slave
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{OCW2_EOI}, true) // EOI for IRQ_CASCADE on master
	if (pic.Master.isr & (1 << IRQ_CASCADE)) != 0 {
		t.Errorf("Master ISR for cascade not cleared after EOI. Master ISR: 0x%X", pic.Master.isr)
	}
	pic.Slave.HandleIO(PIC2_COMMAND_PORT, []byte{OCW2_EOI}, true)  // EOI for IRQ 0 on slave
	if (pic.Slave.isr & (1 << 0)) != 0 {
		t.Errorf("Slave ISR bit 0 not cleared after EOI. Slave ISR: 0x%X", pic.Slave.isr)
	}


	// 3. Masking: Mask IRQ 1 on master, then raise it
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{1 << 1}, true) // Mask IRQ 1
	if pic.Master.imr != (1 << 1) { t.Errorf("Master IMR not set correctly. Expected 0x02, got 0x%X", pic.Master.imr)}

	pic.RaiseIRQ(1)
	if (pic.Master.irr & (1 << 1)) == 0 { // IRR should still get set
		t.Errorf("Master IRR bit 1 not set even if masked. IRR: 0x%X", pic.Master.irr)
	}
	if pic.HasPendingInterrupt() { // Should be false as only masked IRQ is pending
		vec, ok = pic.GetInterruptVector()
		t.Errorf("PICController shows pending interrupt for masked IRQ1. Vec: 0x%X, OK: %v", vec, ok)
	}
	vec, ok = pic.Master.GetInterruptVector() // Try to get from master directly
	if ok {
		t.Errorf("Master GetInterruptVector returned true for masked IRQ 1. Vector: 0x%X", vec)
	}

	// Unmask IRQ 1
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{0x00}, true) // Unmask all
	if !pic.HasPendingInterrupt() {
		t.Errorf("PICController shows no pending interrupt after unmasking IRQ1.")
	}
	vec, ok = pic.GetInterruptVector()
	if !ok || vec != 0x20+1 {
		t.Errorf("Expected vector 0x21 for IRQ 1 after unmasking, got 0x%X (ok: %v)", vec, ok)
	}
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{OCW2_EOI}, true) // EOI

	// 4. Priority: Raise IRQ 7 then IRQ 0 (both on master). IRQ 0 is higher priority.
	pic.RaiseIRQ(7)
	pic.RaiseIRQ(0) // IRQ 0 is highest priority on master
	vec, ok = pic.GetInterruptVector()
	if !ok || vec != 0x20+0 {
		t.Errorf("Expected vector 0x20 for IRQ 0 (higher priority), got 0x%X (ok: %v)", vec, ok)
	}
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{OCW2_EOI}, true) // EOI for IRQ 0

	// IRQ 7 should still be pending
	if !pic.HasPendingInterrupt() {
		t.Errorf("PICController shows no pending interrupt for IRQ7 after IRQ0 handled.")
	}
	vec, ok = pic.GetInterruptVector()
	if !ok || vec != 0x20+7 {
		t.Errorf("Expected vector 0x27 for IRQ 7, got 0x%X (ok: %v)", vec, ok)
	}
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{OCW2_EOI}, true) // EOI for IRQ 7

	if pic.HasPendingInterrupt() {
		t.Errorf("PICController shows pending interrupt when none should be.")
	}

	// log.Println("TestPIC_IRQ_Handling Passed")
}

// TestPIC_AutoEOI tests Auto EOI behavior (if implemented and enabled in ICW4).
func TestPIC_AutoEOI(t *testing.T) {
	pic := NewPICController()
	// Initialize Master with Auto EOI
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{ICW1_INIT | ICW1_ICW4}, true)
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{0x30}, true) // Vector base 0x30
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{0x00}, true) // No slaves for simplicity here
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{ICW4_8086 | ICW4_AUTO}, true) // Auto EOI enabled

	if !pic.Master.autoEOI {
		t.Fatal("Master Auto EOI not enabled after ICW4")
	}

	// Unmask IRQ 0
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{0xFE}, true) // Unmask IRQ0 (0xFF -> 0xFE)

	pic.Master.RaiseIRQ(0)
	if !pic.Master.HasPendingInterrupt() {
		t.Fatal("Master shows no pending interrupt for IRQ0 with AutoEOI setup.")
	}

	vec, ok := pic.Master.GetInterruptVector()
	if !ok || vec != 0x30+0 {
		t.Errorf("Expected vector 0x30 for IRQ 0 with AutoEOI, got 0x%X (ok: %v)", vec, ok)
	}

	// With Auto EOI, ISR bit should be cleared immediately by GetInterruptVector
	if (pic.Master.isr & (1 << 0)) != 0 {
		t.Errorf("Master ISR bit 0 not cleared after GetInterruptVector with Auto EOI. ISR: 0x%X", pic.Master.isr)
	}

	if pic.Master.HasPendingInterrupt() {
		t.Errorf("Master still shows pending interrupt after IRQ0 handled with Auto EOI.")
	}
	// log.Println("TestPIC_AutoEOI Passed")
}

func TestPIC_ReadIMR_IRR_ISR(t *testing.T) {
	pic := NewPICController()
	// Minimal init
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{ICW1_INIT | ICW1_ICW4}, true)
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{0x20}, true)
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{0x00}, true) // single mode
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{ICW4_8086}, true)

	// Read IMR (should be 0x00 after ICW1 if not explicitly set after, but our ICW1 clears it)
	// Then set it to something.
	pic.Master.HandleIO(PIC1_DATA_PORT, []byte{0xAB}, true) // Set IMR
	val, err := pic.Master.HandleIO(PIC1_DATA_PORT, nil, false) // Read IMR
	if err != nil || val[0] != 0xAB {
		t.Errorf("Error reading IMR or wrong value. Expected 0xAB, got 0x%X, err: %v", val, err)
	}

	// Raise IRQ0 and IRQ2
	pic.Master.RaiseIRQ(0)
	pic.Master.RaiseIRQ(2)

	// Read IRR (OCW3 command)
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{OCW3_READ_IRR}, true)
	val, err = pic.Master.HandleIO(PIC1_COMMAND_PORT, nil, false) // Read from command port
	expectedIRR := byte((1<<0) | (1<<2))
	if err != nil || val[0] != expectedIRR {
		t.Errorf("Error reading IRR or wrong value. Expected 0x%X, got 0x%X, err: %v", expectedIRR, val, err)
	}

	// Get one interrupt. Given IMR=0xAB, IRQ0 is masked. IRQ2 is also masked.
	// IRR = 0x05 (IRQ0, IRQ2). IMR = 0xAB (10101011)
	// Unmasked requests: IRR & ~IMR = 00000101 & 01010100 = 00000100 (IRQ2)
	// This means my previous calculation of pendingAndUnmasked was wrong too.
	// ^0xAB is 0x54 (01010100).
	// pendingAndUnmasked = 0x05 & 0x54 = 0x04. So IRQ2 is the only unmasked one.
	// GetInterruptVector will service IRQ2.
	// After servicing IRQ2:
	//   irr = 0x05 &^ (1<<2) = 0x01 (IRQ0 still physically requested but masked)
	//   isr = 0x00 | (1<<2) = 0x04 (IRQ2 in service)
	vectorVal, _ := pic.Master.GetInterruptVector()
	expectedVector := byte(0x20 + 2) // Base 0x20 + IRQ2
	if vectorVal != expectedVector {
		t.Errorf("Expected vector 0x%02X for IRQ2, got 0x%02X", expectedVector, vectorVal)
	}


	// Direct inspection after GetInterruptVector
	if pic.Master.irr != byte(1<<0) { // IRR should be IRQ0 (0x01) (IRQ2 was cleared from IRR)
		t.Errorf("Direct IRR check after GetInterruptVector: Expected 0x%02X, got 0x%02X", byte(1<<0), pic.Master.irr)
	}
	if pic.Master.isr != byte(1<<2) { // ISR should be IRQ2 (0x04)
		t.Errorf("Direct ISR check after GetInterruptVector: Expected 0x%02X, got 0x%02X", byte(1<<2), pic.Master.isr)
	}

	// Read ISR (OCW3 command) - should reflect that IRQ2 is in service
	t.Logf("TestPIC_ReadIMR_IRR_ISR: State before OCW3_READ_ISR cmd: IRR=0x%02X, ISR=0x%02X, readISR_flag=%v", pic.Master.irr, pic.Master.isr, pic.Master.readISR)
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{OCW3_READ_ISR}, true)
	t.Logf("TestPIC_ReadIMR_IRR_ISR: State after OCW3_READ_ISR cmd (flag should be true): IRR=0x%02X, ISR=0x%02X, readISR_flag=%v", pic.Master.irr, pic.Master.isr, pic.Master.readISR)
	val, err = pic.Master.HandleIO(PIC1_COMMAND_PORT, nil, false) // Read from command port
	t.Logf("TestPIC_ReadIMR_IRR_ISR: Value read from CMD port (expected ISR=0x04): 0x%02X", val[0])
	expectedISR := byte(1<<2) // IRQ2 should be in service
	if err != nil || val[0] != expectedISR {
		t.Errorf("Error reading ISR or wrong value. Expected 0x%X, got 0x%X, err: %v", expectedISR, val[0], err)
	}

	// IRR should now only have IRQ0 set (IRQ2 was cleared by GetInterruptVector, IRQ0 remains as it was masked)
	pic.Master.HandleIO(PIC1_COMMAND_PORT, []byte{OCW3_READ_IRR}, true)
	val, err = pic.Master.HandleIO(PIC1_COMMAND_PORT, nil, false)
	expectedIRR = byte(1<<0) // IRQ0 is still physically requested in IRR
	if err != nil || val[0] != expectedIRR {
		t.Errorf("Error reading IRR after one GetInt or wrong value. Expected 0x%X, got 0x%X, err: %v", expectedIRR, val, err)
	}
}
// Note: More tests could be added for edge cases, specific EOI, rotating EOI, special mask mode, etc.
// but this covers the core functionality.
