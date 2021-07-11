package util

import (
	"bytes"
	"github.com/drud/ddev/pkg/output"
	"io"
	"os"
)

// CaptureUserOut captures output written to UserOut to a string.
// Capturing starts when it is called. It returns an anonymous function that
// when called, will return a string containing the output during capture, and
// revert once again to the original value of os.StdOut.
func CaptureUserOut() func() string {
	old := output.UserOut.Out // keep backup of the real stdout
	r, w, _ := os.Pipe()
	output.UserOut.Out = w

	return func() string {
		outC := make(chan string)
		// copy the output in a separate goroutine so printing can't block indefinitely
		go func() {
			var buf bytes.Buffer
			_, err := io.Copy(&buf, r)
			CheckErr(err)
			outC <- buf.String()
		}()

		// back to normal state
		CheckClose(w)
		output.UserOut.Out = old // restoring the real stdout

		out := <-outC
		return out
	}
}

// CaptureStdOut captures Stdout to a string. Capturing starts when it is called. It returns an anonymous function that when called, will return a string
// containing the output during capture, and revert once again to the original value of os.StdOut.
func CaptureStdOut() func() string {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	return func() string {
		outC := make(chan string)
		// copy the output in a separate goroutine so printing can't block indefinitely
		go func() {
			var buf bytes.Buffer
			_, err := io.Copy(&buf, r)
			CheckErr(err)
			outC <- buf.String()
		}()

		// back to normal state
		CheckClose(w)
		os.Stdout = old // restoring the real stdout
		out := <-outC
		return out
	}
}

// CaptureOutputToFile captures Stdout to a string. Capturing starts when it is called. It returns an anonymous function that when called, will return a string
// containing the output during capture, and revert once again to the original value of os.StdOut.
func CaptureOutputToFile() (func() string, error) {
	oldStdout := os.Stdout // keep backup of the real stdout
	oldStderr := os.Stderr
	f, err := os.CreateTemp("", "CaptureOutputToFile")
	if err != nil {
		return nil, err
	}
	os.Stdout = f
	os.Stderr = f

	return func() string {
		_ = f.Close()
		os.Stdout = oldStdout // restoring the real stdout
		os.Stderr = oldStderr
		out, _ := os.ReadFile(f.Name())
		_ = os.RemoveAll(f.Name())
		return string(out)
	}, nil
}
