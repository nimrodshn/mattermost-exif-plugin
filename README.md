# mattermost-exif-plugin [![Build Status](https://travis-ci.org/mattermost/mattermost-plugin-sample.svg?branch=master)](https://travis-ci.org/mattermost/mattermost-plugin-sample)

This plugin removes any EXIF data on image file uploaded to Mattermost channels. **Please Note:** mattermost-exif-plugin currently only supports compressed JPEG files.

To build the plugin run `make` and upload the zipped plugin to Mattermost using the `System Console->Plugin->Management` screen.


## Exif Remover
This plugin comes with a small cli tool to remove exif IFD's: `exif-remover`.
To compile it simply run `make exif-remover`.

To run `exif-remover` on a given image simply run:
```
exif-remover --input=/path/to/input/image.jpg --output=/path/to/output/image.jpg
```