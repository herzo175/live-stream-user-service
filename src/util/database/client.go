package database

import (
	"fmt"
	"log"
	"os"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

type SQLDatabaseClient struct {
	DB *gorm.DB
}

type Database interface {
	FindOne(schema interface{}, query interface{}, args ...interface{}) error
	FindMany(schemas interface{}, start, stop int, query interface{}, args ...interface{}) (int, error)
	FindAll(schemas interface{}, start, stop int) (int, error)
	Create(schema interface{}) error
	Update(schema interface{}, update map[string]interface{}, query interface{}, args ...interface{}) error
	Delete(schema interface{}, query interface{}, args ...interface{}) error
}

func MakeClient() *SQLDatabaseClient {
	// TODO: user for DB(s)
	db, err := gorm.Open("postgres", os.Getenv("DB_CONNECTION_CONFIG"))

	db.LogMode(true)

	if err != nil {
		log.Fatal(err)
	}

	return &SQLDatabaseClient{DB: db}
}

// schema must be a pointer to a schema
func (config *SQLDatabaseClient) FindOne(schema interface{}, query interface{}, args ...interface{}) error {
	// return config.DB.Where(query, args).First(schema).Error
	return config.DB.Where(query, args...).First(schema).Error
}

// schemas must be a pointer to a list of schemas
func (config *SQLDatabaseClient) FindMany(
	schemas interface{},
	start, stop int,
	query interface{},
	args ...interface{},
) (int, error) {
	// NOTE: return paginated list by default?
	var count int
	sqlQuery := config.DB.Offset(start).Limit(stop).Where(query, args...).Find(schemas).Count(&count)

	return count, sqlQuery.Error
}

func (config *SQLDatabaseClient) FindAll(schemas interface{}, start, stop int) (int, error) {
	// NOTE: return paginated list by default?
	var count int
	sqlQuery := config.DB.Offset(start).Limit(stop).Find(schemas).Count(&count)

	return count, sqlQuery.Error
}

func (config *SQLDatabaseClient) Create(schema interface{}) error {
	if config.DB.NewRecord(schema) {
		err := config.DB.Create(schema).Error

		if config.DB.NewRecord(schema) {
			return nil
		} else {
			return fmt.Errorf("Item failed to insert into DB: %v", err)
		}
	} else {
		return fmt.Errorf("Schema has an item already for that primary key: %v", schema)
	}
}

func (config *SQLDatabaseClient) Update(
	schema interface{},
	update map[string]interface{},
	query interface{},
	args ...interface{},
) error {
	return config.DB.Model(schema).Where(query, args...).Updates(update).Error
}

func (config *SQLDatabaseClient) Delete(schema interface{}, query interface{}, args ...interface{}) error {
	return config.DB.Where(query, args...).Delete(schema).Error
}
