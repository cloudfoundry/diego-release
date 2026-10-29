package log_streamer

import "sync/atomic"

// tagStore holds the tags shared by every streamDestination (stdout, stderr
// and all WithSource children) and the logRateLimiter of a single container.
// Each of those has its own lock (or none), so the map must never be mutated
// in place: Store swaps in a fresh copy and readers get an immutable snapshot.
type tagStore struct {
	tags atomic.Pointer[map[string]string]
}

func newTagStore(tags map[string]string) *tagStore {
	s := &tagStore{}
	s.Store(tags)
	return s
}

// Load returns a snapshot that must not be modified.
func (s *tagStore) Load() map[string]string {
	return *s.tags.Load()
}

func (s *tagStore) Store(tags map[string]string) {
	snapshot := make(map[string]string, len(tags))
	for k, v := range tags {
		snapshot[k] = v
	}
	s.tags.Store(&snapshot)
}
