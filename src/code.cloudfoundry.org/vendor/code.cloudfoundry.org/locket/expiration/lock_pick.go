package expiration

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/db"
	"code.cloudfoundry.org/locket/models"
)

//go:generate counterfeiter . LockPick
type LockPick interface {
	RegisterTTL(logger lager.Logger, lock *db.Lock)
	ExpirationCounts() (uint32, uint32) // return lock and presence expirations, resp.
}

type lockPick struct {
	lockDB                db.LockDB
	clock                 clock.Clock
	metronClient          loggingclient.IngressClient
	lockTTLs              map[checkKey]chanAndIndex
	lockMutex             *sync.Mutex
	presencesExpiredCount *uint32
	locksExpiredCount     *uint32
}

type chanAndIndex struct {
	channel chan struct{}
	index   int64
}

type checkKey struct {
	key string
	id  string
}

func NewLockPick(lockDB db.LockDB, clock clock.Clock, metronClient loggingclient.IngressClient) lockPick {
	return lockPick{
		lockDB:                lockDB,
		clock:                 clock,
		metronClient:          metronClient,
		lockTTLs:              make(map[checkKey]chanAndIndex),
		lockMutex:             &sync.Mutex{},
		presencesExpiredCount: new(uint32),
		locksExpiredCount:     new(uint32),
	}
}

func (l lockPick) ExpirationCounts() (uint32, uint32) {
	return atomic.LoadUint32(l.locksExpiredCount), atomic.LoadUint32(l.presencesExpiredCount)
}

func (l lockPick) RegisterTTL(logger lager.Logger, lock *db.Lock) {
	logger = logger.Session("register-ttl", lager.Data{"key": lock.Key, "modified-index": lock.ModifiedIndex, "type": lock.Type})
	logger.Debug("starting")
	logger.Debug("completed")

	newChanIndex := chanAndIndex{
		channel: make(chan struct{}),
		index:   lock.ModifiedIndex,
	}
	l.lockMutex.Lock()
	defer l.lockMutex.Unlock()

	channelIndex, ok := l.lockTTLs[checkKeyFromLock(lock)]
	if ok && channelIndex.index >= newChanIndex.index {
		logger.Debug("found-expiration-goroutine-for-index", lager.Data{"index": channelIndex.index})
		return
	}

	if ok && channelIndex.index < newChanIndex.index {
		close(channelIndex.channel)
	}

	l.lockTTLs[checkKeyFromLock(lock)] = newChanIndex
	go l.checkExpiration(logger, lock, newChanIndex.channel)
}

func (l lockPick) checkExpiration(logger lager.Logger, lock *db.Lock, closeChan chan struct{}) {
	lockTimer := l.clock.NewTimer(time.Duration(lock.TtlInSeconds) * time.Second)

	select {
	case <-closeChan:
		logger.Debug("cancelling-old-check-goroutine")
		return
	case <-lockTimer.C():
		defer func() {
			l.lockMutex.Lock()
			chanIndex := l.lockTTLs[checkKeyFromLock(lock)]
			if chanIndex.index == lock.ModifiedIndex {
				delete(l.lockTTLs, checkKeyFromLock(lock))
			}
			l.lockMutex.Unlock()
		}()

		expired, err := l.lockDB.FetchAndRelease(context.Background(), logger, lock)
		if err != nil {
			logger.Error("failed-compare-and-release", err)
			return
		}

		if expired {
			logger.Info("lock-expired")
			counter := l.locksExpiredCount
			if lock.Type == models.PresenceType {
				counter = l.presencesExpiredCount
			}
			atomic.AddUint32(counter, 1)
		}
		return
	}
}

func checkKeyFromLock(lock *db.Lock) checkKey {
	return checkKey{
		key: lock.Key,
		id:  lock.ModifiedId,
	}
}
