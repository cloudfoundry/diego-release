package db

//go:generate counterfeiter -generate

//counterfeiter:generate . DB

type DB interface {
	DomainDB
	EncryptionDB
	EvacuationDB
	LRPDB
	TaskDB
	VersionDB
	SuspectDB
	BBSHealthCheckDB
}
