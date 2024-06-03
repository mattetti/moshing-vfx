package datamosh

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/bits"
)

var Debug bool

type key int

const (
	DebugKey key = iota
	IFrameCountKey
	TrackKey
	IFrameRemovedCountKey
	InteractiveKey
)

// readBitsInt reads the specified number of bits from the reader and returns as an int.
func readBitsInt(r io.Reader, bitPos *int, currentByte *byte, buffer *[]byte, numBits int) (int, error) {
	value, err := readBits(r, bitPos, currentByte, buffer, numBits)
	return int(value), err
}

// readBits reads a specified number of bits from the reader.
func readBits(r io.Reader, bitPos *int, currentByte *byte, buffer *[]byte, numBits int) (uint32, error) {
	var result uint32
	for i := 0; i < numBits; i++ {
		bit, err := readBit(r, bitPos, currentByte, buffer)
		if err != nil {
			return 0, err
		}
		result = (result << 1) | uint32(bit)
	}
	return result, nil
}

// readBit reads a single bit from the reader.
func readBit(r io.Reader, bitPos *int, currentByte *byte, buffer *[]byte) (byte, error) {
	if *bitPos%8 == 0 {
		n, err := r.Read(*buffer)
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, io.EOF
		}
		*currentByte = (*buffer)[0]
	}
	bit := (*currentByte >> (7 - (*bitPos % 8))) & 1
	*bitPos++
	return bit, nil
}

// readExpGolombCode reads an Exp-Golomb coded value from the reader.
func readExpGolombCode(r io.Reader, bitPos *int, currentByte *byte, buffer *[]byte) (uint32, error) {
	var leadingZeroBits uint32

	// Read bits in bulk for efficiency
	buf := make([]byte, 4)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}
	val := binary.BigEndian.Uint32(buf)
	leadingZeroBits = uint32(bits.LeadingZeros32(val))

	// Skip the leading zero bits
	for i := uint32(0); i < leadingZeroBits; i++ {
		_, err := readBit(r, bitPos, currentByte, buffer)
		if err != nil {
			return 0, err
		}
	}

	// Read the remaining bits
	codeNum, err := readBits(r, bitPos, currentByte, buffer, int(leadingZeroBits))
	if err != nil {
		return 0, err
	}

	codeNum += (1 << leadingZeroBits) - 1
	return codeNum, nil
}

func bitsToInt(bits []byte, size uint) uint32 {
	var result uint32
	for i := uint(0); i < size; i++ {
		byteIdx := i / 8
		bitIdx := 7 - (i % 8)
		bit := (bits[byteIdx] >> bitIdx) & 0x01
		result = (result << 1) | uint32(bit)
	}
	return result
}

// hexDump reads data from an io.Reader and prints it in hex format.
// Then rewinds the reader to the original position.
func hexDump(r io.ReadSeeker, size int) error {
	buffer := make([]byte, size)
	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read data: %v", err)
	}

	fmt.Println(hex.Dump(buffer[:n]))
	r.Seek(-int64(n), io.SeekCurrent)
	return nil
}

// bitDump reads data from an io.ReadSeeker and prints it in bit format.
// Then rewinds the reader to the original position, and prints additional info.
func bitDump(r io.ReadSeeker, size int) error {
	// Save the current position
	currentPos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current position: %v", err)
	}

	buffer := make([]byte, size)
	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read data: %v", err)
	}

	// Convert the buffer to a bitstream
	var bitstream string
	var leadingZeroBits int
	var foundFirstOneBit bool
	for i := 0; i < n; i++ {
		for j := 7; j >= 0; j-- {
			bit := (buffer[i] >> j) & 1
			bitstream += fmt.Sprintf("%d", bit)
			if !foundFirstOneBit {
				leadingZeroBits++
				if bit == 1 {
					foundFirstOneBit = true
				}
			}
		}
	}

	if foundFirstOneBit {
		leadingZeroBits-- // Adjust because we counted one extra zero before finding the first 1 bit
	}

	fmt.Printf("Bitstream: %s\n", bitstream)
	fmt.Printf("Leading zero bits: %d\n", leadingZeroBits)
	fmt.Printf("Total bytes read: %d\n", n)

	// Rewind the reader to the original position
	_, err = r.Seek(currentPos, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to rewind reader: %v", err)
	}

	return nil
}
