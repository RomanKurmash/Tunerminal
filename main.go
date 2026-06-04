package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

const (
	sampleRate      = 22050
	chunkSize       = 1024
	noiseGateThresh = 0.005
	pidFile         = "/tmp/tuner.pid"
)

type Config struct {
	Language string `json:"language"`
	Tuning   string `json:"tuning"`
}

func loadConfig() Config {
	data, err := os.ReadFile("/home/romankurmash/.config/tuner/config.json")
	if err != nil {
		// Fallback to legacy config or defaults
		lang := "uk"
		legacyLang, err := os.ReadFile("/home/romankurmash/.config/tuner/lang.conf")
		if err == nil {
			lang = string(legacyLang)
		}
		return Config{Language: lang, Tuning: "D Standard"}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{Language: "uk", Tuning: "D Standard"}
	}
	if cfg.Language == "" {
		cfg.Language = "uk"
	}
	if cfg.Tuning == "" {
		cfg.Tuning = "D Standard"
	}
	return cfg
}

func saveConfig(cfg Config) {
	_ = os.MkdirAll("/home/romankurmash/.config/tuner", 0755)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err == nil {
		_ = os.WriteFile("/home/romankurmash/.config/tuner/config.json", data, 0644)
	}
}

func getLanguage() string {
	return loadConfig().Language
}

func getTuning() string {
	return loadConfig().Tuning
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "stop":
			stopDaemon()
			return
		case "settings", "lang":
			runSettingsMenu()
			return
		case "--daemon-draw":
			runDaemonDrawMode()
			return
		case "--daemon":
			runDaemonMode()
			return
		case "--status":
			runStatusMode()
			return
		case "foreground", "-f", "--foreground":
			runForegroundMode()
			return
		case "-h", "--help":
			printHelp()
			return
		default:
			fmt.Printf("Unknown command: %s\n", os.Args[1])
			printHelp()
			return
		}
	}

	// No arguments: start the daemon in the background drawing to current terminal
	startDaemon()
}

func printHelp() {
	fmt.Println("Guitar Tuner CLI - A lightweight real-time background chromatic tuner")
	fmt.Println("\nCommands:")
	fmt.Println("  tuner                 Start the tuner in the background drawing to this terminal")
	fmt.Println("  tuner stop            Stop the background tuner (or use 'stop tuner')")
	fmt.Println("  tuner settings        Open tuner settings (Language and Tuning)")
	fmt.Println("  tuner foreground      Run tuner in the foreground")
	fmt.Println("  tuner --status        Print current tuning status once and exit")
	fmt.Println("  tuner --daemon        Run silently in background (only updates /tmp/tuner.status)")
}

func setRawMode(raw bool) {
	var cmd *exec.Cmd
	if raw {
		cmd = exec.Command("stty", "raw", "-echo")
	} else {
		cmd = exec.Command("stty", "-raw", "echo")
	}
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

func runSettingsMenu() {
	// Write pause lock file to temporarily stop background daemon drawing
	_ = os.WriteFile("/tmp/tuner.pause", []byte("1"), 0644)
	defer os.Remove("/tmp/tuner.pause")

	cfg := loadConfig()
	selectedRow := 0 // 0: Language, 1: Tuning, 2: Save, 3: Cancel

	languages := []string{"en", "uk"}
	langLabels := map[string]string{
		"en": "English",
		"uk": "Українська",
	}

	tunings := []string{"E Standard", "D Standard", "Drop D", "Drop C"}

	getLangIdx := func(l string) int {
		for i, v := range languages {
			if v == l {
				return i
			}
		}
		return 0
	}
	getTuningIdx := func(t string) int {
		for i, v := range tunings {
			if v == t {
				return i
			}
		}
		return 0
	}

	langIdx := getLangIdx(cfg.Language)
	tuningIdx := getTuningIdx(cfg.Tuning)

	setRawMode(true)
	defer setRawMode(false)

	// Clear screen and hide cursor
	fmt.Print("\033[H\033[2J\033[?25l")

	for {
		fmt.Print("\033[H")
		fmt.Print("=== TUNER SETTINGS / НАЛАШТУВАННЯ ТЮНЕРА ===\r\n")
		fmt.Print("--------------------------------------------\r\n")

		// Row 0: Language
		langLabel := "  Language / Мова: "
		if selectedRow == 0 {
			fmt.Printf(" > Language / Мова:   < %s >\r\n", langLabels[languages[langIdx]])
		} else {
			fmt.Printf("%s    %s\r\n", langLabel, langLabels[languages[langIdx]])
		}

		// Row 1: Tuning
		tuningLabel := "  Tuning / Стрій:   "
		if selectedRow == 1 {
			fmt.Printf(" > Tuning / Стрій:     < %s >\r\n", tunings[tuningIdx])
		} else {
			fmt.Printf("%s    %s\r\n", tuningLabel, tunings[tuningIdx])
		}

		fmt.Print("\r\n")

		// Row 2: Save
		if selectedRow == 2 {
			fmt.Print(" > [ Save & Exit / Зберегти та вийти ]\r\n")
		} else {
			fmt.Print("   [ Save & Exit / Зберегти та вийти ]\r\n")
		}

		// Row 3: Cancel
		if selectedRow == 3 {
			fmt.Print(" > [ Cancel / Скасувати ]\r\n")
		} else {
			fmt.Print("   [ Cancel / Скасувати ]\r\n")
		}

		fmt.Print("--------------------------------------------\r\n")
		fmt.Print("Use Up/Down Arrow keys to navigate.\r\n")
		fmt.Print("Використовуйте стрілки Вгору/Вниз для навігації.\r\n")
		fmt.Print("Use Left/Right Arrow keys to change values.\r\n")
		fmt.Print("Використовуйте стрілки Вліво/Вправо для зміни значень.\r\n")
		fmt.Print("\r\nPress Escape or Ctrl+C to cancel / Escape або Ctrl+C для скасування.\r\n")

		var buf [3]byte
		n, err := os.Stdin.Read(buf[:])
		if err != nil {
			break
		}

		if n == 1 {
			if buf[0] == '\r' || buf[0] == '\n' {
				// Enter pressed
				if selectedRow == 2 {
					// Save
					cfg.Language = languages[langIdx]
					cfg.Tuning = tunings[tuningIdx]
					saveConfig(cfg)
					if cfg.Language == "uk" {
						fmt.Print("\033[H\033[2J\r\nНалаштування збережено!\r\n")
					} else {
						fmt.Print("\033[H\033[2J\r\nSettings saved!\r\n")
					}
					break
				} else if selectedRow == 3 {
					// Cancel
					if cfg.Language == "uk" {
						fmt.Print("\033[H\033[2JСкасовано.\r\n")
					} else {
						fmt.Print("\033[H\033[2JCancelled.\r\n")
					}
					break
				}
			}
			if buf[0] == 3 || buf[0] == 27 { // Ctrl+C or Escape
				if cfg.Language == "uk" {
					fmt.Print("\033[H\033[2JСкасовано.\r\n")
				} else {
					fmt.Print("\033[H\033[2JCancelled.\r\n")
				}
				break
			}
		} else if n == 3 && buf[0] == '\033' && buf[1] == '[' {
			if buf[2] == 'A' { // Up
				selectedRow = (selectedRow - 1 + 4) % 4
			} else if buf[2] == 'B' { // Down
				selectedRow = (selectedRow + 1) % 4
			} else if buf[2] == 'D' { // Left
				if selectedRow == 0 {
					langIdx = (langIdx - 1 + len(languages)) % len(languages)
				} else if selectedRow == 1 {
					tuningIdx = (tuningIdx - 1 + len(tunings)) % len(tunings)
				}
			} else if buf[2] == 'C' { // Right
				if selectedRow == 0 {
					langIdx = (langIdx + 1) % len(languages)
				} else if selectedRow == 1 {
					tuningIdx = (tuningIdx + 1) % len(tunings)
				}
			}
		}
	}
	// Clear menu area and restore cursor
	fmt.Print("\033[H\033[2J\033[?25h\033[0m")
}

func startDaemon() {
	// Check if already running
	if data, err := os.ReadFile(pidFile); err == nil {
		var oldPid int
		if _, err := fmt.Sscanf(string(data), "%d", &oldPid); err == nil {
			process, err := os.FindProcess(oldPid)
			if err == nil {
				// Send signal 0 to verify existence
				if err = process.Signal(syscall.Signal(0)); err == nil {
					fmt.Println("Tuner is already running in background.")
					return
				}
			}
		}
	}

	// Fork itself
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	var files []*os.File
	if err == nil {
		files = []*os.File{nil, tty, tty}
	} else {
		files = []*os.File{nil, os.Stdout, os.Stderr}
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Error starting tuner:", err)
		return
	}

	attr := &os.ProcAttr{
		Files: files,
	}

	process, err := os.StartProcess(exe, []string{exe, "--daemon-draw"}, attr)
	if err != nil {
		fmt.Println("Error starting background tuner:", err)
		return
	}

	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", process.Pid)), 0644)
	
	lang := getLanguage()
	if lang == "uk" {
		fmt.Printf("Тюнер запущено у фоні (PID: %d).\n", process.Pid)
		fmt.Println("Візуалізація з'явиться в центрі цього терміналу.")
		fmt.Println("Щоб зупинити, введіть 'stop tuner' або 'tuner stop'.")
	} else {
		fmt.Printf("Tuner started in background (PID: %d).\n", process.Pid)
		fmt.Println("Visualization will appear in the center of this terminal.")
		fmt.Println("To stop, type 'stop tuner' or 'tuner stop'.")
	}
}

func stopDaemon() {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("Tuner is not running.")
		return
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		fmt.Println("Invalid PID file.")
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("Tuner process not found.")
		return
	}

	// Attempt to clear drawing on /dev/tty
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err == nil {
		rows, cols, err := getTerminalSize(tty)
		if err == nil {
			boxW := 48
			boxH := 11
			startRow := (rows - boxH) / 2
			startCol := (cols - boxW) / 2
			clearRect(tty, startRow, startCol, boxW, boxH)
		}
		tty.Close()
	}

	_ = process.Signal(syscall.SIGTERM)
	_ = os.Remove(pidFile)
	_ = os.Remove("/tmp/tuner.status")
	_ = os.Remove("/tmp/tuner.json")
	fmt.Println("Tuner stopped.")
}

func runDaemonDrawMode() {
	ar, err := NewAudioReader(sampleRate)
	if err != nil {
		return
	}
	defer ar.Close()

	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		tty = os.Stdout
	}
	defer tty.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	buffer := make([]float64, 2048)
	initial, err := ar.ReadSamples(2048)
	if err != nil {
		return
	}
	copy(buffer, initial)

	go func() {
		<-sigChan
		ClearActiveTuner(tty)
		os.Exit(0)
	}()

	for {
		newSamples, err := ar.ReadSamples(chunkSize)
		if err != nil {
			return
		}

		copy(buffer[0:1024], buffer[1024:2048])
		copy(buffer[1024:2048], newSamples)

		rms := CalculateRMS(buffer)
		var info PitchInfo

		if rms > noiseGateThresh {
			freq := DetectPitch(buffer, sampleRate)
			info = FrequencyToPitch(freq, rms)
		} else {
			info = FrequencyToPitch(0, rms)
		}

		// If paused by a foreground command (like tuner lang), skip drawing to /dev/tty
		if _, err := os.Stat("/tmp/tuner.pause"); err == nil {
			continue
		}

		cfg := loadConfig()
		DrawCenteredTuner(tty, info, cfg.Language, cfg.Tuning)
		writeStatusAtomically(info)
	}
}

func runForegroundMode() {
	// Write pause lock file to temporarily stop background daemon drawing
	_ = os.WriteFile("/tmp/tuner.pause", []byte("1"), 0644)
	defer os.Remove("/tmp/tuner.pause")

	ar, err := NewAudioReader(sampleRate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not start arecord. Is microphone connected? Error: %v\n", err)
		os.Exit(1)
	}
	defer ar.Close()

	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		tty = os.Stdout
	}
	defer tty.Close()

	// Hide cursor and clear
	fmt.Fprint(tty, "\033[H\033[2J\033[?25l")
	defer fmt.Fprint(tty, "\033[?25h\033[0m")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		ClearActiveTuner(tty)
		fmt.Fprint(tty, "\033[?25h\033[0m")
		os.Exit(0)
	}()

	buffer := make([]float64, 2048)
	initial, err := ar.ReadSamples(2048)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading initial audio samples: %v\n", err)
		os.Exit(1)
	}
	copy(buffer, initial)

	for {
		newSamples, err := ar.ReadSamples(chunkSize)
		if err != nil {
			break
		}

		copy(buffer[0:1024], buffer[1024:2048])
		copy(buffer[1024:2048], newSamples)

		rms := CalculateRMS(buffer)
		var info PitchInfo

		if rms > noiseGateThresh {
			freq := DetectPitch(buffer, sampleRate)
			info = FrequencyToPitch(freq, rms)
		} else {
			info = FrequencyToPitch(0, rms)
		}

		cfg := loadConfig()
		DrawCenteredTuner(tty, info, cfg.Language, cfg.Tuning)
	}
}

func runStatusMode() {
	statusBytes, err := os.ReadFile("/tmp/tuner.status")
	if err != nil {
		fmt.Println("Silent (---)")
		return
	}
	fmt.Print(string(statusBytes))
}

func runDaemonMode() {
	ar, err := NewAudioReader(sampleRate)
	if err != nil {
		os.Exit(1)
	}
	defer ar.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		os.Exit(0)
	}()

	buffer := make([]float64, 2048)
	initial, err := ar.ReadSamples(2048)
	if err != nil {
		os.Exit(1)
	}
	copy(buffer, initial)

	for {
		newSamples, err := ar.ReadSamples(chunkSize)
		if err != nil {
			return
		}

		copy(buffer[0:1024], buffer[1024:2048])
		copy(buffer[1024:2048], newSamples)

		rms := CalculateRMS(buffer)
		var info PitchInfo

		if rms > noiseGateThresh {
			freq := DetectPitch(buffer, sampleRate)
			info = FrequencyToPitch(freq, rms)
		} else {
			info = FrequencyToPitch(0, rms)
		}

		writeStatusAtomically(info)
	}
}

func writeStatusAtomically(info PitchInfo) {
	jsonBytes, err := json.Marshal(info)
	if err == nil {
		_ = writeTmpAndRename(jsonBytes, "/tmp/tuner.json")
	}

	var statusStr string
	if info.Note == "---" {
		statusStr = "Silent (---)\n"
	} else {
		sign := ""
		if info.Cents > 0 {
			sign = "+"
		}
		okStr := ""
		if info.InTune {
			okStr = " (OK)"
		}
		statusStr = fmt.Sprintf("%s%d %s%.1fc%s\n", info.Note, info.Octave, sign, info.Cents, okStr)
	}
	_ = writeTmpAndRename([]byte(statusStr), "/tmp/tuner.status")
}

func writeTmpAndRename(data []byte, targetPath string) error {
	tmpPath := targetPath + ".tmp"
	err := os.WriteFile(tmpPath, data, 0644)
	if err != nil {
		return err
	}
	return os.Rename(tmpPath, targetPath)
}
