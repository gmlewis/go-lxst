// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

//go:build integration

package filters

import (
	"math"
	"testing"
)

func makeParityFrame(samples, channels int) [][]float32 {
	frame := make([][]float32, samples)
	for i := range frame {
		frame[i] = make([]float32, channels)
		frame[i][0] = float32(math.Sin(float64(i)*0.1)) * 0.5
		if channels > 1 {
			frame[i][1] = float32(math.Cos(float64(i)*0.1)) * 0.3
		}
	}
	return frame
}

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

func TestParity_HighPass_CNative(t *testing.T) {
	hp := NewHighPass(300)
	frame := makeParityFrame(480, 2)
	out := hp.HandleFrame(frame, 48000)

	expectedFirst := [][2]float64{
		{0.0, 0.28866419196128845},
		{0.0480305515229702, 0.2763145864009857},
		{0.09376631677150726, 0.261561781167984},
		{0.13681890070438385, 0.24453970789909363},
		{0.17682409286499023, 0.22540542483329773},
	}

	for i, exp := range expectedFirst {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out[i][ch]), exp[ch], 0.001) {
				t.Errorf("HighPass frame1[%d][%d]: got %v, want %v", i, ch, out[i][ch], exp[ch])
			}
		}
	}

	expectedLast := [][2]float64{
		{-0.3097585439682007, -0.2021367847919464},
		{-0.341844379901886, -0.1825723946094513},
		{-0.3705146312713623, -0.16118378937244415},
		{-0.3954828083515167, -0.13818471133708954},
		{-0.41649946570396423, -0.11380492150783539},
	}

	for i, exp := range expectedLast {
		idx := 475 + i
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out[idx][ch]), exp[ch], 0.001) {
				t.Errorf("HighPass frame1 last[%d][%d]: got %v, want %v", idx, ch, out[idx][ch], exp[ch])
			}
		}
	}
}

func TestParity_HighPass_SecondFrame(t *testing.T) {
	hp := NewHighPass(300)
	frame := makeParityFrame(480, 2)
	_ = hp.HandleFrame(frame, 48000)
	out2 := hp.HandleFrame(frame, 48000)

	expectedFirst := [][2]float64{
		{-0.06374193727970123, 0.38516291975975037},
		{-0.01330282911658287, 0.3691670000553131},
		{0.03475048020482063, 0.3509056866168976},
		{0.08003303408622742, 0.33050766587257385},
		{0.12218394130468369, 0.30812498927116394},
	}

	for i, exp := range expectedFirst {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out2[i][ch]), exp[ch], 0.001) {
				t.Errorf("HighPass frame2[%d][%d]: got %v, want %v", i, ch, out2[i][ch], exp[ch])
			}
		}
	}
}

func TestParity_LowPass_CNative(t *testing.T) {
	lp := NewLowPass(3000)
	frame := makeParityFrame(480, 2)
	out := lp.HandleFrame(frame, 48000)

	expectedFirst := [][2]float64{
		{0.0, 0.08459094166755676},
		{0.014075003564357758, 0.14490719139575958},
		{0.03811565041542053, 0.18695248663425446},
		{0.06903207302093506, 0.2150503396987915},
		{0.10446921736001968, 0.23232606053352356},
	}

	for i, exp := range expectedFirst {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out[i][ch]), exp[ch], 0.001) {
				t.Errorf("LowPass frame1[%d][%d]: got %v, want %v", i, ch, out[i][ch], exp[ch])
			}
		}
	}
}

func TestParity_LowPass_SecondFrame(t *testing.T) {
	lp := NewLowPass(3000)
	frame := makeParityFrame(480, 2)
	_ = lp.HandleFrame(frame, 48000)
	out2 := lp.HandleFrame(frame, 48000)

	expectedFirst := [][2]float64{
		{-0.17386919260025024, -0.09339115023612976},
		{-0.11076833307743073, 0.017110664397478104},
		{-0.05152563378214836, 0.09519071877002716},
		{0.00466692540794611, 0.14916262030601501},
		{0.05825309455394745, 0.1850166767835617},
	}

	for i, exp := range expectedFirst {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out2[i][ch]), exp[ch], 0.001) {
				t.Errorf("LowPass frame2[%d][%d]: got %v, want %v", i, ch, out2[i][ch], exp[ch])
			}
		}
	}
}

func TestParity_BandPass_CNative(t *testing.T) {
	bp := NewBandPass(300, 3000)
	frame := makeParityFrame(480, 2)
	out := bp.HandleFrame(frame, 48000)

	expectedFirst := [][2]float64{
		{0.0, 0.08139458298683167},
		{0.013543164357542992, 0.13635613024234772},
		{0.03616366907954216, 0.17166033387184143},
		{0.0645454004406929, 0.1922101229429245},
		{0.09620460122823715, 0.20157019793987274},
	}

	for i, exp := range expectedFirst {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out[i][ch]), exp[ch], 0.001) {
				t.Errorf("BandPass frame1[%d][%d]: got %v, want %v", i, ch, out[i][ch], exp[ch])
			}
		}
	}
}

func TestParity_BandPass_SecondFrame(t *testing.T) {
	bp := NewBandPass(300, 3000)
	frame := makeParityFrame(480, 2)
	_ = bp.HandleFrame(frame, 48000)
	out2 := bp.HandleFrame(frame, 48000)

	expectedFirst := [][2]float64{
		{-0.2640123665332794, -0.009142501279711723},
		{-0.19331985712051392, 0.0975293442606926},
		{-0.12901091575622559, 0.16897381842136383},
		{-0.07006683945655823, 0.21452148258686066},
		{-0.01585792936384678, 0.24091483652591705},
	}

	for i, exp := range expectedFirst {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out2[i][ch]), exp[ch], 0.002) {
				t.Errorf("BandPass frame2[%d][%d]: got %v, want %v", i, ch, out2[i][ch], exp[ch])
			}
		}
	}
}

func TestParity_AGC_SineInput(t *testing.T) {
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)
	frame := makeParityFrame(480, 2)
	out := agc.HandleFrame(frame, 48000)

	expectedFirst := [][2]float64{
		{0.0, 0.26026681065559387},
		{0.04221417009830475, 0.2589665651321411},
		{0.08400655537843704, 0.2550787925720215},
		{0.12495957314968109, 0.24864238500595093},
		{0.16466403007507324, 0.2397216260433197},
	}

	for i, exp := range expectedFirst {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out[i][ch]), exp[ch], 0.001) {
				t.Errorf("AGC sine frame1[%d][%d]: got %v, want %v", i, ch, out[i][ch], exp[ch])
			}
		}
	}
}

func TestParity_AGC_SecondFrame(t *testing.T) {
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)
	frame := makeParityFrame(480, 2)
	_ = agc.HandleFrame(frame, 48000)
	out2 := agc.HandleFrame(frame, 48000)

	expectedFirst := [][2]float64{
		{0.0, 0.22800599038600922},
		{0.03596020117402077, 0.2268669158220291},
		{0.07156109809875488, 0.22346104681491852},
		{0.10644698888063431, 0.21782244741916656},
		{0.14026927947998047, 0.2100074291229248},
	}

	for i, exp := range expectedFirst {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out2[i][ch]), exp[ch], 0.001) {
				t.Errorf("AGC sine frame2[%d][%d]: got %v, want %v", i, ch, out2[i][ch], exp[ch])
			}
		}
	}
}

func TestParity_AGC_ConstantLowInput(t *testing.T) {
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)
	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = []float32{0.1, 0.1}
	}
	out := agc.HandleFrame(frame, 48000)

	expected := float64(0.09305963665246964)
	for i := 0; i < 5; i++ {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out[i][ch]), expected, 0.001) {
				t.Errorf("AGC low[%d][%d]: got %v, want %v", i, ch, out[i][ch], expected)
			}
		}
	}
}

func TestParity_AGC_PeakLimiting(t *testing.T) {
	agc := NewAGC(-12.0, 12.0, 0.0001, 0.002, 0.001)
	frame := make([][]float32, 480)
	for i := range frame {
		frame[i] = []float32{2.0, 2.0}
	}
	out := agc.HandleFrame(frame, 48000)

	for i := 0; i < len(out); i++ {
		for ch := 0; ch < 2; ch++ {
			if math.Abs(float64(out[i][ch])) > 0.75+0.001 {
				t.Errorf("AGC peak limit failed at [%d][%d]: got %v, max 0.75", i, ch, out[i][ch])
			}
		}
	}

	for i := 0; i < 5; i++ {
		for ch := 0; ch < 2; ch++ {
			if !approxEqual(float64(out[i][ch]), 0.75, 0.001) {
				t.Errorf("AGC high[%d][%d]: got %v, want ~0.75", i, ch, out[i][ch])
			}
		}
	}
}
