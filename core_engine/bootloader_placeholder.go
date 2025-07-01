package core_engine

// MinimalBootloader prints 'H' to COM1 (0x3F8) and then halts.
// Opcodes:
// BA F8 03   ; mov dx, 0x3F8  (COM1 data port)
// B0 48      ; mov al, 'H'    (ASCII 'H')
// EE         ; out dx, al     (Output character)
// F4         ; hlt            (Halt processor)
var MinimalBootloader = []byte{
	0xBA, 0xF8, 0x03, // mov dx, 0x3F8
	0xB0, 0x48,       // mov al, 'H'
	0xEE,             // out dx, al
	0xF4,             // hlt
}

// JustHaltBootloader simply halts the processor.
// Opcodes:
// F4 ; hlt
var JustHaltBootloader = []byte{
	0xF4, // hlt
}
