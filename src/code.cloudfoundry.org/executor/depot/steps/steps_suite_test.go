package steps_test

import (
	"time"

	multierror "github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gleak"

	"testing"
)

var (
	goroutineErrors *multierror.Error
	checkGoroutines bool
)

func TestSteps(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Steps Suite")
}

var _ = BeforeEach(func() {
	goroutineErrors = &multierror.Error{}
	checkGoroutines = true
	snapshot := gleak.Goroutines()
	DeferCleanup(func() {
		if checkGoroutines {
			Eventually(gleak.Goroutines).ShouldNot(gleak.HaveLeaked(snapshot))
		} else {
			// allow enough time for the goroutines to stabilize. Otherwise, sometimes
			// new tests could start while goroutines are being created and the leaked
			// goroutine is reported for an unrelated test. This is specially true for
			// the monitor_test.go which disables goroutine checks.
			time.Sleep(time.Second)
		}
		if err := goroutineErrors.ErrorOrNil(); err != nil {
			Fail(err.Error())
		}

	})
})

type NonDisplayableError struct{}

func (e NonDisplayableError) Error() string       { return "some-non-displaybale-error" }
func (e NonDisplayableError) IsDisplayable() bool { return false }
