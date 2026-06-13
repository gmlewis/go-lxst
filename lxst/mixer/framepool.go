// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package mixer

import "sync"

// FramePool provides a pool of reusable audio frame buffers.
// It reduces memory allocations by recycling frames that are
// no longer in use. Each frame is a 2D slice of float32 samples
// organized as [samples][channels].
type FramePool struct {
	pool sync.Pool
	rows int
	cols int
}

// NewFramePool creates a new FramePool that produces frames
// with the specified number of samples (rows) and channels (cols).
func NewFramePool(rows, cols int) *FramePool {
	return &FramePool{
		rows: rows,
		cols: cols,
		pool: sync.Pool{
			New: func() any {
				frame := make([][]float32, rows)
				for i := range frame {
					frame[i] = make([]float32, cols)
				}
				return frame
			},
		},
	}
}

// Get retrieves a frame from the pool. The returned frame
// is guaranteed to be zeroed and ready for use.
func (fp *FramePool) Get() [][]float32 {
	frame := fp.pool.Get().([][]float32)
	for i := range frame {
		for j := range frame[i] {
			frame[i][j] = 0
		}
	}
	return frame
}

// Put returns a frame to the pool for reuse.
// The frame should not be used after calling Put.
func (fp *FramePool) Put(frame [][]float32) {
	if cap(frame) >= fp.rows {
		fp.pool.Put(frame[:fp.rows])
	}
}
