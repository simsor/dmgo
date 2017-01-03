package dmgo

import (
	"fmt"
	"sort"
)

type lcd struct {
	// not marshalled in snapshot
	framebuffer [160 * 144 * 4]byte

	// everything else marshalled

	FlipRequested bool // for whatever really draws the fb

	PastFirstFrame bool

	VideoRAM []byte

	OAM            [160]byte
	OAMForScanline []oamEntry

	// for oam sprite priority
	BGMask     [160]bool
	SpriteMask [160]bool

	ScrollY byte
	ScrollX byte
	WindowY byte
	WindowX byte

	BackgroundPaletteReg byte
	ObjectPalette0Reg    byte
	ObjectPalette1Reg    byte

	HBlankInterrupt bool
	VBlankInterrupt bool
	OAMInterrupt    bool
	LYCInterrupt    bool

	LYReg  byte
	LYCReg byte

	InVBlank     bool
	InHBlank     bool
	AccessingOAM bool
	ReadingData  bool

	// control bits
	DisplayOn                   bool
	UseUpperWindowTileMap       bool
	DisplayWindow               bool
	UseLowerBGAndWindowTileData bool
	UseUpperBGTileMap           bool
	BigSprites                  bool
	DisplaySprites              bool
	DisplayBG                   bool

	CyclesSinceLYInc       uint
	CyclesSinceVBlankStart uint

	StatIRQSignal bool
}

var lastOAMWarningCycles uint
var lastOAMWarningLine byte

func (lcd *lcd) writeOAM(addr uint16, val byte) {
	if !lcd.AccessingOAM && !lcd.ReadingData {
		lcd.OAM[addr] = val
	} else {
		if lcd.CyclesSinceLYInc != lastOAMWarningCycles || lcd.LYReg != lastOAMWarningLine {
			lastOAMWarningCycles = lcd.CyclesSinceLYInc
			lastOAMWarningLine = lcd.LYReg
			fmt.Println("TOUCHED OAM DURING USE: CyclesSinceLYInc", lcd.CyclesSinceLYInc, "LYReg", lcd.LYReg)
		}
	}
}
func (lcd *lcd) readOAM(addr uint16) byte {
	if !lcd.AccessingOAM && !lcd.ReadingData {
		return lcd.OAM[addr]
	}
	return 0xff
}

func (lcd *lcd) init() {
	lcd.AccessingOAM = true // at start of line
}

func (lcd *lcd) writeVideoRAM(addr uint16, val byte) {
	if !lcd.ReadingData {
		lcd.VideoRAM[addr] = val
	}
}
func (lcd *lcd) readVideoRAM(addr uint16) byte {
	if !lcd.ReadingData {
		return lcd.VideoRAM[addr]
	}
	return 0xff
}

func (cs *cpuState) updateStatIRQ() {
	lastSignal := cs.LCD.StatIRQSignal
	cs.LCD.StatIRQSignal = (cs.LCD.LYCInterrupt && cs.LCD.LYReg == cs.LCD.LYCReg) ||
		(cs.LCD.HBlankInterrupt && cs.LCD.InHBlank) ||
		(cs.LCD.OAMInterrupt && cs.LCD.AccessingOAM) ||
		((cs.LCD.VBlankInterrupt || cs.LCD.OAMInterrupt) && cs.LCD.InVBlank)
	if !lastSignal && cs.LCD.StatIRQSignal { // rising edge only
		cs.LCDStatIRQ = true
	}
}

// FIXME: timings will have to change for double-speed mode
// (maybe instead of counting cycles I'll count actual instruction time?)
// (or maybe it'll always be dmg cycles and gbc will just produce half as many of them?
func (lcd *lcd) runCycle(cs *cpuState) {
	if !lcd.DisplayOn {
		return
	}

	lcd.CyclesSinceLYInc++

	switch lcd.CyclesSinceLYInc {
	case 4:
		if !lcd.InVBlank {
			lcd.AccessingOAM = true
		}
		cs.updateStatIRQ()
	case 80:
		if lcd.AccessingOAM {
			lcd.parseOAMForScanline(lcd.LYReg)
			lcd.AccessingOAM = false
			lcd.ReadingData = true
		}
	case 252:
		if lcd.ReadingData {
			lcd.ReadingData = false
			lcd.InHBlank = true
			lcd.renderScanline()
			cs.updateStatIRQ()
		}
	case 456:
		lcd.CyclesSinceLYInc = 0
		lcd.InHBlank = false
		lcd.LYReg++

		if lcd.LYReg == 144 && !lcd.InVBlank {
			lcd.InVBlank = true
			cs.VBlankIRQ = true

			if lcd.PastFirstFrame {
				lcd.FlipRequested = true
			} else {
				lcd.PastFirstFrame = true
			}
		}
	}

	if lcd.InVBlank {
		lcd.CyclesSinceVBlankStart++
		if lcd.CyclesSinceVBlankStart == 456*10 {
			lcd.LYReg = 0
			lcd.InVBlank = false
			lcd.CyclesSinceLYInc = 0
			lcd.CyclesSinceVBlankStart = 0
		}
		// NOTE: TCAGBD claims the oam flag triggers this as well
		cs.updateStatIRQ()
	}
}

func (lcd *lcd) getTilePixel(tdataAddr uint16, tileNum, x, y byte) byte {
	if tdataAddr == 0x0800 { // 0x8000 relative
		tileNum = byte(int(int8(tileNum)) + 128)
	}
	mapBitY, mapBitX := y&0x07, x&0x07
	dataByteL := lcd.VideoRAM[tdataAddr+(uint16(tileNum)<<4)+(uint16(mapBitY)<<1)]
	dataByteH := lcd.VideoRAM[tdataAddr+(uint16(tileNum)<<4)+(uint16(mapBitY)<<1)+1]
	dataBitL := (dataByteL >> (7 - mapBitX)) & 0x1
	dataBitH := (dataByteH >> (7 - mapBitX)) & 0x1
	return (dataBitH << 1) | dataBitL
}
func (lcd *lcd) getTileNum(tmapAddr uint16, x, y byte) byte {
	tileNumY, tileNumX := uint16(y>>3), uint16(x>>3)
	tileNum := lcd.VideoRAM[tmapAddr+tileNumY*32+tileNumX]
	return tileNum
}

func (lcd *lcd) getBGPixel(x, y byte) byte {
	mapAddr := lcd.getBGTileMapAddr()
	dataAddr := lcd.getBGAndWindowTileDataAddr()
	tileNum := lcd.getTileNum(mapAddr, x, y)
	return lcd.getTilePixel(dataAddr, tileNum, x, y)
}

func (lcd *lcd) getWindowPixel(x, y byte) byte {
	mapAddr := lcd.getWindowTileMapAddr()
	dataAddr := lcd.getBGAndWindowTileDataAddr()
	tileNum := lcd.getTileNum(mapAddr, x, y)
	return lcd.getTilePixel(dataAddr, tileNum, x, y)
}

func (lcd *lcd) getSpritePixel(e *oamEntry, x, y byte) (byte, byte, byte, bool) {
	tileX := byte(int16(x) - e.x)
	tileY := byte(int16(y) - e.y)
	if e.xFlip() {
		tileX = 7 - tileX
	}
	if e.yFlip() {
		tileY = e.height - 1 - tileY
	}
	tileNum := e.tileNum
	if e.height == 16 {
		tileNum &^= 0x01
		if tileY >= 8 {
			tileNum++
		}
	}
	rawPixel := lcd.getTilePixel(0x0000, tileNum, tileX, tileY) // addr 8000 relative
	if rawPixel == 0 {
		return 0, 0, 0, false // transparent
	}
	palReg := lcd.ObjectPalette0Reg
	if e.palSelector() {
		palReg = lcd.ObjectPalette1Reg
	}
	palettedPixel := (palReg >> (rawPixel * 2)) & 0x03
	r, g, b := lcd.applyCustomPalette(palettedPixel)
	return r, g, b, true
}

var standardPalette = [][]byte{
	{0x00, 0x00, 0x00},
	{0x55, 0x55, 0x55},
	{0xaa, 0xaa, 0xaa},
	{0xff, 0xff, 0xff},
}

func (lcd *lcd) applyCustomPalette(val byte) (byte, byte, byte) {
	// TODO: actual custom palette choices stored in lcd
	outVal := standardPalette[3-val]
	return outVal[0], outVal[1], outVal[2]
}

// 0x8000 relative
func (lcd *lcd) getBGTileMapAddr() uint16 {
	if lcd.UseUpperBGTileMap {
		return 0x1c00
	}
	return 0x1800
}

// 0x8000 relative
func (lcd *lcd) getWindowTileMapAddr() uint16 {
	if lcd.UseUpperWindowTileMap {
		return 0x1c00
	}
	return 0x1800
}

// 0x8000 relative
func (lcd *lcd) getBGAndWindowTileDataAddr() uint16 {
	if lcd.UseLowerBGAndWindowTileData {
		return 0x0000
	}
	return 0x0800
}

type oamEntry struct {
	y         int16
	x         int16
	height    byte
	tileNum   byte
	flagsByte byte
}

func (e *oamEntry) behindBG() bool    { return e.flagsByte&0x80 != 0 }
func (e *oamEntry) yFlip() bool       { return e.flagsByte&0x40 != 0 }
func (e *oamEntry) xFlip() bool       { return e.flagsByte&0x20 != 0 }
func (e *oamEntry) palSelector() bool { return e.flagsByte&0x10 != 0 }

func yInSprite(y byte, spriteY int16, height int) bool {
	return int16(y) >= spriteY && int16(y) < spriteY+int16(height)
}
func (lcd *lcd) parseOAMForScanline(scanline byte) {
	height := 8
	if lcd.BigSprites {
		height = 16
	}

	// reslice so we don't realloc
	lcd.OAMForScanline = lcd.OAMForScanline[:0]

	// search all sprites, limit total found to 10 per scanline
	for i := 0; len(lcd.OAMForScanline) < 10 && i < 40; i++ {
		addr := i * 4
		spriteY := int16(lcd.OAM[addr]) - 16
		if yInSprite(scanline, spriteY, height) {
			lcd.OAMForScanline = append(lcd.OAMForScanline, oamEntry{
				y:         spriteY,
				x:         int16(lcd.OAM[addr+1]) - 8,
				height:    byte(height),
				tileNum:   lcd.OAM[addr+2],
				flagsByte: lcd.OAM[addr+3],
			})
		}
	}

	// NOTE: Pandocs suggest that on DMG, x coord is first sort priority,
	// oam index second, and that may be true for object draw sort order,
	// but dkland suggests indexes reign supreme for the total number of
	// drawable sprites. In that game they set x to zero to disable, and
	// dk is never drawn below those sprites because his sprites are
	// always at the front of the oam table.
	//
	// NOTE 2: After watching The Ultimate Game Boy talk, which is highly
	// recommended, my opinion here has solidified. There it's suggested
	// that the only thing that happens in oam search is the selection
	// of the top ten, and the ten are decided on based on scanline test
	// alone (well, Michael also suggests that an x != 0 test is made,
	// but he's wrong about other things in the talk, so I'm holding out
	// until I see evidence of this. it would make the setting of x to
	// zero in disabled dkland sprites make more sense, though)

	// resort to x-coord order (DMG only, CGB stops with the above)
	sort.Stable(sortableOAM(lcd.OAMForScanline))
}

type sortableOAM []oamEntry

func (s sortableOAM) Less(i, j int) bool { return s[i].x < s[j].x }
func (s sortableOAM) Len() int           { return len(s) }
func (s sortableOAM) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (lcd *lcd) renderScanline() {
	if lcd.LYReg >= 144 {
		return
	}
	lcd.fillScanline(0)

	y := lcd.LYReg

	for i := 0; i < 160; i++ {
		lcd.BGMask[i] = false
		lcd.SpriteMask[i] = false
	}

	if lcd.DisplayBG {
		bgY := y + lcd.ScrollY
		for x := byte(0); x < 160; x++ {
			bgX := x + lcd.ScrollX
			pixel := lcd.getBGPixel(bgX, bgY)
			if pixel == 0 {
				lcd.BGMask[x] = true
			}
			r, g, b := lcd.applyPalettes(pixel)
			lcd.setFramebufferPixel(x, y, r, g, b)
		}
	}
	if lcd.DisplayWindow && y >= lcd.WindowY {
		winY := y - lcd.WindowY
		winStartX := int(lcd.WindowX) - 7
		for x := winStartX; x < 160; x++ {
			if x < 0 {
				continue
			}
			pixel := lcd.getWindowPixel(byte(x-winStartX), winY)
			if pixel == 0 {
				lcd.BGMask[x] = true
			}
			r, g, b := lcd.applyPalettes(pixel)
			lcd.setFramebufferPixel(byte(x), y, r, g, b)
		}
	}

	if lcd.DisplaySprites {
		for i := range lcd.OAMForScanline {
			e := &lcd.OAMForScanline[i]
			lcd.renderSpriteAtScanline(e, y)
		}
	}
}

func (lcd *lcd) applyPalettes(rawPixel byte) (byte, byte, byte) {
	palettedPixel := (lcd.BackgroundPaletteReg >> (rawPixel * 2)) & 0x03
	return lcd.applyCustomPalette(palettedPixel)
}

func (lcd *lcd) renderSpriteAtScanline(e *oamEntry, y byte) {
	startX := byte(0)
	if e.x > 0 {
		startX = byte(e.x)
	}
	endX := byte(e.x + 8)
	for x := startX; x < endX && x < 160; x++ {
		if (!e.behindBG() || lcd.BGMask[x]) && !lcd.SpriteMask[x] {
			if r, g, b, a := lcd.getSpritePixel(e, x, y); a {
				lcd.setFramebufferPixel(x, y, r, g, b)
				lcd.SpriteMask[x] = true
			}
		}
	}
}

func (lcd *lcd) getFramebufferPixel(xByte, yByte byte) (byte, byte, byte) {
	x, y := int(xByte), int(yByte)
	yIdx := y * 160 * 4
	r := lcd.framebuffer[yIdx+x*4+0]
	g := lcd.framebuffer[yIdx+x*4+1]
	b := lcd.framebuffer[yIdx+x*4+2]
	return r, g, b
}
func (lcd *lcd) setFramebufferPixel(xByte, yByte, r, g, b byte) {
	x, y := int(xByte), int(yByte)
	yIdx := y * 160 * 4
	lcd.framebuffer[yIdx+x*4+0] = r
	lcd.framebuffer[yIdx+x*4+1] = g
	lcd.framebuffer[yIdx+x*4+2] = b
	lcd.framebuffer[yIdx+x*4+3] = 0xff
}
func (lcd *lcd) fillScanline(val byte) {
	yIdx := int(lcd.LYReg) * 160 * 4
	for x := 0; x < 160; x++ {
		lcd.framebuffer[yIdx+x*4+0] = val
		lcd.framebuffer[yIdx+x*4+1] = val
		lcd.framebuffer[yIdx+x*4+2] = val
		lcd.framebuffer[yIdx+x*4+3] = 0xff
	}
}

func (lcd *lcd) writeScrollY(val byte) {
	lcd.ScrollY = val
}
func (lcd *lcd) writeScrollX(val byte) {
	lcd.ScrollX = val
}
func (lcd *lcd) writeLycReg(val byte) {
	lcd.LYCReg = val
}
func (lcd *lcd) writeLyReg(val byte) {
	lcd.LYReg = val
}
func (lcd *lcd) writeBackgroundPaletteReg(val byte) {
	lcd.BackgroundPaletteReg = val
}
func (lcd *lcd) writeObjectPalette0Reg(val byte) {
	lcd.ObjectPalette0Reg = val
}
func (lcd *lcd) writeObjectPalette1Reg(val byte) {
	lcd.ObjectPalette1Reg = val
}
func (lcd *lcd) writeWindowY(val byte) {
	lcd.WindowY = val
}
func (lcd *lcd) writeWindowX(val byte) {
	lcd.WindowX = val
}

func (lcd *lcd) writeControlReg(val byte) {
	boolsFromByte(val,
		&lcd.DisplayOn,
		&lcd.UseUpperWindowTileMap,
		&lcd.DisplayWindow,
		&lcd.UseLowerBGAndWindowTileData,
		&lcd.UseUpperBGTileMap,
		&lcd.BigSprites,
		&lcd.DisplaySprites,
		&lcd.DisplayBG,
	)

	if !lcd.DisplayOn {
		lcd.LYReg = 0
	}
}
func (lcd *lcd) readControlReg() byte {
	return byteFromBools(
		lcd.DisplayOn,
		lcd.UseUpperWindowTileMap,
		lcd.DisplayWindow,
		lcd.UseLowerBGAndWindowTileData,
		lcd.UseUpperBGTileMap,
		lcd.BigSprites,
		lcd.DisplaySprites,
		lcd.DisplayBG,
	)
}

func (lcd *lcd) writeStatusReg(val byte) {
	boolsFromByte(val,
		nil,
		&lcd.LYCInterrupt,
		&lcd.OAMInterrupt,
		&lcd.VBlankInterrupt,
		&lcd.HBlankInterrupt,
		nil,
		nil,
		nil,
	)
}
func (lcd *lcd) readStatusReg() byte {
	return byteFromBools(
		true, // bit 7 always set
		lcd.LYCInterrupt,
		lcd.OAMInterrupt,
		lcd.VBlankInterrupt,
		lcd.HBlankInterrupt,
		lcd.DisplayOn && (lcd.LYReg == lcd.LYCReg),
		lcd.DisplayOn && (lcd.AccessingOAM || lcd.ReadingData),
		lcd.DisplayOn && (lcd.InVBlank || lcd.ReadingData),
	)
}
