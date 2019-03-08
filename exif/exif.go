package exif

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
)

const (
	// APP1 marker prefix.
	markerPrefix = 0xFF

	// APP1 marker.
	appMarker = 0xE1

	// The size of each tag in a single IFD.
	tagSize = 12

	// The size of the offset field in the IFD.
	ifdOffsetSize = 4

	// The size of the byte order field in IFD in bytes.
	byteOrderSize = 2

	// The size of the data length field in the EXIF headers in bytes.
	dataLenghtSize = 2

	// the size of the tag count field in the IFD.
	tagCountLenSize = 2
)

// The exif identifier.
var exifIdent = []byte{'E', 'x', 'i', 'f', 0x00, 0x00}

// Discard parsed the file passed and writes to io.Writer the
// same file without the EXIF IFD's.
func Discard(file io.Reader, output io.Writer) error {
	raw, err := ioutil.ReadAll(file)
	if err != nil {
		// Ignore unexpected EOF errors.
		if err != io.EOF {
			return err
		}
	}

	ifdOffset, _, byteOrder, err := parseImageHeaders(raw)
	if err != nil {
		return err
	}
	log.Printf("The offset to the EXIF IFD is %d:", ifdOffset)

	result, err := removeFirstIFD(raw, ifdOffset, byteOrder)
	if err != nil {
		return err
	}

	output.Write(result)

	log.Println("Succesfully removed IFD.")
	return nil
}

// parseImageHeaders parses the image headers to check that the information in the headers is not corrupted
// it also return the followig information uppon succesful parsing:
// The first image folder directory (IFD) offset (which is the EXIF IFD - see http://www.exif.org/Exif2-2.PDF p.15).
// The length of the data.
// the byteOrder and any error which might occur in the process of parsing the headers.
func parseImageHeaders(raw []byte) (uint32, int, binary.ByteOrder, error) {
	var byteOrder binary.ByteOrder

	markerOffset, err := locateAPPMarker(raw)
	if err != nil {
		return 0, 0, binary.BigEndian, err
	}

	log.Printf("Found marker offset at: %d", markerOffset)

	// Create a new buffer after the APP1 markers to read the rest of the headers from.
	buff := bytes.NewBuffer(raw[markerOffset+2:])
	dataLengthBytes := make([]byte, dataLenghtSize)
	if n, err := buff.Read(dataLengthBytes); err != nil || n != dataLenghtSize {
		return 0, 0, binary.BigEndian,
			fmt.Errorf("an error occurred while attempting to find data length: %v", err)
	}
	// remove two bytes for 0xffff (EOF).
	dataLength := int(binary.BigEndian.Uint16(dataLengthBytes)) - 2

	// check exif identifier (four bytes for 'EXIF' and two padding bytes.)
	exifHeader := make([]byte, len(exifIdent))
	if n, err := buff.Read(exifHeader); err != nil || n != len(exifIdent) {
		if !bytes.Equal(exifHeader, exifIdent) {
			return 0, 0, binary.BigEndian,
				fmt.Errorf("an error occurred while attempting to find EXIF ident code: %v", err)
		}
	}

	// Read byte order from TIFF Header.
	bo := make([]byte, byteOrderSize)
	if n, err := buff.Read(bo); err != nil || n != byteOrderSize {
		return 0, 0, binary.BigEndian,
			fmt.Errorf("an error occurred while attempting to find TIFF header: %v", err)
	}

	// Either "II" (0x4949) - LittleEndian
	// or "MM" (0x4d4d) - BigEndian
	// depending on the machine.
	// (See: http://www.cipa.jp/std/documents/e/DC-008-2012_E.pdf p.19 for details.)
	switch string(bo) {
	case "II":
		byteOrder = binary.LittleEndian
	case "MM":
		byteOrder = binary.BigEndian
	default:
		return 0, 0, binary.BigEndian,
			fmt.Errorf("could not read byte order from tiff header")
	}

	// The TIFF header keeps a 2-byte number (0x002A) as padding.
	fixedNum := make([]byte, 2)
	if _, err := buff.Read(fixedNum); err != nil || byteOrder.Uint16(fixedNum) != 42 {
		return 0, 0, binary.BigEndian,
			fmt.Errorf("an error occurred while attempting to find TIFF header: %v", err)
	}

	// load offset to first IFD (The EXIF IFD: see http://www.exif.org/Exif2-2.PDF p.15)
	ifdOffsetBytes := make([]byte, ifdOffsetSize)
	if n, err := buff.Read(ifdOffsetBytes); err != nil || n != ifdOffsetSize {
		return 0, 0, binary.BigEndian,
			fmt.Errorf("an error occurred while attempting to find the first IFD offset: %v", err)
	}
	ifdOffset := byteOrder.Uint32(ifdOffsetBytes)

	// the if the offset is 0x0008 it comes right after the header.
	// See http://www.exif.org/Exif2-2.PDF p.10
	if ifdOffset == 8 {
		ifdOffset = uint32(len(raw) - buff.Len())
	}

	return ifdOffset, dataLength, byteOrder, nil
}

func locateAPPMarker(raw []byte) (int, error) {
	var markerOffset int
	for markerOffset = 0; markerOffset < len(raw)-1; markerOffset++ {
		if foundAPPMarker(raw, markerOffset) {
			return markerOffset, nil
		}
	}
	return -1, fmt.Errorf("an error occurred: Could not find image markers")
}

func foundAPPMarker(raw []byte, offset int) bool {
	return (raw[offset] == markerPrefix) && (raw[offset+1] == appMarker)
}

func purgeDirs(raw []byte, ifdOffset uint32, byteOrder binary.ByteOrder) ([]byte, error) {
	ifdReader := bytes.NewReader(raw)
	result := raw
	var offset uint32
	offset = ifdOffset
	for offset != 0 {
		// Retrieve the tag count - the first field in the IFD.
		ifdReader.Seek(int64(offset), io.SeekStart)
		var tagCount int16
		if err := binary.Read(ifdReader, byteOrder, &tagCount); err != nil {
			return nil, err
		}

		log.Printf("the number of tags is: %d", tagCount)

		// Find the offset to the next ifd.
		ifdReader.Seek(int64(tagCount*tagSize), io.SeekCurrent)
		if err := binary.Read(ifdReader, byteOrder, &offset); err != nil {
			return nil, err
		}

		log.Printf("the offset to the next ifd is: %d", offset)

		if offset > uint32(len(raw)) || offset == 0 {
			break
		}

		// The end of the IFD block is the size of the number of tags * tag size (which is 12 bytes.)
		exifdEnd := int16(offset) + tagCountLenSize + tagCount*tagSize + ifdOffsetSize
		ifdOffset := int16(offset)

		numOfBytesDiscarded := tagCountLenSize + tagCount*tagSize + ifdOffsetSize
		filler := make([]byte, numOfBytesDiscarded)
		result = append(result[:ifdOffset], filler...)
		result = append(result[:exifdEnd], raw[exifdEnd:]...)

		log.Printf("Number of bytes discarede thus far: %d", numOfBytesDiscarded)

		if ifdReader.Len() == 0 {
			return nil, fmt.Errorf("Offset past EOF")
		}
	}

	return result, nil
}

func removeFirstIFD(raw []byte, ifdOffset uint32, byteOrder binary.ByteOrder) ([]byte, error) {
	ifdReader := bytes.NewReader(raw[ifdOffset:])

	// Retrieve the tag count - the first field in the IFD.
	var tagCount int16
	if err := binary.Read(ifdReader, byteOrder, &tagCount); err != nil {
		return nil, err
	}

	log.Printf("the number of tags is: %d", tagCount)

	// The end of the IFD block is the size of the number of tags * tag size (which is 12 bytes.)
	exifdEnd := int16(ifdOffset) + tagCountLenSize + tagCount*tagSize + ifdOffsetSize
	return append(raw[:ifdOffset], raw[exifdEnd:]...), nil
}
