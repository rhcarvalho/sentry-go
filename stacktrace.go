package sentry

import (
	"go/build"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

const unknown string = "unknown"

// The module download is split into two parts: downloading the go.mod and downloading the actual code.
// If you have dependencies only needed for tests, then they will show up in your go.mod,
// and go get will download their go.mods, but it will not download their code.
// The test-only dependencies get downloaded only when you need it, such as the first time you run go test.
//
// https://github.com/golang/go/issues/26913#issuecomment-411976222

// Stacktrace holds information about the frames of the stack.
type Stacktrace struct {
	Frames []Frame `json:"frames,omitempty"`
	// REVIEW: FramesOmitted was added to Protocol version 5 (https://docs.sentry.io/server/changelog/#protocol-version-5)
	// but is not listed in the Unified API: https://docs.sentry.io/development/sdk-dev/event-payloads/stacktrace/
	// See also: https://github.com/getsentry/sentry/blob/1f1771d6a1ce2b8e9077637c153be42dd8112a2f/src/sentry/interfaces/stacktrace.py#L370-L468
	FramesOmitted []uint `json:"frames_omitted,omitempty"`
}

// NewStacktrace creates a Stacktrace with Frames.... and omits frames ....
// Returns nil when...
// REVIEW: godoc
func NewStacktrace() *Stacktrace {
	// REVIEW: isn't 100 too much?
	pc := make([]uintptr, 100)
	const skip = 2 // skip runtime.Callers and NewStacktrace itself
	n := runtime.Callers(skip, pc)
	if n == 0 {
		return nil
	}
	pc = pc[:n] // only keep valid pcs

	return &Stacktrace{
		Frames: userStackFrames(pc),
	}
}

// userStackFrames returns Go runtime stack frames relevant for users of
// sentry-go. It does not include frames internal to sentry-go nor the Go
// runtime. Following Sentry's Unified API convention, the last frame is the one
// that called sentry-go.
func userStackFrames(pc []uintptr) []Frame {
	frames := runtime.CallersFrames(pc)

	var s []Frame
	for {
		frame, more := frames.Next()

		// Skip frames that are internal to the SDK.
		if strings.HasPrefix(frame.Function, "github.com/getsentry/sentry-go.") {
			continue
		}

		// Once a Go runtime frame is reached ignore all following frames as not
		// being relevant to debug user code. Typically, that means that we stop
		// at main.main.
		if strings.HasPrefix(frame.Function, "runtime.") {
			break
		}

		s = append(s, NewFrame(frame))

		if !more {
			break
		}
	}

	// Reverse the slice to match the order expected by the Sentry API.
	for i := len(s)/2 - 1; i >= 0; i-- {
		opp := len(s) - 1 - i
		s[i], s[opp] = s[opp], s[i]
	}
	return s
}

// ExtractStacktrace creates a new `Stacktrace` based on the given `error` object.
// Returns nil when...
// REVIEW: godoc
// TODO: Make it configurable so that anyone can provide their own implementation?
// Use of reflection allows us to not have a hard dependency on any given package, so we don't have to import it
func ExtractStacktrace(err error) *Stacktrace {
	method := extractReflectedStacktraceMethod(err)
	if !method.IsValid() {
		return nil
	}

	pc := extractPcs(method)
	if len(pc) == 0 {
		return nil
	}

	return &Stacktrace{
		Frames: userStackFrames(pc),
	}
}

func extractReflectedStacktraceMethod(err error) reflect.Value {
	var method reflect.Value

	// https://github.com/pingcap/errors
	methodGetStackTracer := reflect.ValueOf(err).MethodByName("GetStackTracer")
	// https://github.com/pkg/errors
	methodStackTrace := reflect.ValueOf(err).MethodByName("StackTrace")
	// https://github.com/go-errors/errors
	methodStackFrames := reflect.ValueOf(err).MethodByName("StackFrames")

	if methodGetStackTracer.IsValid() {
		stacktracer := methodGetStackTracer.Call(make([]reflect.Value, 0))[0]
		stacktracerStackTrace := reflect.ValueOf(stacktracer).MethodByName("StackTrace")

		if stacktracerStackTrace.IsValid() {
			method = stacktracerStackTrace
		}
	}

	if methodStackTrace.IsValid() {
		method = methodStackTrace
	}

	if methodStackFrames.IsValid() {
		method = methodStackFrames
	}

	return method
}

func extractPcs(method reflect.Value) []uintptr {
	var pcs []uintptr

	stacktrace := method.Call(make([]reflect.Value, 0))[0]

	if stacktrace.Kind() != reflect.Slice {
		return nil
	}

	for i := 0; i < stacktrace.Len(); i++ {
		pc := stacktrace.Index(i)

		if pc.Kind() == reflect.Uintptr {
			pcs = append(pcs, uintptr(pc.Uint()))
			continue
		}

		if pc.Kind() == reflect.Struct {
			field := pc.FieldByName("ProgramCounter")
			if field.IsValid() && field.Kind() == reflect.Uintptr {
				pcs = append(pcs, uintptr(field.Uint()))
				continue
			}
		}
	}

	return pcs
}

// https://docs.sentry.io/development/sdk-dev/event-payloads/stacktrace/
type Frame struct {
	Function string `json:"function,omitempty"`
	Symbol   string `json:"symbol,omitempty"`
	Module   string `json:"module,omitempty"`
	// REVIEW: Why do we use Module instead of Package? In Go, those two have a
	// very specific meaning. Package is never set.
	Package     string                 `json:"package,omitempty"`
	Filename    string                 `json:"filename,omitempty"`
	AbsPath     string                 `json:"abs_path,omitempty"`
	Lineno      int                    `json:"lineno,omitempty"`
	Colno       int                    `json:"colno,omitempty"`
	PreContext  []string               `json:"pre_context,omitempty"`
	ContextLine string                 `json:"context_line,omitempty"`
	PostContext []string               `json:"post_context,omitempty"`
	InApp       bool                   `json:"in_app,omitempty"`
	Vars        map[string]interface{} `json:"vars,omitempty"`
}

// NewFrame assembles a stacktrace frame out of `runtime.Frame`.
func NewFrame(f runtime.Frame) Frame {
	abspath := f.File
	filename := f.File
	function := f.Function
	var module string

	if filename != "" {
		filename = filepath.Base(filename)
	} else {
		filename = unknown // REVIEW: why "unknown" instead of the empty string?
	}

	if abspath == "" {
		abspath = unknown // REVIEW: why "unknown" instead of the empty string?
	}

	if function != "" {
		module, function = deconstructFunctionName(function)
	}

	frame := Frame{
		AbsPath:  abspath,
		Filename: filename,
		Lineno:   f.Line,
		Module:   module,
		Function: function,
	}

	frame.InApp = isInAppFrame(frame)

	return frame
}

// REVIEW: what does the return value of isInAppFrame mean?
// FIXME: replace if condition...return bool with return !condition
func isInAppFrame(frame Frame) bool {
	if strings.HasPrefix(frame.AbsPath, build.Default.GOROOT) ||
		strings.Contains(frame.Module, "vendor") ||
		strings.Contains(frame.Module, "third_party") { // REVIEW: why is "third_party" special?
		return false
	}

	return true
}

// Transform `runtime/debug.*T·ptrmethod` into `{ module: runtime/debug, function: *T.ptrmethod }`
func deconstructFunctionName(name string) (module string, function string) {
	// TODO: handle anonymous functions like:
	// github.com/getsentry/sentry-go_test.TestNewStacktrace.func1
	if idx := strings.LastIndex(name, "."); idx != -1 {
		module = name[:idx]
		function = name[idx+1:]
	}
	// REVIEW: when do we actually need to replace "·"?
	// Why do we only split on "." above?
	function = strings.Replace(function, "·", ".", -1)
	return module, function
}

func callerFunctionName() string {
	// REVIEW
	pcs := make([]uintptr, 1)
	runtime.Callers(3, pcs)
	callersFrames := runtime.CallersFrames(pcs)
	callerFrame, _ := callersFrames.Next()
	_, function := deconstructFunctionName(callerFrame.Function)
	return function
}
