package db

import (
	"database/sql"

	"gorm.io/gorm"
)

//go:generate counterfeiter -o fakes/fake_client.go . Client
type Client interface {
	Close() error
	Where(query interface{}, args ...interface{}) Client
	Create(value interface{}) (int64, error)
	Delete(value interface{}, where ...interface{}) (int64, error)
	Save(value interface{}) (int64, error)
	Update(column string, value interface{}) (int64, error)
	First(out interface{}, where ...interface{}) error
	Find(out interface{}, where ...interface{}) error
	AutoMigrate(values ...interface{}) error
	Begin() Client
	Rollback() error
	Commit() error
	HasTable(value interface{}) bool
	AddUniqueIndex(indexName string, columns interface{}) error
	RemoveIndex(indexName string, columns interface{}) error
	Model(value interface{}) Client
	Exec(query string, args ...interface{}) int64
	ExecWithError(query string, args ...interface{}) error
	Rows(tableName string) (*sql.Rows, error)
	DropColumn(column string) error
	Dialect() gorm.Dialector
	Migrator() gorm.Migrator
}

type gormClient struct {
	db *gorm.DB
}

func NewGormClient(db *gorm.DB) Client {
	return &gormClient{db: db}
}
func (c *gormClient) DropColumn(name string) error {
	return c.db.Migrator().DropColumn(c.db.Statement.Model, name)
}
func (c *gormClient) Close() error {
	sqlDB, err := c.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
func (c *gormClient) AddUniqueIndex(indexName string, columns interface{}) error {
	return c.db.Migrator().CreateIndex(columns, indexName)
}

func (c *gormClient) Dialect() gorm.Dialector {
	return c.db.Dialector
}

func (c *gormClient) RemoveIndex(indexName string, columns interface{}) error {
	return c.db.Migrator().DropIndex(columns, indexName)
}

func (c *gormClient) Model(value interface{}) Client {
	var newClient gormClient
	newClient.db = c.db.Model(value)
	return &newClient
}
func (c *gormClient) Where(query interface{}, args ...interface{}) Client {
	var newClient gormClient
	newClient.db = c.db.Where(query, args...)
	return &newClient
}

func (c *gormClient) Create(value interface{}) (int64, error) {
	newDb := c.db.Create(value)
	return newDb.RowsAffected, newDb.Error
}

func (c *gormClient) Delete(value interface{}, where ...interface{}) (int64, error) {
	newDb := c.db.Delete(value, where...)
	return newDb.RowsAffected, newDb.Error
}

func (c *gormClient) Save(value interface{}) (int64, error) {
	newDb := c.db.Save(value)
	return newDb.RowsAffected, newDb.Error
}

func (c *gormClient) Update(column string, value interface{}) (int64, error) {
	newDb := c.db.Update(column, value)
	return newDb.RowsAffected, newDb.Error
}

func (c *gormClient) First(out interface{}, where ...interface{}) error {
	return c.db.First(out, where...).Error
}

func (c *gormClient) Find(out interface{}, where ...interface{}) error {
	return c.db.Find(out, where...).Error
}

func (c *gormClient) AutoMigrate(values ...interface{}) error {
	return c.db.AutoMigrate(values...)
}

func (c *gormClient) Begin() Client {
	var newClient gormClient
	newClient.db = c.db.Begin()
	return &newClient
}

func (c *gormClient) Rollback() error {
	return c.db.Rollback().Error
}

func (c *gormClient) Commit() error {
	return c.db.Commit().Error
}

func (c *gormClient) HasTable(value interface{}) bool {
	return c.db.Migrator().HasTable(value)
}

func (c *gormClient) Exec(query string, args ...interface{}) int64 {
	dbClient := c.db.Exec(query, args...)
	return dbClient.RowsAffected
}

func (c *gormClient) ExecWithError(query string, args ...interface{}) error {
	return c.db.Exec(query, args...).Error
}

func (c *gormClient) Migrator() gorm.Migrator {
	return c.db.Migrator()
}

func (c *gormClient) Rows(tablename string) (*sql.Rows, error) {
	tableDb := c.db.Table(tablename)
	return tableDb.Rows()
}
