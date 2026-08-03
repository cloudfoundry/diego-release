package uaaclient

import (
	"context"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	uaa "github.com/cloudfoundry-community/go-uaa"
	"golang.org/x/oauth2"
)

//go:generate counterfeiter -o fakes/token_fetcher.go . TokenFetcher
type TokenFetcher interface {
	FetchKey() (*uaa.JWK, error)
	FetchToken(ctx context.Context, forceUpdate bool) (*oauth2.Token, error)
}

func NewTokenFetcher(
	devMode bool,
	cfg Config,
	clk clock.Clock,
	maxNumberOfRetries uint,
	retryInterval time.Duration,
	expirationBufferInSec int64,
	logger lager.Logger,
) (TokenFetcher, error) {
	if devMode {
		logger.Info("using-noop-token-fetcher")
		return &noOpTokenFetcher{}, nil
	}

	api, err := NewAPI(cfg, logger)
	if err != nil {
		logger.Error("Failed to create UAA client", err)
		return nil, err
	}

	return &tokenFetcher{
		api:                   api,
		clock:                 clk,
		logger:                logger,
		maxNumberOfRetries:    maxNumberOfRetries,
		retryInterval:         retryInterval,
		expirationBufferInSec: expirationBufferInSec,
	}, nil
}

type noOpTokenFetcher struct {
}

func (f *noOpTokenFetcher) FetchKey() (*uaa.JWK, error) {
	return &uaa.JWK{}, nil
}

func (f *noOpTokenFetcher) FetchToken(ctx context.Context, forceUpdate bool) (*oauth2.Token, error) {
	return &oauth2.Token{}, nil
}

type tokenFetcher struct {
	clock  clock.Clock
	api    *uaa.API
	logger lager.Logger

	cachedToken           *oauth2.Token
	cachedTokenMutex      sync.Mutex
	refetchTokenTime      time.Time
	maxNumberOfRetries    uint
	retryInterval         time.Duration
	expirationBufferInSec int64
}

func (c *tokenFetcher) FetchKey() (*uaa.JWK, error) {
	return c.api.TokenKey()
}

func (c *tokenFetcher) FetchToken(ctx context.Context, forceUpdate bool) (*oauth2.Token, error) {
	logger := c.logger.Session("uaa-client")
	logger.Debug("started-fetching-token", lager.Data{"force-update": forceUpdate})

	c.cachedTokenMutex.Lock()
	defer c.cachedTokenMutex.Unlock()

	if !forceUpdate && c.canReturnCachedToken() {
		return c.cachedToken, nil
	}

	retry := true
	var retryCount uint = 0
	var token *oauth2.Token
	var err error
	for retry {
		token, err = c.api.Token(ctx)
		if token != nil {
			break
		}

		if err != nil {
			logger.Error("error-fetching-token", err)
		}

		if retry && retryCount < c.maxNumberOfRetries {
			logger.Debug("retry-fetching-token", lager.Data{"retry-count": retryCount})
			retryCount++
			c.clock.Sleep(c.retryInterval)
			continue
		} else {
			return nil, err
		}
	}

	logger.Debug("successfully-fetched-token")
	c.updateCachedToken(token)
	return c.cachedToken, err
}

func (c *tokenFetcher) canReturnCachedToken() bool {
	return c.cachedToken != nil && c.clock.Now().Before(c.refetchTokenTime)
}

func (c *tokenFetcher) updateCachedToken(token *oauth2.Token) {
	c.logger.Debug("caching-token")
	c.cachedToken = token
	c.refetchTokenTime = token.Expiry.Add(-1 * time.Duration(c.expirationBufferInSec) * time.Second)
}
