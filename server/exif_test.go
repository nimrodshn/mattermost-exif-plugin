package main

import (
	"bytes"
	"testing"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

func TestDiscardExif(t *testing.T) {
	p := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{},
	}

	testTable := []struct {
		Input  []byte
		Output []byte
	}{
		{
			Input: []byte{
				0x00, 0x00,
				0xFF, 0xE1, // Markers
				0x00, 0x0F,
				'E', 'x', 'i', 'f', 0x00, 0x00, // EXIF identifier.
				0x4d, 0x4d, // "MM" - Big Endian.
				0x00, 0x2A, // Fixed 2-bytes.
				0x00, 0x00, 0x00, 0x14, // Offset twenty to first IFD.
				0x00, 0x01, // One tag.
				0x00, 0x00, // Remove bytes from this part onwards.
				0x00, 0x00,
				0x00, 0x00,
				0x00, 0x00,
				0x00, 0x00,
				0x00, 0x00,
				0x00, 0x00,
				0x00, 0x00,
				0xFF, 0xFF,
			},
			Output: []byte{
				0x00, 0x00,
				0xFF, 0xE1, // Markers
				0x00, 0x0F,
				'E', 'x', 'i', 'f', 0x00, 0x00, // EXIF identifier.
				0x4d, 0x4d, // "II" - Litte Endian.
				0x00, 0x2A, // Fixed 2-bytes.
				0x00, 0x00, 0x00, 0x14,
				0xFF, 0xFF,
			},
		},
	}

	for _, test := range testTable {
		buff := bytes.NewBuffer(test.Input)
		result := make([]byte, 0)
		resultWriter := bytes.NewBuffer(result)

		_, str := p.DiscardExif(&model.FileInfo{}, buff, resultWriter)

		if str != "" {
			t.Errorf("Expected string to be empty instead recieved: %s", str)
		}

		if !bytes.Equal(test.Output, resultWriter.Bytes()) {
			t.Errorf("Expected result to be: %s instead got: %s", string(test.Output), string(resultWriter.Bytes()))
		}
	}
}
