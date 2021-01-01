package main

import (
	"github.com/simsor/go-kindle/kindle"
	"github.com/theinternetftw/dmgo"

	"fmt"
	"io/ioutil"
	"os"
	"time"
)

func main() {
	kindle.ClearScreen()
	go kindleKeyWorker()

	assert(len(os.Args) == 2, "usage: ./dmgo ROM_FILENAME")
	cartFilename := os.Args[1]

	cartBytes, err := ioutil.ReadFile(cartFilename)
	dieIf(err)

	assert(len(cartBytes) > 3, "cannot parse, file is too small")

	// TODO: config file instead
	devMode := fileExists("devmode")

	var emu dmgo.Emulator

	fileMagic := string(cartBytes[:3])
	if fileMagic == "GBS" {
		// nsf(e) file
		emu = dmgo.NewGbsPlayer(cartBytes, devMode)
	} else {
		// rom file

		cartInfo := dmgo.ParseCartInfo(cartBytes)

		fmt.Printf("Game title: %q\n", cartInfo.Title)
		fmt.Printf("Cart type: %d\n", cartInfo.CartridgeType)
		fmt.Printf("Cart RAM size: %d\n", cartInfo.GetRAMSize())
		fmt.Printf("Cart ROM size: %d\n", cartInfo.GetROMSize())

		emu = dmgo.NewEmulator(cartBytes, devMode)
	}

	startEmu(cartFilename, emu)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func startHeadlessEmu(emu dmgo.Emulator) {
	// FIXME: settings are for debug right now
	ticker := time.NewTicker(17 * time.Millisecond)

	for {
		emu.Step()
		if emu.FlipRequested() {
			<-ticker.C
		}
	}
}

func startEmu(filename string, emu dmgo.Emulator) {

	//snapshotPrefix := filename + ".snapshot"

	saveFilename := filename + ".sav"
	if saveFile, err := ioutil.ReadFile(saveFilename); err == nil {
		err = emu.SetCartRAM(saveFile)
		if err != nil {
			fmt.Println("error loading savefile,", err)
		} else {
			fmt.Println("loaded save!")
		}
	}

	//audio, err := glimmer.OpenAudioBuffer(2, 8192, 44100, 16, 2)
	//workingAudioBuffer := make([]byte, audio.BufferSize())
	//dieIf(err)

	newInput := dmgo.Input{}

	//frameTimer := glimmer.MakeFrameTimer(1.0 / 60.0)

	lastSaveTime := time.Now()
	lastInputPollTime := time.Now()

	count := 0
	screen := kindle.Framebuffer().Buffer()
	line := make([]byte, 2*160)
	xOffset := (600/2 - 160)
	yOffset := (800/2 - 144)

	for {

		count++
		if count == 100 {
			count = 0
			now := time.Now()

			inputDiff := now.Sub(lastInputPollTime)

			if inputDiff > 8*time.Millisecond {
				// Button input update

				newInput = dmgo.Input{
					Joypad: dmgo.Joypad{
						Sel:   currentKindleKeys.LPageNext || currentKindleKeys.LPagePrev,
						Start: currentKindleKeys.RPageNext || currentKindleKeys.RPagePrev,
						Up:    currentKindleKeys.Up,
						Down:  currentKindleKeys.Down,
						Left:  currentKindleKeys.Left,
						Right: currentKindleKeys.Right,
						A:     currentKindleKeys.Keyboard,
						B:     currentKindleKeys.Back,
					},
				}

				lastInputPollTime = time.Now()
			}
		}

		emu.UpdateInput(newInput)
		emu.Step()

		if emu.FlipRequested() {
			fb := emu.Framebuffer()
			for l := 0; l < 144; l++ {
				offset := l * 4 * 160
				for i, j := 0, 0; i < 160*4; i, j = i+4, j+2 {
					r := fb[i+offset] // we're skipping all other channels on purpose:
					// the GB only displays 4 colors, and they are 255, 170 and 85
					line[j] = ^byte(r)
					line[j+1] = ^byte(r)
				}

				copy(screen[(l*2+yOffset)*600+xOffset:], line)
				copy(screen[(l*2+1+yOffset)*600+xOffset:], line)
			}
			kindle.Framebuffer().DirtyRefresh()

			//frameTimer.WaitForFrametime()
			//if emu.InDevMode() {
			//	frameTimer.PrintStatsEveryXFrames(60 * 5)
			//}

			if time.Now().Sub(lastSaveTime) > 5*time.Second {
				ram := emu.GetCartRAM()
				if len(ram) > 0 {
					ioutil.WriteFile(saveFilename, ram, os.FileMode(0644))
					lastSaveTime = time.Now()
				}
			}
		}
	}
}

func assert(test bool, msg string) {
	if !test {
		fmt.Println(msg)
		os.Exit(1)
	}
}

func dieIf(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
