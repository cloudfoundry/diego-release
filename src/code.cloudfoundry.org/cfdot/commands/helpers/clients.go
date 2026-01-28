package helpers

import (
	"strings"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket"
	locketmodels "code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/rep"
	"github.com/spf13/cobra"
)

const (
	clientSessionCacheSize int = -1
	maxIdleConnsPerHost    int = -1
)

type TLSConfig struct {
	BBSUrl            string
	LocketApiLocation string
	CACertFile        string
	CertFile          string
	KeyFile           string
	SkipCertVerify    bool
	Timeout           int
}

func NewBBSClient(cmd *cobra.Command, bbsClientConfig TLSConfig) (bbs.Client, error) {
	var err error
	var client bbs.Client

	if !strings.HasPrefix(bbsClientConfig.BBSUrl, "https") {
		client, err = bbs.NewClientWithConfig(bbs.ClientConfig{
			URL:            bbsClientConfig.BBSUrl,
			Retries:        1,
			RequestTimeout: time.Duration(bbsClientConfig.Timeout) * time.Second,
		})
	} else {
		client, err = bbs.NewClientWithConfig(
			bbs.ClientConfig{
				URL:                    bbsClientConfig.BBSUrl,
				IsTLS:                  true,
				InsecureSkipVerify:     bbsClientConfig.SkipCertVerify,
				CAFile:                 bbsClientConfig.CACertFile,
				CertFile:               bbsClientConfig.CertFile,
				KeyFile:                bbsClientConfig.KeyFile,
				ClientSessionCacheSize: clientSessionCacheSize,
				MaxIdleConnsPerHost:    maxIdleConnsPerHost,
				Retries:                1,
				RequestTimeout:         time.Duration(bbsClientConfig.Timeout) * time.Second,
			},
		)
	}

	return client, err
}

func NewRepClient(clientFactory rep.ClientFactory, address, url string) (rep.Client, error) {
	traceID := ""
	return clientFactory.CreateClient(address, traceID, url)
}

func NewLocketClient(logger lager.Logger, cmd *cobra.Command, locketClientConfig TLSConfig) (locketmodels.LocketClient, error) {
	var err error
	var client locketmodels.LocketClient
	config := locket.ClientLocketConfig{
		LocketAddress:        locketClientConfig.LocketApiLocation,
		LocketCACertFile:     locketClientConfig.CACertFile,
		LocketClientCertFile: locketClientConfig.CertFile,
		LocketClientKeyFile:  locketClientConfig.KeyFile,
	}

	if locketClientConfig.SkipCertVerify {
		client, err = locket.NewClientSkipCertVerify(
			logger,
			config,
		)
	} else {
		client, err = locket.NewClient(
			logger,
			config,
		)
	}

	return client, err
}

func (config *TLSConfig) Merge(newConfig TLSConfig) {
	if newConfig.BBSUrl != "" {
		config.BBSUrl = newConfig.BBSUrl
	}
	if newConfig.LocketApiLocation != "" {
		config.LocketApiLocation = newConfig.LocketApiLocation
	}
	if newConfig.Timeout != 0 {
		config.Timeout = newConfig.Timeout
	}
	if newConfig.KeyFile != "" {
		config.KeyFile = newConfig.KeyFile
	}
	if newConfig.CACertFile != "" {
		config.CACertFile = newConfig.CACertFile
	}
	if newConfig.CertFile != "" {
		config.CertFile = newConfig.CertFile
	}
	config.SkipCertVerify = config.SkipCertVerify || newConfig.SkipCertVerify
}
