// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package codec2

import "testing"

func TestCodec2_Modes(t *testing.T) {
	t.Parallel()
	validModes := []int{CODEC2_700C, CODEC2_1200, CODEC2_1300, CODEC2_1400, CODEC2_1600, CODEC2_2400, CODEC2_3200}

	for _, mode := range validModes {
		t.Run(fmtMode(mode), func(t *testing.T) {
			t.Parallel()
			c, err := NewCodec2(mode)
			if err != nil {
				t.Fatalf("NewCodec2 failed for mode %d: %v", mode, err)
			}
			if c.mode != mode {
				t.Errorf("Mode: got %d, want %d", c.mode, mode)
			}
			if c.frameQuantumMs != FRAME_QUANTA_MS {
				t.Errorf("Frame quanta: got %f, want %f", c.frameQuantumMs, FRAME_QUANTA_MS)
			}
			if c.channels != 1 {
				t.Errorf("Channels: got %d, want 1", c.channels)
			}
			if c.PreferredSampleRate() != INPUT_RATE {
				t.Errorf("PreferredSampleRate: got %d, want %d", c.PreferredSampleRate(), INPUT_RATE)
			}
		})
	}
}

func fmtMode(mode int) string {
	switch mode {
	case CODEC2_700C:
		return "700C"
	case CODEC2_1200:
		return "1200"
	case CODEC2_1300:
		return "1300"
	case CODEC2_1400:
		return "1400"
	case CODEC2_1600:
		return "1600"
	case CODEC2_2400:
		return "2400"
	case CODEC2_3200:
		return "3200"
	default:
		return "unknown"
	}
}

func TestCodec2_InvalidMode(t *testing.T) {
	t.Parallel()
	_, err := NewCodec2(999)
	if err == nil {
		t.Error("Expected error for invalid mode")
	}
}

func TestCodec2_HeaderEncoding(t *testing.T) {
	t.Parallel()
	// Test that mode headers match Python
	testCases := []struct {
		mode  int
		header byte
	}{
		{CODEC2_700C, 0x00},
		{CODEC2_1200, 0x01},
		{CODEC2_1300, 0x02},
		{CODEC2_1400, 0x03},
		{CODEC2_1600, 0x04},
		{CODEC2_2400, 0x05},
		{CODEC2_3200, 0x06},
	}

	for _, tc := range testCases {
		t.Run(fmtMode(tc.mode), func(t *testing.T) {
			t.Parallel()
			c, _ := NewCodec2(tc.mode)
			if c.modeHeader != tc.header {
				t.Errorf("Mode header: got 0x%02x, want 0x%02x", c.modeHeader, tc.header)
			}
		})
	}
}

func TestCodec2_SetMode(t *testing.T) {
	t.Parallel()
	c, _ := NewCodec2(CODEC2_2400)
	
	err := c.SetMode(CODEC2_3200)
	if err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}
	if c.mode != CODEC2_3200 {
		t.Errorf("Mode not changed: got %d, want %d", c.mode, CODEC2_3200)
	}
	if c.modeHeader != 0x06 {
		t.Errorf("Mode header not updated: got 0x%02x, want 0x06", c.modeHeader)
	}
}

func TestCodec2_Encode_ReturnsModeHeader(t *testing.T) {
	t.Parallel()
	c, _ := NewCodec2(CODEC2_2400)
	
	frame := [][]float32{{0.1}, {0.2}, {0.3}}
	encoded := c.Encode(frame)
	
	// Should return just the mode header (stub implementation)
	if len(encoded) != 1 {
		t.Errorf("Expected 1 byte (mode header), got %d", len(encoded))
	}
	if encoded[0] != 0x05 { // 2400 mode header
		t.Errorf("Header byte: got 0x%02x, want 0x05", encoded[0])
	}
}

func TestCodec2_Decode_HeaderSwitch(t *testing.T) {
	t.Parallel()
	c, _ := NewCodec2(CODEC2_2400)
	
	// Decode with different mode header
	data := []byte{0x06, 0x00, 0x01} // 3200 mode header + dummy data
	_ = c.Decode(data)
	
	// Should have switched mode
	if c.mode != CODEC2_3200 {
		t.Errorf("Mode not switched: got %d, want %d", c.mode, CODEC2_3200)
	}
}

func TestCodec2_FrameQuantum(t *testing.T) {
	t.Parallel()
	c, _ := NewCodec2(CODEC2_2400)
	
	if c.FrameQuantumMs() != FRAME_QUANTA_MS {
		t.Errorf("FrameQuantumMs: got %f, want %f", c.FrameQuantumMs(), FRAME_QUANTA_MS)
	}
}

func TestCodec2_ValidFrameMs(t *testing.T) {
	t.Parallel()
	c, _ := NewCodec2(CODEC2_2400)
	
	valid := c.ValidFrameMs()
	if len(valid) != 1 || valid[0] != FRAME_QUANTA_MS {
		t.Errorf("ValidFrameMs: got %v, want [%f]", valid, FRAME_QUANTA_MS)
	}
}