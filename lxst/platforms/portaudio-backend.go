// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package platforms

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// This file implements a PortAudio-based audio backend using purego to
// dynamically load libportaudio at runtime — no CGO required. PortAudio
// provides cross-platform microphone input (recording) AND speaker output
// (playback) on macOS (CoreAudio), Windows (WASAPI/DirectSound), and
// Linux (ALSA/PulseAudio/JACK).
//
// The backend uses PortAudio's blocking read/write API (Pa_ReadStream /
// Pa_WriteStream) rather than the callback API, which integrates cleanly
// with the LXST pipeline's goroutine-based ingest/digest model.

var (
	ErrPortAudioNotInitialized = errors.New("portaudio not initialized")
	ErrPortAudioNoLibrary      = errors.New("portaudio library not found")
	ErrPortAudioNoDevice       = errors.New("no audio device available")
	ErrPortAudioOpenFailed     = errors.New("portaudio stream open failed")
	ErrPortAudioRecorderInUse  = errors.New("portaudio recorder already in use")
	ErrPortAudioPlayerInUse    = errors.New("portaudio player already in use")
)

// PortAudio sample format flags (from portaudio.h).
const (
	paFloat32 = 0x00000001
	paInt16   = 0x00000008
)

// PortAudio stream flags.
const (
	paNoFlag    = 0
	paClipOff   = 0x00000001
	paDitherOff = 0x00000002
)

// paNoDevice is the special PaDeviceIndex value for "no device".
const paNoDevice = -1

// paDeviceIndex and paHostApiIndex are C int types.
const (
	paFramesPerBufferUnspecified = 0
)

// portaudioLib holds the dynamically loaded portaudio shared library
// handle and bound function pointers. It is initialised once per process.
var portaudioLib struct {
	once    sync.Once
	loaded  bool
	loadErr error
	handle  uintptr

	paInitialize          func() int32
	paTerminate           func() int32
	paGetErrorText        func(int32) string
	paGetDeviceCount      func() int32
	paGetDefaultInputDev  func() int32
	paGetDefaultOutputDev func() int32
	paGetDeviceInfo       func(int32) *paDeviceInfo
	paOpenStream          func(*uintptr, *paStreamParameters, *paStreamParameters, float64, uint64, uint64, uintptr, uintptr) int32
	paCloseStream         func(uintptr) int32
	paStartStream         func(uintptr) int32
	paStopStream          func(uintptr) int32
	paAbortStream         func(uintptr) int32
	paReadStream          func(uintptr, unsafe.Pointer, uint64) int32
	paWriteStream         func(uintptr, unsafe.Pointer, uint64) int32
	paGetStreamInfo       func(uintptr) *paStreamInfo
}

// paDeviceInfo mirrors the C PaDeviceInfo struct.
type paDeviceInfo struct {
	StructVersion            int32
	Name                     *byte
	HostApi                  int32
	MaxInputChannels         int32
	MaxOutputChannels        int32
	DefaultLowInputLatency   float64
	DefaultLowOutputLatency  float64
	DefaultHighInputLatency  float64
	DefaultHighOutputLatency float64
	DefaultSampleRate        float64
}

// paStreamParameters mirrors the C PaStreamParameters struct.
type paStreamParameters struct {
	Device                    int32
	ChannelCount              int32
	SampleFormat              uint64
	SuggestedLatency          float64
	HostApiSpecificStreamInfo uintptr
}

// paStreamInfo mirrors the C PaStreamInfo struct.
type paStreamInfo struct {
	StructVersion int32
	InputLatency  float64
	OutputLatency float64
	SampleRate    float64
}

// loadPortAudio opens the portaudio shared library and binds the
// required functions. It runs exactly once via sync.Once. It tries the
// bare library name first (relying on the dynamic linker's search path,
// including DYLD_LIBRARY_PATH / LD_LIBRARY_PATH / PATH), then falls
// back to a set of well-known installation directories.
func loadPortAudio() {
	portaudioLib.once.Do(func() {
		libName := portAudioLibraryName()
		handle, err := openLibrary(libName)
		if err != nil {
			// Try well-known absolute paths for platforms where the
			// library is not in the default dynamic-linker search path
			// (e.g. Homebrew on Apple Silicon installs to
			// /opt/homebrew/lib which is not searched by default).
			for _, candidate := range portAudioLibraryPaths() {
				if h, e := openLibrary(candidate); e == nil {
					handle = h
					err = nil
					break
				}
			}
		}
		if err != nil {
			portaudioLib.loadErr = fmt.Errorf("%w: %v", ErrPortAudioNoLibrary, err)
			return
		}
		portaudioLib.handle = handle

		purego.RegisterLibFunc(&portaudioLib.paInitialize, handle, "Pa_Initialize")
		purego.RegisterLibFunc(&portaudioLib.paTerminate, handle, "Pa_Terminate")
		purego.RegisterLibFunc(&portaudioLib.paGetErrorText, handle, "Pa_GetErrorText")
		purego.RegisterLibFunc(&portaudioLib.paGetDeviceCount, handle, "Pa_GetDeviceCount")
		purego.RegisterLibFunc(&portaudioLib.paGetDefaultInputDev, handle, "Pa_GetDefaultInputDevice")
		purego.RegisterLibFunc(&portaudioLib.paGetDefaultOutputDev, handle, "Pa_GetDefaultOutputDevice")
		purego.RegisterLibFunc(&portaudioLib.paGetDeviceInfo, handle, "Pa_GetDeviceInfo")
		purego.RegisterLibFunc(&portaudioLib.paOpenStream, handle, "Pa_OpenStream")
		purego.RegisterLibFunc(&portaudioLib.paCloseStream, handle, "Pa_CloseStream")
		purego.RegisterLibFunc(&portaudioLib.paStartStream, handle, "Pa_StartStream")
		purego.RegisterLibFunc(&portaudioLib.paStopStream, handle, "Pa_StopStream")
		purego.RegisterLibFunc(&portaudioLib.paAbortStream, handle, "Pa_AbortStream")
		purego.RegisterLibFunc(&portaudioLib.paReadStream, handle, "Pa_ReadStream")
		purego.RegisterLibFunc(&portaudioLib.paWriteStream, handle, "Pa_WriteStream")
		purego.RegisterLibFunc(&portaudioLib.paGetStreamInfo, handle, "Pa_GetStreamInfo")

		if rc := portaudioLib.paInitialize(); rc != 0 {
			portaudioLib.loadErr = fmt.Errorf("Pa_Initialize failed: %s", portaudioLib.paGetErrorText(rc))
			return
		}
		portaudioLib.loaded = true
	})
}

// portAudioLibraryName returns the shared library filename for the
// current platform.
func portAudioLibraryName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libportaudio.dylib"
	case "windows":
		return "portaudio.dll"
	default:
		return "libportaudio.so"
	}
}

// portAudioLibraryPaths returns well-known absolute paths where the
// PortAudio shared library is commonly installed, used as fallbacks
// when the bare filename cannot be found by the dynamic linker.
func portAudioLibraryPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		// Homebrew on Apple Silicon and Intel, plus MacPorts.
		return []string{
			"/opt/homebrew/lib/libportaudio.dylib",
			"/usr/local/lib/libportaudio.dylib",
			"/opt/local/lib/libportaudio.dylib",
		}
	case "windows":
		return []string{
			"portaudio.dll",
			"C:\\Windows\\System32\\portaudio.dll",
		}
	default:
		// Linux: ALSA/PulseAudio builds install libportaudio.so.2 in
		// the standard multiarch lib directories.
		return []string{
			"libportaudio.so.2",
			"libportaudio.so",
			"/usr/lib/x86_64-linux-gnu/libportaudio.so.2",
			"/usr/lib/x86_64-linux-gnu/libportaudio.so",
			"/usr/lib/aarch64-linux-gnu/libportaudio.so.2",
			"/usr/lib/aarch64-linux-gnu/libportaudio.so",
			"/usr/lib/libportaudio.so.2",
			"/usr/lib/libportaudio.so",
			"/usr/local/lib/libportaudio.so.2",
			"/usr/local/lib/libportaudio.so",
		}
	}
}

// paErrorString returns a human-readable error string for a PortAudio
// error code. Safe to call even if the library failed to load.
func paErrorString(rc int32) string {
	if portaudioLib.loaded && portaudioLib.paGetErrorText != nil {
		return portaudioLib.paGetErrorText(rc)
	}
	return fmt.Sprintf("portaudio error %v", rc)
}

// PortAudioBackend implements AudioBackend using libportaudio via purego.
type PortAudioBackend struct {
	sampleRate int
	channels   int
	bitDepth   int

	mu         sync.Mutex
	recorderMu sync.Mutex
	playerMu   sync.Mutex

	recorder *portAudioRecorder
	player   *portAudioPlayer

	micNames     []string
	speakerNames []string
}

// portAudioRecorder wraps a PortAudio input stream for blocking reads.
type portAudioRecorder struct {
	stream         uintptr
	sampleRate     int
	channels       int
	samplesPerRead int
	readBuf        []float32
	closed         bool
	mu             sync.Mutex
}

// portAudioPlayer wraps a PortAudio output stream for blocking writes.
type portAudioPlayer struct {
	stream uintptr
	closed bool
	mu     sync.Mutex
}

// NewPortAudioBackend creates a new PortAudio-based audio backend.
// Returns an error if libportaudio cannot be loaded or initialised.
func NewPortAudioBackend(sampleRate, channels, bitDepth int) (AudioBackend, error) {
	loadPortAudio()
	if portaudioLib.loadErr != nil {
		return nil, portaudioLib.loadErr
	}
	if !portaudioLib.loaded {
		return nil, ErrPortAudioNoLibrary
	}

	if sampleRate <= 0 {
		sampleRate = 48000
	}
	if channels <= 0 {
		channels = 2
	}
	if channels > 2 {
		channels = 2
	}

	pa := &PortAudioBackend{
		sampleRate: sampleRate,
		channels:   channels,
		bitDepth:   bitDepth,
	}

	pa.micNames = pa.enumerateDevices(true)
	pa.speakerNames = pa.enumerateDevices(false)

	return pa, nil
}

// enumerateDevices returns the list of input or output device names.
func (pa *PortAudioBackend) enumerateDevices(input bool) []string {
	count := portaudioLib.paGetDeviceCount()
	if count <= 0 {
		return []string{"default"}
	}

	var result []string
	for i := int32(0); i < count; i++ {
		info := portaudioLib.paGetDeviceInfo(i)
		if info == nil {
			continue
		}
		if input && info.MaxInputChannels > 0 {
			result = append(result, fmt.Sprintf("<Microphone %s>", paDeviceName(info)))
		}
		if !input && info.MaxOutputChannels > 0 {
			result = append(result, fmt.Sprintf("<Speaker %s>", paDeviceName(info)))
		}
	}

	if len(result) == 0 {
		return []string{"default"}
	}
	return result
}

// paDeviceName safely extracts the device name from a PaDeviceInfo.
func paDeviceName(info *paDeviceInfo) string {
	if info == nil || info.Name == nil {
		return "unknown"
	}
	return stringPtrToString(info.Name)
}

// stringPtrToString converts a C *char to a Go string safely.
func stringPtrToString(p *byte) string {
	if p == nil {
		return ""
	}
	// Find the null terminator.
	length := 0
	for ptr := unsafe.Pointer(p); *(*byte)(ptr) != 0; ptr = unsafe.Add(ptr, 1) {
		length++
	}
	// Copy into a Go slice so the GC can manage it and the result does
	// not rely on the C memory remaining valid.
	buf := make([]byte, length)
	for i := 0; i < length; i++ {
		buf[i] = *(*byte)(unsafe.Add(unsafe.Pointer(p), i))
	}
	return string(buf)
}

func (pa *PortAudioBackend) SampleRate() int { return pa.sampleRate }
func (pa *PortAudioBackend) Channels() int   { return pa.channels }
func (pa *PortAudioBackend) BitDepth() int   { return pa.bitDepth }

func (pa *PortAudioBackend) AllMicrophones() []string {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	return pa.micNames
}

func (pa *PortAudioBackend) DefaultMicrophone() string {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	if len(pa.micNames) > 0 {
		return pa.micNames[0]
	}
	return "default"
}

func (pa *PortAudioBackend) AllSpeakers() []string {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	return pa.speakerNames
}

func (pa *PortAudioBackend) DefaultSpeaker() string {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	if len(pa.speakerNames) > 0 {
		return pa.speakerNames[0]
	}
	return "default"
}

func (pa *PortAudioBackend) Flush() error { return nil }

func (pa *PortAudioBackend) ReleaseRecorder() error {
	pa.recorderMu.Lock()
	defer pa.recorderMu.Unlock()
	if pa.recorder != nil {
		if err := pa.recorder.Close(); err != nil {
			log.Printf("PortAudioBackend.ReleaseRecorder: recorder.Close failed: %v", err)
		}
		pa.recorder = nil
	}
	return nil
}

func (pa *PortAudioBackend) ReleasePlayer() error {
	pa.playerMu.Lock()
	defer pa.playerMu.Unlock()
	if pa.player != nil {
		if err := pa.player.Close(); err != nil {
			log.Printf("PortAudioBackend.ReleasePlayer: player.Close failed: %v", err)
		}
		pa.player = nil
	}
	return nil
}

// resolveInputDeviceIndex finds the PortAudio device index for the
// preferred input device name, falling back to the default input device.
func (pa *PortAudioBackend) resolveInputDeviceIndex(preferred string) int32 {
	return pa.resolveDeviceIndex(preferred, true)
}

// resolveOutputDeviceIndex finds the PortAudio device index for the
// preferred output device name, falling back to the default output device.
func (pa *PortAudioBackend) resolveOutputDeviceIndex(preferred string) int32 {
	return pa.resolveDeviceIndex(preferred, false)
}

// resolveDeviceIndex maps a friendly device name (as returned by
// AllMicrophones/AllSpeakers) to a PortAudio device index. If the
// preferred name is empty or not found, the system default is used.
func (pa *PortAudioBackend) resolveDeviceIndex(preferred string, input bool) int32 {
	if preferred == "" {
		if input {
			return portaudioLib.paGetDefaultInputDev()
		}
		return portaudioLib.paGetDefaultOutputDev()
	}

	// Allow matching against the friendly "<Microphone Name>" /
	// "<Speaker Name>" strings or the raw device name.
	needle := strings.TrimSpace(preferred)
	needle = strings.TrimPrefix(needle, "<Microphone ")
	needle = strings.TrimPrefix(needle, "<Speaker ")
	needle = strings.TrimSuffix(needle, ">")

	count := portaudioLib.paGetDeviceCount()
	for i := int32(0); i < count; i++ {
		info := portaudioLib.paGetDeviceInfo(i)
		if info == nil {
			continue
		}
		name := paDeviceName(info)
		if !strings.EqualFold(name, needle) {
			continue
		}
		if input && info.MaxInputChannels > 0 {
			return i
		}
		if !input && info.MaxOutputChannels > 0 {
			return i
		}
	}

	if input {
		return portaudioLib.paGetDefaultInputDev()
	}
	return portaudioLib.paGetDefaultOutputDev()
}

func (pa *PortAudioBackend) GetRecorder(samplesPerFrame int) (AudioRecorder, error) {
	loadPortAudio()
	if !portaudioLib.loaded {
		return nil, ErrPortAudioNoLibrary
	}

	pa.recorderMu.Lock()
	defer pa.recorderMu.Unlock()

	if pa.recorder != nil {
		return nil, ErrPortAudioRecorderInUse
	}

	if samplesPerFrame <= 0 {
		samplesPerFrame = 960
	}

	devIdx := pa.resolveInputDeviceIndex("")
	if devIdx == paNoDevice {
		return nil, ErrPortAudioNoDevice
	}

	info := portaudioLib.paGetDeviceInfo(devIdx)
	if info == nil || info.MaxInputChannels <= 0 {
		return nil, ErrPortAudioNoDevice
	}

	channels := int32(pa.channels)
	if channels > info.MaxInputChannels {
		channels = info.MaxInputChannels
	}

	params := &paStreamParameters{
		Device:           devIdx,
		ChannelCount:     channels,
		SampleFormat:     paFloat32,
		SuggestedLatency: info.DefaultLowInputLatency,
	}

	var stream uintptr
	rc := portaudioLib.paOpenStream(
		&stream,
		params,
		nil,
		float64(pa.sampleRate),
		uint64(paFramesPerBufferUnspecified),
		paNoFlag,
		0,
		0,
	)
	if rc != 0 {
		return nil, fmt.Errorf("%w: %s", ErrPortAudioOpenFailed, paErrorString(rc))
	}

	if rc := portaudioLib.paStartStream(stream); rc != 0 {
		_ = portaudioLib.paCloseStream(stream)
		return nil, fmt.Errorf("Pa_StartStream failed: %s", paErrorString(rc))
	}

	rec := &portAudioRecorder{
		stream:         stream,
		sampleRate:     pa.sampleRate,
		channels:       int(channels),
		samplesPerRead: samplesPerFrame,
		readBuf:        make([]float32, samplesPerFrame*int(channels)),
	}
	pa.recorder = rec
	return rec, nil
}

func (pa *PortAudioBackend) GetPlayer(samplesPerFrame int, lowLatency bool) (AudioPlayer, error) {
	loadPortAudio()
	if !portaudioLib.loaded {
		return nil, ErrPortAudioNoLibrary
	}

	pa.playerMu.Lock()
	defer pa.playerMu.Unlock()

	if pa.player != nil {
		log.Printf("PortAudioBackend.GetPlayer: player already in use (spf=%v, lowLatency=%v, backend=%p)", samplesPerFrame, lowLatency, pa)
		return nil, ErrPortAudioPlayerInUse
	}

	if samplesPerFrame <= 0 {
		samplesPerFrame = 960
	}

	devIdx := pa.resolveOutputDeviceIndex("")
	if devIdx == paNoDevice {
		return nil, ErrPortAudioNoDevice
	}

	info := portaudioLib.paGetDeviceInfo(devIdx)
	if info == nil || info.MaxOutputChannels <= 0 {
		return nil, ErrPortAudioNoDevice
	}

	channels := int32(pa.channels)
	if channels > info.MaxOutputChannels {
		channels = info.MaxOutputChannels
	}

	latency := info.DefaultLowOutputLatency
	if !lowLatency && info.DefaultHighOutputLatency > 0 {
		latency = info.DefaultHighOutputLatency
	}

	params := &paStreamParameters{
		Device:           devIdx,
		ChannelCount:     channels,
		SampleFormat:     paFloat32,
		SuggestedLatency: latency,
	}

	var stream uintptr
	rc := portaudioLib.paOpenStream(
		&stream,
		nil,
		params,
		float64(pa.sampleRate),
		uint64(paFramesPerBufferUnspecified),
		paNoFlag,
		0,
		0,
	)
	if rc != 0 {
		return nil, fmt.Errorf("%w: %s", ErrPortAudioOpenFailed, paErrorString(rc))
	}

	if rc := portaudioLib.paStartStream(stream); rc != 0 {
		_ = portaudioLib.paCloseStream(stream)
		return nil, fmt.Errorf("Pa_StartStream failed: %s", paErrorString(rc))
	}

	plr := &portAudioPlayer{stream: stream}
	pa.player = plr
	return plr, nil
}

// FramesPerBuffer is unused — kept for documentation. PortAudio
// accepts 0 (paFramesPerBufferUnspecified) so the host API picks an
// optimal block size for the blocking read/write API.
func (pa *PortAudioBackend) FramesPerBuffer(samplesPerFrame, channels int) int {
	_ = samplesPerFrame
	_ = channels
	return paFramesPerBufferUnspecified
}

// Record reads numFrames of interleaved float32 audio from the input
// stream and de-interleaves it into [numFrames][channels] layout.
func (r *portAudioRecorder) Record(numFrames int) ([][]float32, error) {
	r.mu.Lock()
	closed := r.closed
	r.mu.Unlock()
	if closed {
		return nil, errors.New("recorder closed")
	}

	channels := r.channels
	totalSamples := numFrames * channels
	if totalSamples > len(r.readBuf) {
		r.readBuf = make([]float32, totalSamples)
	}
	buf := r.readBuf[:totalSamples]

	// Pa_ReadStream blocks until the entire buffer is filled.
	rc := portaudioLib.paReadStream(r.stream, unsafe.Pointer(&buf[0]), uint64(numFrames))
	if rc != 0 {
		// On overflow or transient errors, return silence rather than
		// breaking the pipeline.
		frame := make([][]float32, numFrames)
		for i := range frame {
			frame[i] = make([]float32, channels)
		}
		return frame, nil
	}

	// De-interleave: buf is [frame0ch0, frame0ch1, frame1ch0, ...].
	frame := make([][]float32, numFrames)
	for i := 0; i < numFrames; i++ {
		frame[i] = make([]float32, channels)
		for ch := 0; ch < channels; ch++ {
			frame[i][ch] = buf[i*channels+ch]
		}
	}
	return frame, nil
}

func (r *portAudioRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.stream != 0 {
		_ = portaudioLib.paAbortStream(r.stream)
		_ = portaudioLib.paCloseStream(r.stream)
		r.stream = 0
	}
	return nil
}

// Play accepts a [numFrames][channels] float32 frame, interleaves it,
// and writes it to the output stream (blocking until consumed).
func (p *portAudioPlayer) Play(frame [][]float32) error {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return errors.New("player closed")
	}

	numFrames := len(frame)
	if numFrames == 0 {
		return nil
	}
	channels := len(frame[0])
	if channels == 0 {
		return nil
	}

	// Interleave into a single buffer.
	buf := make([]float32, numFrames*channels)
	for i := 0; i < numFrames; i++ {
		for ch := 0; ch < channels && ch < len(frame[i]); ch++ {
			buf[i*channels+ch] = frame[i][ch]
		}
	}

	rc := portaudioLib.paWriteStream(p.stream, unsafe.Pointer(&buf[0]), uint64(numFrames))
	if rc != 0 {
		return fmt.Errorf("Pa_WriteStream failed: %s", paErrorString(rc))
	}
	return nil
}

func (p *portAudioPlayer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	if p.stream != 0 {
		_ = portaudioLib.paStopStream(p.stream)
		_ = portaudioLib.paCloseStream(p.stream)
		p.stream = 0
	}
	return nil
}

func (p *portAudioPlayer) EnableLowLatency() error {
	// Latency is set at stream-open time via SuggestedLatency. This is a
	// no-op for an already-open stream.
	return nil
}

// Ensure PortAudioBackend implements AudioBackend.
var _ AudioBackend = (*PortAudioBackend)(nil)
