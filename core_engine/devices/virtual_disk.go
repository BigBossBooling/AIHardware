package devices

import (
	"fmt"
	"sync"
)

// VirtualDiskImage defines the interface for a block storage device.
type VirtualDiskImage interface {
	// ReadSectors reads 'count' sectors starting from LBA 'lba'.
	// It should return a byte slice of length count * SectorSize().
	ReadSectors(lba uint64, count uint16) ([]byte, error)

	// WriteSectors writes the provided 'data' to sectors starting at LBA 'lba'.
	// The length of 'data' must be a multiple of SectorSize().
	// The number of sectors written is len(data) / SectorSize().
	WriteSectors(lba uint64, data []byte) error

	// SectorSize returns the size of a single sector in bytes (e.g., 512).
	SectorSize() uint16

	// Size returns the total size of the disk image in bytes.
	Size() uint64

	// Flush ensures any buffered data is written to the backing store.
	// For an in-memory disk, this might be a no-op.
	Flush() error
}

// MemoryDiskImage is an in-memory implementation of VirtualDiskImage.
type MemoryDiskImage struct {
	mu         sync.RWMutex
	data       []byte
	sectorSize uint16
	numSectors uint64
}

// NewMemoryDiskImage creates a new in-memory disk image of a given size.
// sizeInBytes must be a multiple of sectorSize.
func NewMemoryDiskImage(sizeInBytes uint64, sectorSize uint16) (*MemoryDiskImage, error) {
	if sectorSize == 0 {
		return nil, fmt.Errorf("sector size cannot be zero")
	}
	if sizeInBytes%uint64(sectorSize) != 0 {
		return nil, fmt.Errorf("total size (0x%X) must be a multiple of sector size (0x%X)", sizeInBytes, sectorSize)
	}
	if sizeInBytes == 0 {
		return nil, fmt.Errorf("disk image size cannot be zero")
	}

	numSectors := sizeInBytes / uint64(sectorSize)
	data := make([]byte, sizeInBytes)

	return &MemoryDiskImage{
		data:       data,
		sectorSize: sectorSize,
		numSectors: numSectors,
	}, nil
}

// ReadSectors implements the VirtualDiskImage interface.
func (mdi *MemoryDiskImage) ReadSectors(lba uint64, count uint16) ([]byte, error) {
	mdi.mu.RLock()
	defer mdi.mu.RUnlock()

	if count == 0 {
		return []byte{}, nil // Or an error, depending on desired behavior for 0 count.
	}
	if lba+uint64(count) > mdi.numSectors {
		return nil, fmt.Errorf("read past end of disk: LBA %d + Count %d > Total Sectors %d", lba, count, mdi.numSectors)
	}

	startOffset := lba * uint64(mdi.sectorSize)
	endOffset := startOffset + (uint64(count) * uint64(mdi.sectorSize))

	// Create a copy to avoid returning a slice that could be modified if the caller holds onto it
	// while another write happens.
	readData := make([]byte, uint64(count)*uint64(mdi.sectorSize))
	copy(readData, mdi.data[startOffset:endOffset])

	return readData, nil
}

// WriteSectors implements the VirtualDiskImage interface.
func (mdi *MemoryDiskImage) WriteSectors(lba uint64, data []byte) error {
	mdi.mu.Lock()
	defer mdi.mu.Unlock()

	if len(data) == 0 {
		return fmt.Errorf("write data cannot be empty")
	}
	if uint16(len(data))%mdi.sectorSize != 0 {
		return fmt.Errorf("write data size (%d) must be a multiple of sector size (%d)", len(data), mdi.sectorSize)
	}

	numSectorsToWrite := uint16(len(data) / int(mdi.sectorSize))
	if lba+uint64(numSectorsToWrite) > mdi.numSectors {
		return fmt.Errorf("write past end of disk: LBA %d + Count %d > Total Sectors %d", lba, numSectorsToWrite, mdi.numSectors)
	}

	startOffset := lba * uint64(mdi.sectorSize)
	copy(mdi.data[startOffset:], data)

	return nil
}

// SectorSize implements the VirtualDiskImage interface.
func (mdi *MemoryDiskImage) SectorSize() uint16 {
	return mdi.sectorSize
}

// Size implements the VirtualDiskImage interface.
func (mdi *MemoryDiskImage) Size() uint64 {
	return uint64(len(mdi.data))
}

// Flush implements the VirtualDiskImage interface. For MemoryDiskImage, it's a no-op.
func (mdi *MemoryDiskImage) Flush() error {
	// No-op for in-memory disk
	return nil
}
