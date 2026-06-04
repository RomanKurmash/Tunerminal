package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"syscall"
	"unsafe"
)

// ANSI colors & styles
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Red    = "\033[31m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
)

// Labels represents language-specific text interfaces.
type Labels struct {
	Header   string
	Note     string
	Freq     string
	Signal   string
	Flat     string
	Sharp    string
	Silent   string
	FlatDev  string
	SharpDev string
	InTune   string
}

var translations = map[string]Labels{
	"en": {
		Header:   "CHROMATIC GUITAR TUNER",
		Note:     "Note:",
		Freq:     "Freq:",
		Signal:   "Signal:",
		Flat:     "Flat",
		Sharp:    "Sharp",
		Silent:   "Silent (---)",
		FlatDev:  "FLAT",
		SharpDev: "SHARP",
		InTune:   "OK",
	},
	"uk": {
		Header:   "ХРОМАТИЧНИЙ ГІТАРНИЙ ТЮНЕР",
		Note:     "Нота:",
		Freq:     "Частота:",
		Signal:   "Сигнал:",
		Flat:     "Нижче",
		Sharp:    "Вище",
		Silent:   "Тиша (---)",
		FlatDev:  "НИЖЧЕ",
		SharpDev: "ВИЩЕ",
		InTune:   "В ЛАД",
	},
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// getTerminalSize queries the dimensions of the active terminal session.
func getTerminalSize(tty io.Writer) (int, int, error) {
	if file, ok := tty.(*os.File); ok {
		var ws winsize
		_, _, err := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
		if err == 0 {
			return int(ws.Row), int(ws.Col), nil
		}
	}
	// Fallback to opening /dev/tty
	f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err == nil {
		defer f.Close()
		var ws winsize
		_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
		if err == 0 {
			return int(ws.Row), int(ws.Col), nil
		}
	}
	return 24, 80, fmt.Errorf("could not determine terminal size")
}

// clearRect clears a rectangle by printing spaces.
func clearRect(tty io.Writer, r, c, w, h int) {
	spaces := make([]byte, w)
	for i := range spaces {
		spaces[i] = ' '
	}
	for i := 0; i < h; i++ {
		fmt.Fprintf(tty, "\033[%d;%dH%s", r+i, c, spaces)
	}
}

// buildNoteRow constructs the note line of exactly 46 visual characters.
func buildNoteRow(label string, val string, valColor string) string {
	part1 := "  " + label
	part1Runes := []rune(part1)
	for len(part1Runes) < 14 {
		part1Runes = append(part1Runes, ' ')
	}

	valRunes := []rune(val)
	for len(valRunes) < 32 {
		valRunes = append(valRunes, ' ')
	}

	return string(part1Runes) + valColor + string(valRunes) + Dim + Cyan
}

// buildFreqRow constructs the frequency line of exactly 46 visual characters.
func buildFreqRow(label string, val string) string {
	part1 := "  " + label
	part1Runes := []rune(part1)
	for len(part1Runes) < 14 {
		part1Runes = append(part1Runes, ' ')
	}

	valRunes := []rune(val)
	for len(valRunes) < 32 {
		valRunes = append(valRunes, ' ')
	}

	return string(part1Runes) + string(valRunes)
}

// buildSignalRow constructs the signal level bar line of exactly 46 visual characters.
func buildSignalRow(label string, barStr string) string {
	part1 := "  " + label
	part1Runes := []rune(part1)
	for len(part1Runes) < 14 {
		part1Runes = append(part1Runes, ' ')
	}

	val := "[" + Green + barStr + Dim + Cyan + "]"
	
	spaces := ""
	for i := 0; i < 15; i++ {
		spaces += " "
	}

	return string(part1Runes) + val + spaces
}

// buildNeedleRow constructs the needle gauge line of exactly 46 visual characters.
func buildNeedleRow(flatLabel string, needlePos int, inTune bool, silent bool, sharpLabel string) string {
	leftBlock := "  " + flatLabel
	leftRunes := []rune(leftBlock)
	for len(leftRunes) < 9 {
		leftRunes = append(leftRunes, ' ')
	}

	rightBlock := sharpLabel + "  "
	rightRunes := []rune(rightBlock)
	for len(rightRunes) < 12 {
		rightRunes = append([]rune{' '}, rightRunes...)
	}

	gaugeWidth := 23
	center := 11

	needleBuf := ""
	for i := 0; i < gaugeWidth; i++ {
		if i == needlePos {
			if silent {
				needleBuf += fmt.Sprintf("%s▲%s", Gray, Reset+Dim+Cyan)
			} else if inTune {
				needleBuf += fmt.Sprintf("%s▲%s", Green, Reset+Dim+Cyan)
			} else {
				needleBuf += fmt.Sprintf("%s▲%s", Red, Reset+Dim+Cyan)
			}
		} else if i == center {
			if !silent && inTune {
				needleBuf += fmt.Sprintf("%s│%s", Green, Reset+Dim+Cyan)
			} else {
				needleBuf += "│"
			}
		} else {
			needleBuf += "░"
		}
	}

	return string(leftRunes) + "[" + needleBuf + "]" + string(rightRunes)
}

var (
	prevRow  int
	prevCol  int
	prevW    int
	prevH    int
	prevRows int
	prevCols int
)

// ClearActiveTuner clears any currently drawn tuner box from the terminal.
func ClearActiveTuner(tty io.Writer) {
	if prevW > 0 && prevH > 0 {
		clearRect(tty, prevRow, prevCol, prevW, prevH)
		prevW = 0
		prevH = 0
		prevRows = 0
		prevCols = 0
	}
}

// DrawCenteredTuner renders a centered watermark-like tuner to /dev/tty.
func DrawCenteredTuner(tty io.Writer, info PitchInfo, lang string) {
	rows, cols, err := getTerminalSize(tty)
	if err != nil {
		rows, cols = 24, 80 // fallback
	}

	boxW := 48
	boxH := 11 // 11 rows total

	// If the terminal window is too small, clear any old drawing and exit.
	if cols < boxW || rows < boxH {
		ClearActiveTuner(tty)
		return
	}

	startRow := (rows - boxH) / 2
	startCol := (cols - boxW) / 2

	// If the window size changed or the position shifted, clear the old drawing position first.
	if startRow != prevRow || startCol != prevCol || boxW != prevW || boxH != prevH || rows != prevRows || cols != prevCols {
		if prevW > 0 && prevH > 0 {
			shiftedRow := prevRow
			if prevRows > 0 {
				shiftedRow = rows - (prevRows - prevRow)
			}
			if shiftedRow >= 1 {
				clearRect(tty, shiftedRow, prevCol, prevW, prevH)
			}
		}
		prevRow = startRow
		prevCol = startCol
		prevW = boxW
		prevH = boxH
		prevRows = rows
		prevCols = cols
	}

	labels := translations[lang]

	// Save cursor position
	fmt.Fprint(tty, "\033[s")

	// Row 1: Top Border
	fmt.Fprintf(tty, "\033[%d;%dH%s%s┌──────────────────────────────────────────────┐%s", startRow, startCol, Dim, Cyan, Reset)

	// Row 2: Header
	headerText := labels.Header
	padding := (46 - len([]rune(headerText))) / 2
	if padding < 0 {
		padding = 0
	}
	headerLine := fmt.Sprintf("│%*s%s%*s│", padding, "", headerText, 46-padding-len([]rune(headerText)), "")
	fmt.Fprintf(tty, "\033[%d;%dH%s%s%s%s", startRow+1, startCol, Dim, Cyan, headerLine, Reset)

	// Row 3: Separator
	fmt.Fprintf(tty, "\033[%d;%dH%s%s├──────────────────────────────────────────────┤%s", startRow+2, startCol, Dim, Cyan, Reset)

	// Row 4: Note Display
	var noteStr string
	var noteColor string
	if info.Note == "---" {
		noteStr = labels.Silent
		noteColor = Dim
	} else {
		noteStr = fmt.Sprintf("%s%d", info.Note, info.Octave)
		if info.InTune {
			noteColor = Dim + Green
		} else {
			noteColor = Dim + Yellow
		}
	}
	noteLine := buildNoteRow(labels.Note, noteStr, noteColor)
	fmt.Fprintf(tty, "\033[%d;%dH%s%s│%s│%s", startRow+3, startCol, Dim, Cyan, noteLine, Reset)

	// Row 5: Freq Display
	var freqStr string
	if info.Note == "---" {
		freqStr = "0.00 Hz"
	} else {
		freqStr = fmt.Sprintf("%.2f Hz", info.Frequency)
	}
	freqLine := buildFreqRow(labels.Freq, freqStr)
	fmt.Fprintf(tty, "\033[%d;%dH%s%s│%s│%s", startRow+4, startCol, Dim, Cyan, freqLine, Reset)

	// Row 6: Signal Display
	barLen := int(info.RMS * 150)
	if barLen > 15 {
		barLen = 15
	}
	barStr := ""
	for i := 0; i < 15; i++ {
		if i < barLen {
			barStr += "■"
		} else {
			barStr += " "
		}
	}
	signalLine := buildSignalRow(labels.Signal, barStr)
	fmt.Fprintf(tty, "\033[%d;%dH%s%s│%s│%s", startRow+5, startCol, Dim, Cyan, signalLine, Reset)

	// Row 7: Blank
	fmt.Fprintf(tty, "\033[%d;%dH%s%s│                                              │%s", startRow+6, startCol, Dim, Cyan, Reset)

	// Row 8: Needle / Gauge
	center := 11
	needlePos := center
	if info.Note != "---" {
		cents := info.Cents
		if cents < -50 {
			cents = -50
		} else if cents > 50 {
			cents = 50
		}
		needlePos = int(math.Round((cents/50.0)*float64(center))) + center
	}
	needleLine := buildNeedleRow(labels.Flat, needlePos, info.InTune, info.Note == "---", labels.Sharp)
	fmt.Fprintf(tty, "\033[%d;%dH%s%s│%s│%s", startRow+7, startCol, Dim, Cyan, needleLine, Reset)

	// Row 9: Deviation
	var devText string
	if info.Note == "---" {
		devText = "--.- c"
	} else {
		if info.InTune {
			devText = fmt.Sprintf("%+6.1f c (%s)", info.Cents, labels.InTune)
		} else if info.Cents < 0 {
			devText = fmt.Sprintf("%+6.1f c (%s)", info.Cents, labels.FlatDev)
		} else {
			devText = fmt.Sprintf("%+6.1f c (%s)", info.Cents, labels.SharpDev)
		}
	}
	devPadding := (46 - len([]rune(devText))) / 2
	if devPadding < 0 {
		devPadding = 0
	}
	devLine := fmt.Sprintf("│%*s%s%*s│", devPadding, "", devText, 46-devPadding-len([]rune(devText)), "")
	fmt.Fprintf(tty, "\033[%d;%dH%s%s%s%s", startRow+8, startCol, Dim, Cyan, devLine, Reset)

	// Row 10: Bottom Border
	fmt.Fprintf(tty, "\033[%d;%dH%s%s└──────────────────────────────────────────────┘%s", startRow+9, startCol, Dim, Cyan, Reset)

	// Row 11: Hints
	hintText := "stop tuner | tuner lang"
	hintPadding := (48 - len(hintText)) / 2
	if hintPadding < 0 {
		hintPadding = 0
	}
	fmt.Fprintf(tty, "\033[%d;%dH%s%s%*s%s%*s%s", startRow+10, startCol, Dim, Cyan, hintPadding, "", hintText, 48-hintPadding-len(hintText), "", Reset)

	// Restore cursor
	fmt.Fprint(tty, "\033[u")
}
