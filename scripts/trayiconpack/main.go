package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image/png"
	"os"
)

type iconImage struct {
	width  int
	height int
	data   []byte
}

func main() {
	output := flag.String("output", "", "destination .ico file")
	flag.Parse()
	if *output == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: trayiconpack -output icon.ico icon-16.png ...")
		os.Exit(2)
	}

	images := make([]iconImage, 0, flag.NArg())
	for _, path := range flag.Args() {
		file, err := os.Open(path)
		if err != nil {
			fatal(err)
		}
		config, err := png.DecodeConfig(file)
		_ = file.Close()
		if err != nil {
			fatal(fmt.Errorf("decode %s: %w", path, err))
		}
		if config.Width < 1 || config.Width > 256 || config.Height < 1 || config.Height > 256 {
			fatal(fmt.Errorf("%s has unsupported ICO dimensions %dx%d", path, config.Width, config.Height))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fatal(err)
		}
		images = append(images, iconImage{width: config.Width, height: config.Height, data: data})
	}

	file, err := os.Create(*output)
	if err != nil {
		fatal(err)
	}
	defer file.Close()

	mustWrite(file, uint16(0))
	mustWrite(file, uint16(1))
	mustWrite(file, uint16(len(images)))

	offset := uint32(6 + 16*len(images))
	for _, image := range images {
		width := byte(image.width)
		height := byte(image.height)
		if image.width == 256 {
			width = 0
		}
		if image.height == 256 {
			height = 0
		}
		mustWrite(file, width)
		mustWrite(file, height)
		mustWrite(file, byte(0))
		mustWrite(file, byte(0))
		mustWrite(file, uint16(1))
		mustWrite(file, uint16(32))
		mustWrite(file, uint32(len(image.data)))
		mustWrite(file, offset)
		offset += uint32(len(image.data))
	}
	for _, image := range images {
		if _, err := file.Write(image.data); err != nil {
			fatal(err)
		}
	}
}

func mustWrite(file *os.File, value any) {
	if err := binary.Write(file, binary.LittleEndian, value); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
