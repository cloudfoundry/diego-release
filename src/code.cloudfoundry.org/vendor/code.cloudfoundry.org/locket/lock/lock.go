package lock

import (
	"os"
	"time"

	"context"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/models"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type lockRunner struct {
	logger lager.Logger

	locker         models.LocketClient
	lock           *models.Resource
	ttlInSeconds   int64
	clock          clock.Clock
	retryInterval  time.Duration
	exitOnLostLock bool
}

func NewLockRunner(
	logger lager.Logger,
	locker models.LocketClient,
	lock *models.Resource,
	ttlInSeconds int64,
	clock clock.Clock,
	retryInterval time.Duration,
) *lockRunner {
	return &lockRunner{
		logger:         logger,
		locker:         locker,
		lock:           lock,
		ttlInSeconds:   ttlInSeconds,
		clock:          clock,
		retryInterval:  retryInterval,
		exitOnLostLock: true,
	}
}

func NewPresenceRunner(
	logger lager.Logger,
	locker models.LocketClient,
	lock *models.Resource,
	ttlInSeconds int64,
	clock clock.Clock,
	retryInterval time.Duration,
) *lockRunner {
	return &lockRunner{
		logger:         logger,
		locker:         locker,
		lock:           lock,
		ttlInSeconds:   ttlInSeconds,
		clock:          clock,
		retryInterval:  retryInterval,
		exitOnLostLock: false,
	}
}

func contextWithRequestGUID() (context.Context, string, error) {
	ctx := context.Background()

	uuid, err := uuid.NewV4()
	if err != nil {
		return ctx, "", err
	}
	md := metadata.Pairs("uuid", uuid.String())
	return metadata.NewOutgoingContext(ctx, md), uuid.String(), nil
}

func (l *lockRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := l.logger.Session("locket-lock", lager.Data{"lock": l.lock, "ttl_in_seconds": l.ttlInSeconds})

	logger.Info("started")
	defer logger.Info("completed")

	var acquired, isReady bool
	ctx, uuid, err := contextWithRequestGUID()
	if err != nil {
		logger.Error("failed-to-create-context", err)
		return err
	}
	_, err = l.locker.Lock(ctx, &models.LockRequest{Resource: l.lock, TtlInSeconds: l.ttlInSeconds})
	if err != nil {
		lagerData := lager.Data{"request-uuid": uuid}
		resp, fErr := l.locker.Fetch(ctx, &models.FetchRequest{Key: l.lock.Key})
		if fErr != nil {
			logger.Error("failed-fetching-lock-owner", fErr)
		} else {
			lagerData["lock-owner"] = resp.Resource.Owner
		}
		logger.Error("failed-to-acquire-lock", err, lagerData)
	} else {
		logger.Info("acquired-lock")
		close(ready)
		acquired = true
		isReady = true
	}

	retry := l.clock.NewTimer(l.retryInterval)

	for {
		select {
		case sig := <-signals:
			logger.Info("signalled", lager.Data{"signal": sig})

			_, err := l.locker.Release(context.Background(), &models.ReleaseRequest{Resource: l.lock})
			if err != nil {
				logger.Error("failed-to-release-lock", err)
			} else {
				logger.Info("released-lock")
			}

			return nil

		case <-retry.C():
			ctx, uuid, err := contextWithRequestGUID()
			if err != nil {
				logger.Error("failed-to-create-context", err)
				return err
			}
			ctx, cancel := context.WithTimeout(ctx, l.retryInterval)
			start := time.Now()
			_, err = l.locker.Lock(ctx, &models.LockRequest{Resource: l.lock, TtlInSeconds: l.ttlInSeconds}, grpc.WaitForReady(true))
			cancel()
			if err != nil {
				if acquired {
					logger.Error("lost-lock", err, lager.Data{"request-uuid": uuid, "duration": time.Since(start)})
					if l.exitOnLostLock {
						return newLockLostError(err, uuid)
					}

					acquired = false
				} else if status.Code(err) != status.Code(models.ErrLockCollision) {
					logger.Error("failed-to-acquire-lock", err, lager.Data{"request-uuid": uuid, "duration": time.Since(start)})
				}
			} else if !acquired {
				logger.Info("acquired-lock")
				if !isReady {
					close(ready)
					isReady = true
				}
				acquired = true
			}

			retry.Reset(l.retryInterval)
		}
	}
}

func newLockLostError(err error, requestUUID string) error {
	additionalMessage := "request failed"
	switch status.Code(err) {
	case codes.DeadlineExceeded:
		additionalMessage = "request timed out"
	}
	return errors.Wrapf(err, "lost lock (%s), request-uuid %s", additionalMessage, requestUUID)
}
