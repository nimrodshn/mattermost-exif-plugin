package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/nimrodshn/mattermost-exif-plugin/exif"
)

func main() {
	path := flag.String("input", "", "Path to an image file with EXIF IFD.")
	output_path := flag.String("output", "", "Path to output image.")
	flag.Parse()

	raw, err := ioutil.ReadFile(*path)

	if err != nil {
		panic(err)
	}

	input := bytes.NewReader(raw)
	output := new(bytes.Buffer)

	err = exif.Discard(input, output)
	if err != nil {
		log.Fatalf("Error occured while discarding exif headers: %v", err)
	}
	err = ioutil.WriteFile(*output_path, output.Bytes(), os.ModePerm)
	if err != nil {
		log.Fatalf("Error while writing to output file: %v", err)
	}
}
