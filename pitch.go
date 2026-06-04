package main

import (
	"math"
)

// PitchInfo contains the details of the detected pitch.
type PitchInfo struct {
	Note      string  `json:"note"`
	Octave    int     `json:"octave"`
	Frequency float64 `json:"frequency"`
	Cents     float64 `json:"cents"`
	InTune    bool    `json:"in_tune"`
	RMS       float64 `json:"rms"`
}

var noteNames = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// FrequencyToPitch converts a frequency in Hz to a chromatic note name, octave, and cents deviation.
func FrequencyToPitch(freq float64, rms float64) PitchInfo {
	if freq <= 0 {
		return PitchInfo{Note: "---", Frequency: 0, Cents: 0, InTune: false, RMS: rms}
	}

	// n = 12 * log2(f / 440)
	n := 12.0 * math.Log2(freq/440.0)
	nRound := math.Round(n)
	cents := (n - nRound) * 100.0

	// MIDI note 69 is A4 (440 Hz)
	midi := int(nRound) + 69
	if midi < 0 {
		return PitchInfo{Note: "---", Frequency: freq, Cents: 0, InTune: false, RMS: rms}
	}

	noteIdx := (midi % 12 + 12) % 12
	octave := midi/12 - 1

	return PitchInfo{
		Note:      noteNames[noteIdx],
		Octave:    octave,
		Frequency: freq,
		Cents:     cents,
		InTune:    math.Abs(cents) <= 3.0, // standard tuner tolerance
		RMS:       rms,
	}
}

// CalculateRMS computes the root-mean-square amplitude of the buffer.
// This is used for noise gating (ignoring silence).
func CalculateRMS(buffer []float64) float64 {
	sum := 0.0
	for _, val := range buffer {
		sum += val * val
	}
	return math.Sqrt(sum / float64(len(buffer)))
}

// DetectPitch uses the YIN algorithm to estimate the fundamental frequency of the buffer.
// It returns 0.0 if no clear pitch is detected.
func DetectPitch(buffer []float64, sampleRate float64) float64 {
	W := 1024 // window size
	if len(buffer) < 2048 {
		return 0.0
	}

	tauMin := 20  // ~1100 Hz
	tauMax := 400 // ~55.1 Hz
	if tauMax > len(buffer)-W {
		tauMax = len(buffer) - W
	}

	// Step 1: Difference Function
	diff := make([]float64, tauMax)
	for tau := 0; tau < tauMax; tau++ {
		sum := 0.0
		for i := 0; i < W; i++ {
			d := buffer[i] - buffer[i+tau]
			sum += d * d
		}
		diff[tau] = sum
	}

	// Step 2: Cumulative Mean Normalized Difference Function (CMNDF)
	dPrime := make([]float64, tauMax)
	dPrime[0] = 1.0
	runningSum := 0.0
	for tau := 1; tau < tauMax; tau++ {
		runningSum += diff[tau]
		if runningSum > 0 {
			dPrime[tau] = diff[tau] / (runningSum / float64(tau))
		} else {
			dPrime[tau] = 1.0
		}
	}

	// Step 3: Absolute Thresholding
	threshold := 0.15
	tauBest := -1

	// Search for the first local minimum below the threshold
	for tau := tauMin; tau < tauMax-1; tau++ {
		if dPrime[tau] < threshold {
			// Local minimum check
			if dPrime[tau] < dPrime[tau-1] && dPrime[tau] < dPrime[tau+1] {
				tauBest = tau
				break
			}
		}
	}

	// If no local minimum fell below threshold, fallback to global minimum
	if tauBest == -1 {
		minVal := 1.0
		for tau := tauMin; tau < tauMax; tau++ {
			if dPrime[tau] < minVal {
				minVal = dPrime[tau]
				tauBest = tau
			}
		}
	}

	// Check if we found a valid lag
	if tauBest < tauMin || tauBest >= tauMax-1 {
		return 0.0
	}

	// Step 4: Parabolic Interpolation for sub-sample accuracy
	a := dPrime[tauBest-1]
	b := dPrime[tauBest]
	c := dPrime[tauBest+1]

	denominator := 2.0 * (a - 2.0*b + c)
	offset := 0.0
	if denominator != 0 {
		offset = (a - c) / denominator
	}

	tauRefined := float64(tauBest) + offset
	freq := sampleRate / tauRefined

	// Limit to reasonable guitar pitch range (E2 is 82.4Hz, high E4 is 329.6Hz, 12th fret is 659.2Hz)
	if freq >= 60.0 && freq <= 1000.0 {
		return freq
	}

	return 0.0
}
