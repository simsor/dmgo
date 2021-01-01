package main

import (
	"os"

	"github.com/simsor/go-kindle/kindle"
)

type kindleKeysState struct {
	Up    bool
	Down  bool
	Left  bool
	Right bool
	OK    bool

	Back     bool
	Keyboard bool
	Menu     bool
	Home     bool

	LPagePrev bool
	LPageNext bool
	RPagePrev bool
	RPageNext bool
}

var currentKindleKeys kindleKeysState

func kindleKeyWorker() {
	for {
		updateKeyState()
	}
}

func updateKeyState() {
	k := kindle.WaitForKey()
	v := k.IsPressed()

	switch k.KeyCode {
	case kindle.Up:
		currentKindleKeys.Up = v
	case kindle.Down:
		currentKindleKeys.Down = v
	case kindle.Left:
		currentKindleKeys.Left = v
	case kindle.Right:
		currentKindleKeys.Right = v
	case kindle.Back:
		currentKindleKeys.Back = v
	case kindle.Keyboard:
		currentKindleKeys.Keyboard = v
	case kindle.Menu:
		currentKindleKeys.Menu = v
	case kindle.LPagePrev:
		currentKindleKeys.LPagePrev = v
	case kindle.LPageNext:
		currentKindleKeys.LPageNext = v
	case kindle.RPagePrev:
		currentKindleKeys.RPagePrev = v
	case kindle.RPageNext:
		currentKindleKeys.RPageNext = v
	case kindle.Home:
		os.Exit(0)
	}
}
