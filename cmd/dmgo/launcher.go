package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/simsor/go-kindle/kindle"
)

var (
	txtWidth        = 50
	txtHeight       = 40
	menuStartOffset = 6
	skipFrame       = 2
)

func main() {
	kindle.ClearScreen()

	msg := "Kindle Gameboy Emulator"
	kindle.DrawText(txtWidth/2-len(msg)/2, 1, msg)

	msg = strings.Repeat("=", len(msg))
	kindle.DrawText(txtWidth/2-len(msg)/2, 2, msg)

	kindle.DrawText(0, 4, "Select your game: <usb:/gameboy/>")

	kindle.DrawText(2, txtHeight-4, "Frame skip value: ")
	kindle.DrawText(2, txtHeight-3, "(Use Page Next / Prev to change)")
	updateFrameSkip()

	roms := getGameList()

	yOffset := menuStartOffset
	for _, rom := range roms {
		romName := filepath.Base(rom.Name())
		kindle.DrawText(4, yOffset, romName)
		yOffset++
	}

	selection := updateSelection(0, 0)

	for {
		key := kindle.WaitForKey()

		if key.State != 1 {
			continue
		}

		switch key.KeyCode {
		case kindle.Down:
			if selection < len(roms)-1 {
				selection = updateSelection(selection, selection+1)
			}

		case kindle.Up:
			if selection > 0 {
				selection = updateSelection(selection, selection-1)
			}

		case kindle.OK:
			dmgoMain(filepath.Join("/mnt/us/gameboy", roms[selection].Name()))

		case kindle.LPageNext:
			fallthrough
		case kindle.RPageNext:
			skipFrame++
			updateFrameSkip()

		case kindle.LPagePrev:
			fallthrough
		case kindle.RPagePrev:
			skipFrame--
			if skipFrame < 1 {
				skipFrame = 1
			}
			updateFrameSkip()

		case kindle.Home:
			kindle.ClearScreen()
			os.Exit(0)
		}
	}
}

func getGameList() []os.FileInfo {
	files, err := ioutil.ReadDir("/mnt/us/gameboy")
	if err != nil {
		kindle.DrawText(0, txtHeight-2, "Could not open dir:")
		kindle.DrawText(0, txtHeight-1, err.Error())
		return nil
	}

	var gbFiles []os.FileInfo

	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f.Name()), ".gb") {
			gbFiles = append(gbFiles, f)
		}
	}

	return gbFiles
}

func updateSelection(old, new int) int {
	kindle.DrawText(0, old+menuStartOffset, "   ")
	kindle.DrawText(0, new+menuStartOffset, ">>>")
	return new
}

func updateFrameSkip() {
	kindle.DrawText(20, txtHeight-4, "    ")
	kindle.DrawText(20, txtHeight-4, strconv.Itoa(skipFrame))
}
