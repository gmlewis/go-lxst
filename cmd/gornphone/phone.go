// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"time"
)

// Phone states matching the Python ReticulumTelephone constants.
const (
	StateAvailable  byte = 0x00
	StateConnecting byte = 0x01
	StateRinging    byte = 0x02
	StateInCall     byte = 0x03
)

const (
	ringTime    = 30
	waitTime    = 60
	pathTime    = 10
_hwSleepTimeout = 15
)

// Phone represents a Reticulum telephone instance with call state management.
type Phone struct {
	state                   byte
	direction               string
	lastDialledIdentityHash string
	callerHash              string
	callerName              string
	callerAlias             string
	firstRun                bool
	shouldRun               bool
	config                  *PhoneConfig
	lastInput               string
	started                 time.Time
	endpoint                *TelephoneEndpoint
}

// NewPhone creates a new Phone with the given configuration.
func NewPhone(cfg *PhoneConfig) *Phone {
	return &Phone{
		state:    StateAvailable,
		config:   cfg,
		firstRun: true,
	}
}

// SetEndpoint attaches a TelephoneEndpoint for RNS integration.
func (p *Phone) SetEndpoint(ep *TelephoneEndpoint) {
	p.endpoint = ep
}

// Endpoint returns the attached TelephoneEndpoint, if any.
func (p *Phone) Endpoint() *TelephoneEndpoint {
	return p.endpoint
}

// State returns the current phone state.
func (p *Phone) State() byte {
	return p.state
}

// IsAvailable reports whether the phone is available for calls.
func (p *Phone) IsAvailable() bool {
	return p.state == StateAvailable
}

// IsInCall reports whether the phone is in an active call.
func (p *Phone) IsInCall() bool {
	return p.state == StateInCall
}

// IsRinging reports whether the phone is ringing (incoming call).
func (p *Phone) IsRinging() bool {
	return p.state == StateRinging
}

// CallIsConnecting reports whether a call is being connected.
func (p *Phone) CallIsConnecting() bool {
	return p.state == StateConnecting
}

// CallDuration returns how long the current call has been active.
func (p *Phone) CallDuration() time.Duration {
	return time.Since(p.started)
}

// SetState sets the phone state.
func (p *Phone) SetState(state byte) {
	p.state = state
}

// SetCaller sets the identity hash, name, and alias of the current caller.
func (p *Phone) SetCaller(hash, name, alias string) {
	p.callerHash = hash
	p.callerName = name
	p.callerAlias = alias
}

// CallerHash returns the identity hash of the current caller.
func (p *Phone) CallerHash() string {
	return p.callerHash
}

// CallerName returns the name of the current caller from the phonebook.
func (p *Phone) CallerName() string {
	return p.callerName
}

// CallerAlias returns the numerical alias of the current caller.
func (p *Phone) CallerAlias() string {
	return p.callerAlias
}

// LastDialledHash returns the last dialled identity hash.
func (p *Phone) LastDialledHash() string {
	return p.lastDialledIdentityHash
}

// Redial redials the last called identity.
func (p *Phone) Redial() {
	if p.lastDialledIdentityHash != "" {
		p.Dial(p.lastDialledIdentityHash)
	}
}

// Dial initiates a call to the given identity hash.
func (p *Phone) Dial(hash string) {
	if !p.IsAvailable() {
		return
	}
	p.lastDialledIdentityHash = hash
	p.direction = "to"

	name, alias, ok := p.config.LookupHash(hash)
	if ok {
		p.callerHash = hash
		p.callerName = name
		p.callerAlias = alias
	} else {
		p.callerHash = hash
		p.callerName = ""
		p.callerAlias = ""
	}

	p.state = StateConnecting
	fmt.Printf("Calling %s...\n", prettyHex(hash))
}

// Ringing handles an incoming call notification.
func (p *Phone) Ringing(hash string) {
	p.state = StateRinging
	p.direction = "from"
	p.callerHash = hash

	name, alias, ok := p.config.LookupHash(hash)
	if ok {
		p.callerName = name
		p.callerAlias = alias
	} else {
		p.callerName = ""
		p.callerAlias = ""
	}

	fmt.Printf("\n\nIncoming call from %s\n", prettyHex(hash))
	if p.callerName != "" {
		fmt.Printf("  %s", p.callerName)
		if p.callerAlias != "" {
			fmt.Printf(" (%s)", p.callerAlias)
		}
		fmt.Println()
	}
	fmt.Println("Hit enter to answer, r to reject")
}

// Answer accepts an incoming call.
func (p *Phone) Answer() bool {
	if !p.IsRinging() {
		return false
	}
	fmt.Printf("Answering call from %s\n", prettyHex(p.callerHash))
	p.state = StateConnecting
	return true
}

// Hangup terminates the current call.
func (p *Phone) Hangup() {
	if p.state == StateAvailable {
		return
	}

	switch {
	case p.IsInCall():
		fmt.Printf("Call with %s ended\n\n", prettyHex(p.callerHash))
	case p.IsRinging():
		fmt.Printf("Call from %s was not answered\n\n", prettyHex(p.callerHash))
	case p.CallIsConnecting():
		fmt.Printf("Call to %s could not be connected\n\n", prettyHex(p.callerHash))
	}

	p.direction = ""
	p.state = StateAvailable
}

// Reject rejects an incoming call.
func (p *Phone) Reject() {
	if !p.IsRinging() {
		return
	}
	fmt.Printf("Rejecting call from %s\n", prettyHex(p.callerHash))
	p.Hangup()
}

// CallEstablished marks the call as fully established.
func (p *Phone) CallEstablished() {
	if p.CallIsConnecting() || p.IsRinging() {
		p.state = StateInCall
		p.started = time.Now()
		fmt.Printf("Call established with %s\n", prettyHex(p.callerHash))
	}
}

// PrintIdentity prints the identity hash of this telephone.
func (p *Phone) PrintIdentity(hash string) {
	fmt.Printf("Identity hash of this telephone: %s\n\n", prettyHex(hash))
}

// PrintDestination prints the destination hash of this telephone.
func (p *Phone) PrintDestination(hash string) {
	fmt.Printf("Destination hash of this telephone: %s\n\n", prettyHex(hash))
}

// PrintPhonebook displays the phonebook entries.
func (p *Phone) PrintPhonebook() {
	if len(p.config.Phonebook) == 0 {
		fmt.Println("\nNo entries in phonebook")
		return
	}

	fmt.Println()
	fmt.Println("Phonebook:")

	n := 0
	for name, entry := range p.config.Phonebook {
		n++
		alias := fmt.Sprintf("%d", n)
		if entry.Alias != "" {
			alias = entry.Alias
		}
		fmt.Printf("  %s %s : <%s>\n", alias, name, entry.Hash)
	}
	fmt.Println()
}

// ProcessInput processes a line of user input and returns true if the phone should continue running.
func (p *Phone) ProcessInput(input string) bool {
	p.lastInput = input

	if p.IsAvailable() {
		return p.processAvailableInput(input)
	} else if p.IsRinging() {
		return p.processRingingInput(input)
	} else if p.IsInCall() || p.CallIsConnecting() {
		return p.processInCallInput(input)
	}

	return true
}

func (p *Phone) processAvailableInput(input string) bool {
	input = trimSpace(input)

	switch input {
	case "q", "quit", "exit":
		return false
	case "h", "help", "?":
		p.printHelp()
	case "p", "phonebook":
		p.PrintPhonebook()
	case "r", "redial":
		p.Redial()
	case "i", "identity":
		if p.endpoint != nil {
			p.PrintIdentity(p.endpoint.IdentityHash())
		} else {
			fmt.Println("(identity hash will be shown when connected)")
		}
	case "d", "desthash":
		if p.endpoint != nil {
			p.PrintDestination(p.endpoint.DestinationHash())
		} else {
			fmt.Println("(destination hash will be shown when connected)")
		}
	case "a", "announce":
		if p.endpoint != nil {
			if err := p.endpoint.Announce(); err != nil {
				fmt.Printf("Announce failed: %v\n", err)
			} else {
				fmt.Println("Announce sent")
			}
		} else {
			fmt.Println("Announce sent")
		}
	default:
		if len(input) == 32 {
			p.Dial(input)
		} else if input != "" {
			alias, hash, ok := p.config.LookupAlias(input)
			if ok {
				p.Dial(hash)
				_ = alias
			} else {
				fmt.Printf("Unknown command or invalid hash: %s\n", input)
			}
		}
	}

	return true
}

func (p *Phone) processRingingInput(input string) bool {
	input = trimSpace(input)
	if input == "" || input == "a" || input == "answer" {
		p.Answer()
	} else {
		p.Reject()
	}
	return true
}

func (p *Phone) processInCallInput(input string) bool {
	fmt.Printf("Hanging up call with %s\n", prettyHex(p.callerHash))
	p.Hangup()
	return true
}

func (p *Phone) printHelp() {
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  p - phonebook")
	fmt.Println("  r - redial last called")
	fmt.Println("  i - show identity")
	fmt.Println("  d - show destination hash")
	fmt.Println("  a - announce on network")
	fmt.Println("  q - quit")
	fmt.Println("  h - help")
	fmt.Println()
	fmt.Println("Enter identity hash to call, or command:")
}

func prettyHex(hash string) string {
	if len(hash) != 32 {
		return hash
	}
	return hash[:8] + ":" + hash[8:16] + ":" + hash[16:24] + ":" + hash[24:]
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// StatusString returns a human-readable status string for the current state.
func (p *Phone) StatusString() string {
	switch p.state {
	case StateAvailable:
		return "Available"
	case StateConnecting:
		return "Connecting"
	case StateRinging:
		return "Ringing"
	case StateInCall:
		return fmt.Sprintf("In call for %v", p.CallDuration().Truncate(time.Second))
	default:
		return "Unknown"
	}
}

// SetShouldRun controls the main loop.
func (p *Phone) SetShouldRun(run bool) {
	p.shouldRun = run
}

// ShouldRun returns whether the main loop should continue.
func (p *Phone) ShouldRun() bool {
	return p.shouldRun
}

// Start begins the main event loop.
func (p *Phone) Start() {
	p.shouldRun = true
}

// DefaultRnphoneConfig is the default configuration template.
const DefaultRnphoneConfig = `# This is an example rnphone config file.
# You should probably edit it to suit your
# intended usage.

[telephone]
    # You can define the ringtone played when the
    # phone is ringing. Must be in OPUS format, and
    # located in the rnphone config directory.
    
    ringtone = ringer.opus

    # You can define the preferred audio devices
    # to use as the speaker output, ringer output
    # and microphone input. The names do not have
    # to be an exact match to your full soundcard
    # device name, but will be fuzzy matched.
    # You can list available device names with:
    # rnphone -l
    
    # speaker = device name
    # microphone = device name
    # ringer = device name

    # You can configure who is allowed to call
    # this telephone. This can be set to either
    # "all", "none", "phonebook" or a list of
    # identity hashes. See examples below.

    # allowed_callers = all
    # allowed_callers = none
    # allowed_callers = phonebook
    # allowed_callers = b8d80b1b7a9d3147880b366995422a45, fcfb80d4cd3aab7c8710541fb2317974

    # It is also possible to block specific
    # callers on a per-identity basis.

    # blocked_callers = f3e8c3359b39d36f3baff0a616a73d3e, 5d2d14619dfa0ff06278c17347c14331

[phonebook]
    # You can add entries to the phonebook for
    # quick dialling by adding them here

    # Mary = f3e8c3359b39d36f3baff0a616a73d3e
    # Jake = b8d80b1b7a9d3147880b366995422a45
    # Dean = 05d4c6697bb38e5458a3077571157bfa

    # You can optionally specify a numerical
    # alias for calling with a physical keypad

    # Rudy = 5d2d14619dfa0ff06278c17347c14331, 241
    # Josh = fcfb80d4cd3aab7c8710541fb2317974, 7907

[hardware]
    # If the required hardware is connected, and
    # the neccessary modules installed, you can
    # enable various hardware components.
    
    # keypad = gpio_4x4
    # display = i2c_lcd1602

    # If you have a keypad connected, you can
    # also enable a GPIO pin for detecting
    # on-hook/off-hook status

    # keypad_hook_pin = 5
`
