// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package processing provides audio analysis and conversion utilities
// for the LXST audio processing library. It includes functions for
// measuring audio levels (RMS, peak, energy), detecting silence and
// voice activity, counting clips, measuring zero-crossing rate, and
// converting between channel counts and sample rates.
package processing

import (
	"math"
)

// RMS computes the root mean square of all samples in the frame,
// across all channels. Returns 0 for empty or nil frames.
func RMS(frame [][]float32) float64 {
	if len(frame) == 0 {
		return 0.0
	}

	var sum float64
	count := 0
	for _, s := range frame {
		for _, v := range s {
			f := float64(v)
			sum += f * f
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return math.Sqrt(sum / float64(count))
}

// Peak returns the maximum absolute sample value across all channels.
// Returns 0 for empty or nil frames.
func Peak(frame [][]float32) float64 {
	if len(frame) == 0 {
		return 0.0
	}

	var peak float64
	for _, s := range frame {
		for _, v := range s {
			abs := math.Abs(float64(v))
			if abs > peak {
				peak = abs
			}
		}
	}

	return peak
}

// IsSilence returns true if the peak absolute value in the frame is
// below the given threshold. Nil or empty frames are considered silent.
func IsSilence(frame [][]float32, threshold float64) bool {
	if len(frame) == 0 {
		return true
	}

	return Peak(frame) < threshold
}

// RMSdB computes the RMS level in decibels (dBFS) of the frame.
// Returns -Inf for silence.
func RMSdB(frame [][]float32) float64 {
	rms := RMS(frame)
	if rms <= 0 {
		return math.Inf(-1)
	}
	return 20.0 * math.Log10(rms)
}

// VAD performs simple voice activity detection. It returns true when
// the frame's RMS level exceeds the given threshold, indicating the
// likely presence of voice. This is a basic energy-based VAD; more
// sophisticated detectors may use spectral features.
func VAD(frame [][]float32, threshold float64) bool {
	return RMS(frame) > threshold
}

// ConvertChannels converts the channel count of a frame. When
// increasing channels, the last channel is duplicated. When decreasing,
// channels are averaged. When truncating, extra channels are dropped.
// Same-count conversion returns the frame unchanged.
func ConvertChannels(frame [][]float32, targetChannels int) [][]float32 {
	if len(frame) == 0 {
		return frame
	}

	currentChannels := len(frame[0])
	if currentChannels == targetChannels {
		return frame
	}

	result := make([][]float32, len(frame))
	for i, s := range frame {
		result[i] = make([]float32, targetChannels)

		if targetChannels < currentChannels {
			// Average all channels down to targetChannels
			// For mono: average all channels
			if targetChannels == 1 {
				var sum float32
				for _, v := range s {
					sum += v
				}
				result[i][0] = sum / float32(currentChannels)
			} else {
				// Truncate extra channels
				copy(result[i], s[:targetChannels])
			}
		} else {
			// Copy existing channels
			copy(result[i], s)
			// Duplicate last channel for remaining
			lastVal := s[len(s)-1]
			for ch := currentChannels; ch < targetChannels; ch++ {
				result[i][ch] = lastVal
			}
		}
	}

	return result
}

// Resample changes the sample rate of a frame using linear interpolation.
// The fromRate and toRate parameters specify the original and target
// sample rates respectively. For same-rate resampling, the original
// frame is returned unchanged.
func Resample(frame [][]float32, fromRate, toRate int) [][]float32 {
	if len(frame) == 0 || fromRate == toRate {
		return frame
	}

	ratio := float64(toRate) / float64(fromRate)
	newLen := int(math.Round(float64(len(frame)) * ratio))
	if newLen == 0 {
		return nil
	}

	channels := 0
	if len(frame) > 0 {
		channels = len(frame[0])
	}

	result := make([][]float32, newLen)
	for i := 0; i < newLen; i++ {
		result[i] = make([]float32, channels)

		srcPos := float64(i) / ratio
		srcIdx := int(srcPos)
		frac := srcPos - float64(srcIdx)

		for ch := 0; ch < channels; ch++ {
			if srcIdx >= len(frame)-1 {
				result[i][ch] = frame[len(frame)-1][ch]
			} else {
				v0 := float64(frame[srcIdx][ch])
				v1 := float64(frame[srcIdx+1][ch])
				result[i][ch] = float32(v0 + frac*(v1-v0))
			}
		}
	}

	return result
}

// Normalize scales the frame so that the peak absolute value is 1.0.
// Silent frames are returned unchanged.
func Normalize(frame [][]float32) [][]float32 {
	if len(frame) == 0 {
		return frame
	}

	peak := Peak(frame)
	if peak < 1e-9 {
		return frame
	}

	scale := float32(1.0 / peak)
	result := make([][]float32, len(frame))
	for i, s := range frame {
		result[i] = make([]float32, len(s))
		for j, v := range s {
			result[i][j] = v * scale
		}
	}

	return result
}

// ClipCount returns the number of samples at or exceeding full scale
// (absolute value >= 1.0), indicating potential clipping.
func ClipCount(frame [][]float32) int {
	if len(frame) == 0 {
		return 0
	}

	count := 0
	for _, s := range frame {
		for _, v := range s {
			if math.Abs(float64(v)) >= 1.0 {
				count++
			}
		}
	}

	return count
}

// Energy returns the sum of squared sample values across all channels.
func Energy(frame [][]float32) float64 {
	if len(frame) == 0 {
		return 0.0
	}

	var sum float64
	for _, s := range frame {
		for _, v := range s {
			f := float64(v)
			sum += f * f
		}
	}

	return sum
}

// ZeroCrossingRate computes the zero-crossing rate of the first channel.
// This is the fraction of adjacent sample pairs that cross zero,
// useful for distinguishing voiced speech (low ZCR) from unvoiced
// noise (high ZCR).
func ZeroCrossingRate(frame [][]float32) float64 {
	if len(frame) <= 1 {
		return 0.0
	}

	crossings := 0
	for i := 1; i < len(frame); i++ {
		v0 := float64(frame[i-1][0])
		v1 := float64(frame[i][0])
		if (v0 >= 0 && v1 < 0) || (v0 < 0 && v1 >= 0) {
			crossings++
		}
	}

	return float64(crossings) / float64(len(frame)-1)
}
