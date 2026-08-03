package handlers

import (
	"net/http"
	"sync"
)

type UnavailableHandler struct {
	handler http.Handler
	waitCh  <-chan struct{}
}

func NewUnavailableHandler(handler http.Handler, serviceReadyChan ...<-chan struct{}) *UnavailableHandler {
	wg := sync.WaitGroup{}
	for _, ch := range serviceReadyChan {
		wg.Add(1)
		go func(ch <-chan struct{}) {
			defer wg.Done()
			<-ch
		}(ch)
	}

	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	u := &UnavailableHandler{
		handler: handler,
		waitCh:  waitCh,
	}

	return u
}

func (u *UnavailableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case <-u.waitCh:
		u.handler.ServeHTTP(w, r)
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func UnavailableWrap(handler http.Handler, serviceReady ...<-chan struct{}) http.HandlerFunc {
	handler = NewUnavailableHandler(handler, serviceReady...)

	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}
