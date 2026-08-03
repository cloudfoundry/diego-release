package encryptor

import (
	"context"
	"errors"
	"os"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
)

const (
	encryptionDuration = "EncryptionDuration"
)

type Encryptor struct {
	logger       lager.Logger
	db           db.EncryptionDB
	keyManager   encryption.KeyManager
	cryptor      encryption.Cryptor
	clock        clock.Clock
	metronClient loggingclient.IngressClient
}

func New(
	logger lager.Logger,
	db db.EncryptionDB,
	keyManager encryption.KeyManager,
	cryptor encryption.Cryptor,
	clock clock.Clock,
	metronClient loggingclient.IngressClient,
) Encryptor {
	return Encryptor{
		logger:       logger,
		db:           db,
		keyManager:   keyManager,
		cryptor:      cryptor,
		clock:        clock,
		metronClient: metronClient,
	}
}

func (m Encryptor) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := m.logger.Session("encryptor")
	logger.Info("starting")
	defer logger.Info("exited")

	currentEncryptionKey, err := m.db.EncryptionKeyLabel(context.Background(), logger)
	if err != nil {
		if models.ConvertError(err) != models.ErrResourceNotFound {
			logger.Error("failed-to-fetch-encryption-key-label", err)
			return err
		}
	} else {
		if m.keyManager.DecryptionKey(currentEncryptionKey) == nil {
			err := errors.New("Existing encryption key version (" + currentEncryptionKey + ") is not among the known keys")
			logger.Error("unknown-encryption-key-lable", err)
			return err
		}
	}

	close(ready)

	if currentEncryptionKey != m.keyManager.EncryptionKey().Label() {
		logger := logger.WithData(lager.Data{
			"desired-key-label":  m.keyManager.EncryptionKey().Label(),
			"existing-key-label": currentEncryptionKey,
		})

		encryptionStart := m.clock.Now()
		logger.Info("encryption-started")
		err := m.db.PerformEncryption(context.Background(), logger)
		if err != nil {
			logger.Error("encryption-failed", err)
		} else {
			err = m.db.SetEncryptionKeyLabel(context.Background(), logger, m.keyManager.EncryptionKey().Label())
			if err != nil {
				return err
			}
		}

		totalTime := m.clock.Since(encryptionStart)
		logger.Info("encryption-finished", lager.Data{"total_time": totalTime})
		err = m.metronClient.SendDuration(encryptionDuration, totalTime)
		if err != nil {
			logger.Error("failed-to-send-encryption-duration-metrics", err)
		}
	}

	<-signals
	return nil
}
