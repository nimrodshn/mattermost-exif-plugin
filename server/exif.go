package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

const (
	MARKER_PREFIX   = 0xFF
	APP1_MARKER     = 0xE1
	FIELD_SIZE      = 12
	IFD_OFFSET_SIZE = 4
)

var EXIF_IDENT = []byte{'E', 'x', 'i', 'f', 0x00, 0x00}

// FileWillBeUploaded is invoked when a file is uploaded, but before it is committed to backing store.
// Read from file to retrieve the body of the uploaded file.
//
// To reject a file upload, return an non-empty string describing why the file was rejected.
// To modify the file, write to the output and/or return a non-nil *model.FileInfo, as well as an empty string.
// To allow the file without modification, do not write to the output and return a nil *model.FileInfo and an empty string.
//
// Note that this method will be called for files uploaded by plugins, including the plugin that uploaded the post.
// FileInfo.Size will be automatically set properly if you modify the file.
func (p *Plugin) FileWillBeUploaded(c *plugin.Context, info *model.FileInfo, file io.Reader, output io.Writer) (*model.FileInfo, string) {
	return p.discardExif(info, file, output)
}

// discardExif attempts to decode an image file and the encode it back - by that removing the exif metdata.
func (p *Plugin) naiveDiscardExif(info *model.FileInfo, file io.Reader, output io.Writer) (*model.FileInfo, string) {
	im, _, err := image.Decode(file)
	if err != nil {
		p.API.LogError("An error occurred while trying to decoding the uploaded file")
		return nil, fmt.Sprintf("An error occurred while trying to decode the uploaded file: %v", err)
	}
	err = jpeg.Encode(output, im, nil)
	if err != nil {
		p.API.LogError("An error occurred while trying to encode the uploaded file")
		return nil, fmt.Sprintf("An error occurred while trying to encode the uploaded file: %v", err)
	}
	p.API.LogInfo("Processed a new image.")
	return info, ""
}

// discardExif attempts to decode an image file and the encode it back - by that removing the exif metdata.
func (p *Plugin) discardExif(info *model.FileInfo, file io.Reader, output io.Writer) (*model.FileInfo, string) {
	p.API.LogInfo("Started reading data")

	raw, err := ioutil.ReadAll(file)
	if err != nil {
		// Ignore unexpected EOF errors.
		if err != io.EOF {
			p.API.LogError("An error occurred while trying to read the uploaded file" + err.Error())
			return nil, fmt.Sprintf("An error occurred while trying to read the uploaded file: %v", err)
		}
	}

	ifdOffset, _, byteOrder, err := p.parseImageHeaders(raw)
	if err != nil {
		p.API.LogError("An error occured while attempting to parse image headers" + err.Error())
		return nil, fmt.Sprintf("An error occured while attempting to parse image headers: %v", err)
	}

	p.API.LogInfo(fmt.Sprintf("The offset of the first IFD: %d", ifdOffset))
	p.API.LogInfo(fmt.Sprintf("The byte order is: %s", byteOrder.String()))

	ifdReader := bytes.NewReader(raw)
	if _, err := ifdReader.Seek(int64(ifdOffset), io.SeekStart); err != nil {
		p.API.LogError("Could not find first IFD." + err.Error())
	}

	var tagCount uint16
	if err := binary.Read(ifdReader, byteOrder, &tagCount); err != nil {
		p.API.LogError("Could not read the tag count for the EXIF IFD." + err.Error())
		return nil, fmt.Sprintf("Could not read the tag count for the EXIF IFD.")
	}

	p.API.LogInfo(fmt.Sprintf("Number of tags in IFD: %d", tagCount))

	ifdReader.Seek(int64(tagCount*FIELD_SIZE+IFD_OFFSET_SIZE), io.SeekCurrent)
	ifdEnd := ifdReader.Len()

	output.Write(append(raw[:ifdOffset], raw[ifdEnd:]...))

	p.API.LogInfo("Processed a new EXIF image.")
	return info, ""
}

// parseImageHeaders parses the image headers to check that the information in the headers is not corrupted
// it also return the followig information uppon succesful parsing: the length of the data,
// the byteOrder and the first image folder directory (IFD) offset.
func (p *Plugin) parseImageHeaders(raw []byte) (ifdOffset uint32, dataLength int, byteOrder binary.ByteOrder, err error) {
	var markerOffset int
	for markerOffset = 0; markerOffset < len(raw)-2; markerOffset++ {
		if p.matchAPPMarker(raw, markerOffset) {
			break
		}
	}

	p.API.LogInfo(fmt.Sprintf("Found APP1 section in %d", markerOffset))

	// Create a new buffer after the APP1 markers to read the rest of the headers from.
	buff := bytes.NewBuffer(raw[markerOffset+2:])
	dataLengthBytes := make([]byte, 2)
	for k := range dataLengthBytes {
		c, _ := buff.ReadByte()
		dataLengthBytes[k] = c
	}
	dataLength = int(binary.BigEndian.Uint16(dataLengthBytes)) - 2

	// check exif identifier (four bytes for 'EXIF' and two padding bytes.)
	exifHeader := make([]byte, 6)
	_, err = buff.Read(exifHeader)
	if !bytes.Equal(exifHeader, append([]byte("Exif"), 0x00, 0x00)) || err != nil {
		p.API.LogError("An error occurred while attempting to find EXIF ident code.")
		err = fmt.Errorf("an error occurred while attempting to find EXIF ident code: %v", err)
		return
	}

	// Read byte order from TIFF Header.
	bo := make([]byte, 2)
	if _, err = buff.Read(bo); err != nil {
		p.API.LogError("An error occurred while attempting to find TIFF header.")
		err = fmt.Errorf("an error occurred while attempting to find TIFF header: %v", err)
		return
	}

	// Either "II" (0x4949) - LittleEndian
	// or "MM" (0x4d4d) - BigEndian
	// depending on the machine.
	// (See: http://www.cipa.jp/std/documents/e/DC-008-2012_E.pdf p.19 for details.)
	if string(bo) == "II" {
		byteOrder = binary.LittleEndian
	} else if string(bo) == "MM" {
		byteOrder = binary.BigEndian
	} else {
		p.API.LogError("Could not read tiff byte order from tiff header.")
		err = fmt.Errorf("could not read tiff byte order from tiff header.")
		return
	}

	// The TIFF header keeps a 2-byte number (0x002A) as padding.
	fixedNum := make([]byte, 2)
	if _, err = buff.Read(fixedNum); err != nil || byteOrder.Uint16(fixedNum) != 42 {
		p.API.LogError("An error occurred while attempting to find TIFF header.")
		err = fmt.Errorf("an error occurred while attempting to find TIFF header: %v", err)
		return
	}

	// load offset to first IFD
	ifdOffsetBytes := make([]byte, 4)
	_, err = buff.Read(ifdOffsetBytes)
	if err != nil {
		p.API.LogError("An error occurred while attempting to find the first IFD offset.")
		err = fmt.Errorf("an error occurred while attempting to find the first IFD offset: %v", err)
		return
	}

	ifdOffset = byteOrder.Uint32(ifdOffsetBytes)
	return
}

func (p *Plugin) matchAPPMarker(raw []byte, offset int) bool {
	return raw[offset] == MARKER_PREFIX && raw[offset+1] == APP1_MARKER
}
