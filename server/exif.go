package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"github.com/nimrodshn/mattermost-exif-plugin/exif"
)

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
	return p.DiscardExif(info, file, output)
}

// naiveDiscardExif attempts to decode an image file and the encode it back - by that removing the exif metdata.
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

// discardExif attempts to remove the exif IFD's from an image file.
func (p *Plugin) DiscardExif(info *model.FileInfo, file io.Reader, output io.Writer) (*model.FileInfo, string) {
	err := exif.Discard(file, output)
	if err != nil {
		return nil, fmt.Sprintf("An error occurred while trying to discard exif data: %v", err)
	}
	return info, ""
}
