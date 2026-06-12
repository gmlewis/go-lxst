// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package filters provides audio filtering (HighPass, LowPass, BandPass, AGC).
// Pure Go implementation matching Python LXST Filters.py fallback logic.
package filters

import (
	"math"
)

// Filter is the base interface for audio filters.
type Filter interface {
	HandleFrame(frame [][]float32, samplerate int) [][]float32
}

// HighPass implements a high-pass filter.
type HighPass struct {
	cut          float64
	samplerate   int
	channels     int
	filterStates []float32
	lastInputs   []float32
	alpha        float32
	output       [][]float32
	outputFlat   []float32
}

// NewHighPass creates a new HighPass filter with the given cutoff frequency.
func NewHighPass(cut float64) *HighPass {
	return &HighPass{
		cut: cut,
	}
}

func (h *HighPass) HandleFrame(frame [][]float32, samplerate int) [][]float32 {
	if len(frame) == 0 {
		return frame
	}

	samples := len(frame)
	if samples == 0 {
		return frame
	}
	channels := len(frame[0])

	if samplerate != h.samplerate {
		h.samplerate = samplerate
		dt := 1.0 / float64(h.samplerate)
		rc := 1.0 / (2 * math.Pi * h.cut)
		h.alpha = float32(rc / (rc + dt))
	}

	if h.filterStates == nil || h.channels != channels {
		h.channels = channels
		h.filterStates = make([]float32, channels)
		h.lastInputs = make([]float32, channels)
	}

	// Reuse output buffer
	if cap(h.outputFlat) < samples*channels {
		h.outputFlat = make([]float32, samples*channels)
		h.output = make([][]float32, samples)
		for i := range h.output {
			h.output[i] = h.outputFlat[i*channels : (i+1)*channels]
		}
	} else {
		h.output = h.output[:samples]
		for i := 0; i < samples; i++ {
			h.output[i] = h.outputFlat[i*channels : (i+1)*channels]
		}
	}

	// First sample
	for ch := 0; ch < channels; ch++ {
		inputDiff := frame[0][ch] - h.lastInputs[ch]
		h.output[0][ch] = h.alpha * (h.filterStates[ch] + inputDiff)
	}

	// Remaining samples
	for i := 1; i < samples; i++ {
		for ch := 0; ch < channels; ch++ {
			inputDiff := frame[i][ch] - frame[i-1][ch]
			h.output[i][ch] = h.alpha * (h.output[i-1][ch] + inputDiff)
		}
	}

	// Update states
	for ch := 0; ch < channels; ch++ {
		h.filterStates[ch] = h.output[samples-1][ch]
		h.lastInputs[ch] = frame[samples-1][ch]
	}

	return h.output
}

// LowPass implements a low-pass filter.
type LowPass struct {
	cut          float64
	samplerate   int
	channels     int
	filterStates []float32
	alpha        float32
	output       [][]float32
	outputFlat   []float32
}

// NewLowPass creates a new LowPass filter with the given cutoff frequency.
func NewLowPass(cut float64) *LowPass {
	return &LowPass{
		cut: cut,
	}
}

func (l *LowPass) HandleFrame(frame [][]float32, samplerate int) [][]float32 {
	if len(frame) == 0 {
		return frame
	}

	samples := len(frame)
	if samples == 0 {
		return frame
	}
	channels := len(frame[0])

	if samplerate != l.samplerate {
		l.samplerate = samplerate
		dt := 1.0 / float64(l.samplerate)
		rc := 1.0 / (2 * math.Pi * l.cut)
		l.alpha = float32(dt / (rc + dt))
	}

	if l.filterStates == nil || l.channels != channels {
		l.channels = channels
		l.filterStates = make([]float32, channels)
	}

	// Reuse output buffer
	if cap(l.outputFlat) < samples*channels {
		l.outputFlat = make([]float32, samples*channels)
		l.output = make([][]float32, samples)
		for i := range l.output {
			l.output[i] = l.outputFlat[i*channels : (i+1)*channels]
		}
	} else {
		l.output = l.output[:samples]
		for i := 0; i < samples; i++ {
			l.output[i] = l.outputFlat[i*channels : (i+1)*channels]
		}
	}

	// First sample
	oneMinusAlpha := 1.0 - float64(l.alpha)
	for ch := 0; ch < channels; ch++ {
		l.output[0][ch] = l.alpha*frame[0][ch] + float32(oneMinusAlpha)*l.filterStates[ch]
	}

	// Remaining samples
	for i := 1; i < samples; i++ {
		for ch := 0; ch < channels; ch++ {
			l.output[i][ch] = l.alpha*frame[i][ch] + float32(oneMinusAlpha)*l.output[i-1][ch]
		}
	}

	// Update states
	for ch := 0; ch < channels; ch++ {
		l.filterStates[ch] = l.output[samples-1][ch]
	}

	return l.output
}

// BandPass implements a band-pass filter (cascade of HighPass + LowPass).
type BandPass struct {
	lowCut   float64
	highCut  float64
	highPass *HighPass
	lowPass  *LowPass
}

// NewBandPass creates a new BandPass filter with the given low and high cutoff frequencies.
func NewBandPass(lowCut, highCut float64) *BandPass {
	if lowCut >= highCut {
		panic("Low-cut frequency must be less than high-cut frequency")
	}
	return &BandPass{
		lowCut:   lowCut,
		highCut:  highCut,
		highPass: NewHighPass(lowCut),
		lowPass:  NewLowPass(highCut),
	}
}

func (b *BandPass) HandleFrame(frame [][]float32, samplerate int) [][]float32 {
	if len(frame) == 0 {
		return frame
	}
	highPassed := b.highPass.HandleFrame(frame, samplerate)
	bandPassed := b.lowPass.HandleFrame(highPassed, samplerate)
	return bandPassed
}

// AGC implements Automatic Gain Control.
type AGC struct {
	targetLevel    float64
	maxGainDB      float64
	attackTime     float64
	releaseTime    float64
	holdTime       float64
	triggerLevel   float64
	samplerate     int
	channels       int
	currentGainLin []float32
	holdCounter    int
	blockTargetS   float64
	attackCoeff    float64
	releaseCoeff   float64
	holdSamples    int
	output         [][]float32
	outputFlat     []float32
}

// NewAGC creates a new AGC with the given parameters.
// targetLevel: target level in dBFS
// maxGainDB: maximum gain in dB
// attackTime: attack time in seconds
// releaseTime: release time in seconds
// holdTime: hold time in seconds
func NewAGC(targetLevel, maxGainDB, attackTime, releaseTime, holdTime float64) *AGC {
	return &AGC{
		targetLevel:  targetLevel,
		maxGainDB:    maxGainDB,
		attackTime:   attackTime,
		releaseTime:  releaseTime,
		holdTime:     holdTime,
		triggerLevel:  0.003,
		blockTargetS:  0.01,
	}
}

func (a *AGC) HandleFrame(frame [][]float32, samplerate int) [][]float32 {
	if len(frame) == 0 {
		return frame
	}

	samples := len(frame)
	if samples == 0 {
		return frame
	}
	channels := len(frame[0])

	// Recalculate coefficients if samplerate changed
	if samplerate != a.samplerate {
		a.samplerate = samplerate
		a.calculateCoefficients()
	}

	// Initialize gains if needed
	if a.currentGainLin == nil || a.channels != channels {
		a.channels = channels
		a.currentGainLin = make([]float32, channels)
		for i := range a.currentGainLin {
			a.currentGainLin[i] = 1.0
		}
		a.holdCounter = 0
	}

	// Reuse output buffer, copy input
	if cap(a.outputFlat) < samples*channels {
		a.outputFlat = make([]float32, samples*channels)
		a.output = make([][]float32, samples)
		for i := range a.output {
			a.output[i] = a.outputFlat[i*channels : (i+1)*channels]
		}
	} else {
		a.output = a.output[:samples]
		for i := 0; i < samples; i++ {
			a.output[i] = a.outputFlat[i*channels : (i+1)*channels]
		}
	}
	for i := 0; i < samples; i++ {
		for ch := 0; ch < channels; ch++ {
			a.output[i][ch] = frame[i][ch]
		}
	}

	// Process in blocks (matching C and Python implementations)
	numBlocks := max(1, int(float64(samples)/float64(samplerate)/a.blockTargetS))
	blockSize := samples / numBlocks
	if blockSize < 1 {
		blockSize = 1
	}

	for block := 0; block < numBlocks; block++ {
		blockStart := block * blockSize
		blockEnd := (block + 1) * blockSize
		if block == numBlocks-1 {
			blockEnd = samples
		}
		if blockEnd > samples {
			blockEnd = samples
		}

		blockSamples := blockEnd - blockStart
		if blockSamples <= 0 {
			continue
		}

		for ch := 0; ch < channels; ch++ {
			// Calculate RMS for this block (from output, which starts as input copy)
			sumSquares := 0.0
			for j := blockStart; j < blockEnd; j++ {
				val := a.output[j][ch]
				sumSquares += float64(val * val)
			}
			rms := math.Sqrt(sumSquares / float64(blockSamples))

			var targetGain float32
			if rms > 1e-9 && rms > float64(a.triggerLevel) {
				targetGain = float32(math.Min(a.maxGainLin(), a.targetLin()/math.Max(rms, 1e-9)))
			} else {
				targetGain = a.currentGainLin[ch]
			}

			// Smooth gain
			if float64(targetGain) < float64(a.currentGainLin[ch]) {
				a.currentGainLin[ch] = float32(a.attackCoeff*float64(targetGain) + (1-a.attackCoeff)*float64(a.currentGainLin[ch]))
				a.holdCounter = a.holdSamples
			} else {
				if a.holdCounter > 0 {
					a.holdCounter -= blockSamples
				} else {
					a.currentGainLin[ch] = float32(a.releaseCoeff*float64(targetGain) + (1-a.releaseCoeff)*float64(a.currentGainLin[ch]))
				}
			}

			// Apply gain to block
			for j := blockStart; j < blockEnd; j++ {
				a.output[j][ch] *= a.currentGainLin[ch]
			}
		}
	}

	// Peak limiting
	peakLimit := 0.75
	for ch := 0; ch < channels; ch++ {
		peak := 0.0
		for i := 0; i < samples; i++ {
			absVal := math.Abs(float64(a.output[i][ch]))
			if absVal > peak {
				peak = absVal
			}
		}
		if peak > peakLimit {
			scale := peakLimit / peak
			for i := 0; i < samples; i++ {
				a.output[i][ch] *= float32(scale)
			}
		}
	}

	return a.output
}

func (a *AGC) calculateCoefficients() {
	if a.samplerate > 0 {
		a.attackCoeff = 1.0 - math.Exp(-1.0/(a.attackTime*float64(a.samplerate)))
		a.releaseCoeff = 1.0 - math.Exp(-1.0/(a.releaseTime*float64(a.samplerate)))
		a.holdSamples = int(a.holdTime * float64(a.samplerate))
	} else {
		a.attackCoeff = 0.1
		a.releaseCoeff = 0.01
		a.holdSamples = 1000
	}
}

func (a *AGC) targetLin() float64 {
	return math.Pow(10, a.targetLevel/10.0)
}

func (a *AGC) maxGainLin() float64 {
	return math.Pow(10, a.maxGainDB/10.0)
}
