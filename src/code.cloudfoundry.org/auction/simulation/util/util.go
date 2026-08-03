package util

import (
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var guidTracker map[string]int
var lock *sync.Mutex
var R *rand.Rand

func init() {
	ResetGuids()
	lock = &sync.Mutex{}
	R = rand.New(&lockedSource{src: rand.NewSource(time.Now().UnixNano())})
}

func ResetGuids() {
	guidTracker = map[string]int{}
}

func NewGuid(prefix string) string {
	guidTracker[prefix] = guidTracker[prefix] + 1
	return fmt.Sprintf("%s-%d", prefix, guidTracker[prefix])
}

func NewGrayscaleGuid(prefix string) string {
	guidTracker[prefix] = guidTracker[prefix] + 1
	col := R.Intn(200)
	return fmt.Sprintf("%s-%d-%s", prefix, guidTracker[prefix], rgb(col, col, col))
}

func rgb(r int, g int, b int) string {
	return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
}

func RandomIntIn(min, max int) int {
	return R.Intn(max-min+1) + min
}

func RandomSleep(minMilliseconds, maxMilliseconds int) {
	milliseconds := RandomIntIn(minMilliseconds, maxMilliseconds)
	time.Sleep(time.Duration(milliseconds) * time.Millisecond)
}

func RandomGuid() string {
	b := make([]byte, 8)
	lock.Lock()
	_, err := crand.Read(b)
	lock.Unlock()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x", b[0:2], b[2:4], b[4:6], b[6:8])
}

type lockedSource struct {
	lock sync.Mutex
	src  rand.Source
}

func (r *lockedSource) Int63() (n int64) {
	r.lock.Lock()
	n = r.src.Int63()
	r.lock.Unlock()
	return
}

func (r *lockedSource) Seed(seed int64) {
	r.lock.Lock()
	r.src.Seed(seed)
	r.lock.Unlock()
}
