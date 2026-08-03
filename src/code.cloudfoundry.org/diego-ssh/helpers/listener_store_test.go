package helpers_test

import (
	"fmt"
	"sync"

	"code.cloudfoundry.org/diego-ssh/helpers"
	"code.cloudfoundry.org/diego-ssh/test_helpers/fake_net"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ListenerStore", func() {
	var lnStore *helpers.ListenerStore
	JustBeforeEach(func() {
		lnStore = helpers.NewListenerStore()
	})

	It("concurrently adds and removes", func() {
		addrs := make(chan string, 100)
		ln := &fake_net.FakeListener{}

		wg := sync.WaitGroup{}
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(i int) {
				addr := fmt.Sprintf("127.0.0.1:%d", 8080+i)
				defer wg.Done()
				lnStore.AddListener(addr, ln)
				addrs <- addr
			}(i)
		}

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				addr := <-addrs
				defer GinkgoRecover()
				defer wg.Done()
				err := lnStore.RemoveListener(addr)
				Expect(err).ToNot(HaveOccurred())
			}()
		}
		wg.Wait()

		Expect(lnStore.ListAll()).To(HaveLen(0))
	})

	Describe("RemoveListener", func() {
		It("closes listeners when it removes them", func() {
			ln := &fake_net.FakeListener{}
			addr := "127.0.0.1:8080"
			lnStore.AddListener(addr, ln)
			lnStore.RemoveListener(addr)
			Expect(ln.CloseCallCount()).To(Equal(1))
		})

		It("errors if the requested listener does not exist", func() {
			err := lnStore.RemoveListener("127.0.0.1:12345")
			Expect(err).To(MatchError("RemoveListener error: addr 127.0.0.1:12345 doesn't exist"))
		})
	})

	Describe("RemoveAll", func() {
		It("removes and closes all listeners", func() {
			ln1 := &fake_net.FakeListener{}
			addr1 := "127.0.0.1:8080"
			ln2 := &fake_net.FakeListener{}
			addr2 := "127.0.0.1:8081"

			lnStore.AddListener(addr1, ln1)
			lnStore.AddListener(addr2, ln2)

			lnStore.RemoveAll()
			Expect(lnStore.ListAll()).To(HaveLen(0))
			Expect(ln1.CloseCallCount()).To(Equal(1))
			Expect(ln2.CloseCallCount()).To(Equal(1))
		})
	})
})
