package db

import (
	"crypto/tls"

	"github.com/go-sql-driver/mysql"
)

type MySQLAdapter struct{}

func (m *MySQLAdapter) RegisterTLSConfig(key string, config *tls.Config) error {
	return mysql.RegisterTLSConfig(key, config)
}
