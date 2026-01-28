---
title: Workpool Example
expires_at : never
tags: [diego-release, workpool]
---

## Workpool Example

```go
type RateLimitingHandler struct {
	backend http.Handler
	pool    *workpool.WorkPool
}

// Ensures that the backend handler never processes more than maxInFlight requests at a time
func NewRateLimitingHandler(backend http.Handler, maxInFlight int) (*RateLimitingHandler, error) {
	pool, err := workpool.NewWorkPool(maxInFlight)
	if err != nil {
		return nil, err
	}

	return &RateLimitingHandler{
		backend: backend,
		pool:    pool,
	}, nil
}

func (rh *RateLimitingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	rh.pool.Submit(func() {
		rh.backend.ServeHTTP(w, req)
	})
}
```
