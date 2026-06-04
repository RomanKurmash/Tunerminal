package main

import (
	"math"
	"testing"
)

// generateSineWave generates a sine wave at the specified frequency and sample rate.
func generateSineWave(freq float64, sampleRate float64, length int) []float64 {
	buffer := make([]float64, length)
	for i := 0; i < length; i++ {
		t := float64(i) / sampleRate
		buffer[i] = math.Sin(2.0 * math.Pi * freq * t)
	}
	return buffer
}

func TestPitchDetection(t *testing.T) {
	sampleRate := 22050.0
	bufferLen := 2048

	tests := []struct {
		freq         float64
		expectedNote string
	}{
		{73.42, "D"},  // D2 (low D string)
		{98.00, "G"},  // G2 (string 5)
		{130.81, "C"}, // C3 (string 4)
		{174.61, "F"}, // F3 (string 3)
		{220.00, "A"}, // A3 (string 2)
		{293.66, "D"}, // D4 (high D string)
		{440.00, "A"}, // A4 (calibration pitch)
	}

	for _, tt := range tests {
		buf := generateSineWave(tt.freq, sampleRate, bufferLen)
		detectedFreq := DetectPitch(buf, sampleRate)
		if detectedFreq == 0.0 {
			t.Errorf("Failed to detect pitch for frequency %.2f Hz", tt.freq)
			continue
		}

		rms := CalculateRMS(buf)
		pitchInfo := FrequencyToPitch(detectedFreq, rms)

		if pitchInfo.Note != tt.expectedNote {
			t.Errorf("For frequency %.2f Hz: expected note %s, got %s (detected freq: %.2f Hz)",
				tt.freq, tt.expectedNote, pitchInfo.Note, detectedFreq)
		} else {
			t.Logf("Success: %.2f Hz -> Note: %s%d (detected: %.2f Hz, cents error: %.2f)",
				tt.freq, pitchInfo.Note, pitchInfo.Octave, detectedFreq, pitchInfo.Cents)
		}
	}
}

func TestSilence(t *testing.T) {
	buffer := make([]float64, 2048) // all zeros
	detectedFreq := DetectPitch(buffer, 22050.0)
	if detectedFreq != 0.0 {
		t.Errorf("Expected 0.0 for silence, got %.2f Hz", detectedFreq)
	}
}

func TestStringMatching(t *testing.T) {
	tests := []struct {
		note     string
		octave   int
		tuning   string
		expected int
	}{
		{"E", 2, "E Standard", 6},
		{"A", 2, "E Standard", 5},
		{"D", 3, "E Standard", 4},
		{"G", 3, "E Standard", 3},
		{"B", 3, "E Standard", 2},
		{"E", 4, "E Standard", 1},
		{"F", 2, "E Standard", 0}, // not a string

		{"D", 2, "D Standard", 6},
		{"G", 2, "D Standard", 5},
		{"C", 3, "D Standard", 4},
		{"F", 3, "D Standard", 3},
		{"A", 3, "D Standard", 2},
		{"D", 4, "D Standard", 1},

		{"D", 2, "Drop D", 6},
		{"A", 2, "Drop D", 5},
		{"E", 4, "Drop D", 1},

		{"C", 2, "Drop C", 6},
		{"G", 2, "Drop C", 5},
		{"D", 4, "Drop C", 1},
	}

	for _, tt := range tests {
		got := GetStringNumber(tt.note, tt.octave, tt.tuning)
		if got != tt.expected {
			t.Errorf("GetStringNumber(%q, %d, %q) = %d; expected %d",
				tt.note, tt.octave, tt.tuning, got, tt.expected)
		}
	}
}
