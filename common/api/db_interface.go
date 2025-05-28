package common

import "gorm.io/gorm"

// DBTX defines the interface for database operations that handlers will use.
// This allows for mocking the database in tests.
type DBTX interface {
	Find(dest interface{}, conds ...interface{}) *gorm.DB
	Create(value interface{}) *gorm.DB
	Save(value interface{}) *gorm.DB
	Delete(value interface{}, conds ...interface{}) *gorm.DB
	First(dest interface{}, conds ...interface{}) *gorm.DB
	Where(query interface{}, args ...interface{}) *gorm.DB
	Preload(query string, args ...interface{}) *gorm.DB
	Model(value interface{}) *gorm.DB
	// Association(column string) *gorm.Association // gorm.Association is a struct, tricky for simple interface return type matching if methods on it are chained.
	// For now, let's assume handlers get the association from Model().Association() and then call methods on the returned *gorm.Association.
	// Or, we might need to mock *gorm.Association itself if its methods need to be controlled.

	// Add other GORM methods used by your handlers here...
}
