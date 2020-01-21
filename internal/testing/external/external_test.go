// external has tests / benchmarks targetting external processes. This is useful
// to test black box programs that use Sentry, including those not written in
// Go.
package external

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func Benchmark(b *testing.B) {
	var counter uint64
	// ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	time.Sleep(80 * time.Millisecond)
	// 	atomic.AddUint64(&counter, 1)
	// }))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&counter, 1)
	}))
	defer ts.Close()

	// TODO: write certificate to a file and set SSL_CERT_FILE.
	// ts.Certificate()

	pths := []string{"sentry-python", "python-certifi"}
	var err error
	for i, pth := range pths {
		pth, err = filepath.Abs(pth)
		if err != nil {
			b.Fatal(err)
		}
		pths[i] = pth
	}

	cmd := exec.Command("python3", "python.py")
	cmd.Env = []string{
		// fmt.Sprintf("SENTRY_DSN=https://user@%s/42", ts.Listener.Addr(),
		fmt.Sprintf("SENTRY_DSN=http://user@%s/42", ts.Listener.Addr()),
		fmt.Sprintf("TEST_N=%d", b.N),
		fmt.Sprintf("PYTHONPATH=%s", strings.Join(pths, ":")),
	}

	b.ResetTimer()
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.Errorf("err:\n%s\nout:\n%s", err, out)
	}

	if counter != uint64(b.N) {
		b.Errorf("counter = %d, want %d\nout:\n%s", counter, b.N, out)
	}
}
