// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

// Package call provides telephony call endpoint functionality.
package call

import (
	"errors"
	"sync"
)

var (
	ErrNoActiveCall    = errors.New("no active call")
	ErrCallbackNotFunc = errors.New("callback is not callable")
)

type IncomingCallCallback func(link any)

type CallEndpoint struct {
	mu                   sync.Mutex
	identity             any
	destination          any
	activeCall           any
	autoAnswer           bool
	receivePipeline      any
	transmitPipeline     any
	incomingCallCallback IncomingCallCallback
}

func NewCallEndpoint(identity any) *CallEndpoint {
	ce := &CallEndpoint{
		identity:   identity,
		autoAnswer: true,
	}
	return ce
}

func (ce *CallEndpoint) SetIncomingCallCallback(callback IncomingCallCallback) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.incomingCallCallback = callback
	return nil
}

func (ce *CallEndpoint) AutoAnswer() bool {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ce.autoAnswer
}

func (ce *CallEndpoint) SetAutoAnswer(auto bool) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.autoAnswer = auto
}

func (ce *CallEndpoint) HasActiveCall() bool {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ce.activeCall != nil
}

func (ce *CallEndpoint) Terminate() error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if ce.activeCall == nil {
		return ErrNoActiveCall
	}

	ce.activeCall = nil
	ce.receivePipeline = nil
	ce.transmitPipeline = nil

	return nil
}
