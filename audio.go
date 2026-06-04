package main

import (
	"fmt"
	"io"
	"os/exec"
)

// AudioReader wraps the arecord subprocess and reads PCM data from its stdout.
type AudioReader struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
}

// NewAudioReader spawns the arecord subprocess with the specified sample rate.
func NewAudioReader(sampleRate int) (*AudioReader, error) {
	// Command: arecord -t raw -f S16_LE -r <sampleRate> -c 1 -q
	cmd := exec.Command("arecord", "-t", "raw", "-f", "S16_LE", "-r", fmt.Sprintf("%d", sampleRate), "-c", "1", "-q")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe for arecord: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start arecord process: %w", err)
	}

	return &AudioReader{
		cmd:    cmd,
		stdout: stdout,
	}, nil
}

// ReadSamples reads exactly `numSamples` from the arecord pipe and converts them to float64 samples in the range [-1.0, 1.0].
func (ar *AudioReader) ReadSamples(numSamples int) ([]float64, error) {
	// Each sample is 16-bit (2 bytes)
	byteBuf := make([]byte, numSamples*2)
	_, err := io.ReadFull(ar.stdout, byteBuf)
	if err != nil {
		return nil, err
	}

	samples := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		// Reconstruct 16-bit signed integer (little-endian)
		low := byteBuf[2*i]
		high := byteBuf[2*i+1]
		raw := int16(uint16(low) | (uint16(high) << 8))

		// Normalize to [-1.0, 1.0]
		samples[i] = float64(raw) / 32768.0
	}

	return samples, nil
}

// Close terminates the arecord subprocess and closes the stdout pipe.
func (ar *AudioReader) Close() {
	if ar.stdout != nil {
		ar.stdout.Close()
	}
	if ar.cmd != nil && ar.cmd.Process != nil {
		_ = ar.cmd.Process.Kill()
		_ = ar.cmd.Wait()
	}
}
