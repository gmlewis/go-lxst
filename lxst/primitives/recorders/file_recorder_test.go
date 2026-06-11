// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package recorders

import (
	"testing"

	opusPkg "github.com/gmlewis/go-lxst/lxst/codecs/opus"
)

func TestFileRecorder_New(t *testing.T) {
	t.Parallel()

	fr := NewFileRecorder("/tmp/test_recording.opus", "", opusPkg.PROFILE_VOICE_LOW, 0.0, 0.125, 0.075)
	if fr == nil {
		t.Fatal("NewFileRecorder returned nil")
	}
	if fr.Running() {
		t.Error("FileRecorder should not be running initially")
	}
}

func TestFileRecorder_FilePath(t *testing.T) {
	t.Parallel()

	fr := NewFileRecorder("/tmp/test.opus", "", opusPkg.PROFILE_VOICE_LOW, 0.0, 0.125, 0.075)
	if fr.FilePath() != "/tmp/test.opus" {
		t.Errorf("Expected /tmp/test.opus, got %s", fr.FilePath())
	}
}

func TestFileRecorder_Recording(t *testing.T) {
	t.Parallel()

	fr := NewFileRecorder("/tmp/test.opus", "", opusPkg.PROFILE_VOICE_LOW, 0.0, 0.125, 0.075)
	if fr.Recording() {
		t.Error("FileRecorder should not be recording initially")
	}
}

func TestFileRecorder_FramesWaiting(t *testing.T) {
	t.Parallel()

	fr := NewFileRecorder("/tmp/test.opus", "", opusPkg.PROFILE_VOICE_LOW, 0.0, 0.125, 0.075)
	if fr.FramesWaiting() != 0 {
		t.Errorf("Expected 0 frames waiting, got %d", fr.FramesWaiting())
	}
}

func TestFileRecorder_SetSource(t *testing.T) {
	t.Parallel()

	fr := NewFileRecorder("/tmp/test.opus", "", opusPkg.PROFILE_VOICE_LOW, 0.0, 0.125, 0.075)
	fr.SetSource("null-device")
}

func TestFileRecorder_StopNotRunning(t *testing.T) {
	t.Parallel()

	fr := NewFileRecorder("/tmp/test.opus", "", opusPkg.PROFILE_VOICE_LOW, 0.0, 0.125, 0.075)
	err := fr.Stop()
	if err != nil {
		t.Errorf("Stop on non-running recorder should succeed: %v", err)
	}
}
