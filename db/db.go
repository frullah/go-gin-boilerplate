package db

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/frullah/gin-boilerplate/config"
	"github.com/frullah/gin-boilerplate/models"
	"github.com/jinzhu/gorm"
)

// Instance code databases
type Instance byte

// DB instance code
const (
	Default Instance = iota
	InstanceCap
)

var dbInstanceMap = map[string]Instance{
	"default": Default,
}

var (
	db          [InstanceCap]*gorm.DB
	initialized = false
)

// Init database
func Init() error {
	if initialized {
		return nil
	}
	initialized = true

	cnf := config.Get()
	for _, dbInstanceConf := range cnf.DB {
		dbInstance, err := gorm.Open(dbInstanceConf.Type, dbInstanceConf.DSN)
		if err != nil {
			return err
		}
		dbInstance.SingularTable(true)
		dbInstance.LogMode(dbInstanceConf.Logging)
		db[dbInstanceMap[dbInstanceConf.Name]] = dbInstance
	}

	userModel := new(models.User)
	userRoleModel := new(models.UserRole)

	defaultDB := db[Default]
	defaultDB.AutoMigrate(userRoleModel)
	defaultDB.AutoMigrate(userModel).
		AddIndex("role_id__index", "role_id").
		AddForeignKey("role_id", "user_role(id)", "CASCADE", "CASCADE")

	return nil
}

// Get DB
func Get(instance Instance) *gorm.DB {
	return db[instance]
}

// Close all databases connectioon
func Close() {
	for _, dbInstance := range db {
		dbInstance.Close()
	}
}

// SetupTest database
func SetupTest(instance Instance) (sqlmock.Sqlmock, func() error) {
	dbMock, sqlMock, _ := sqlmock.New()
	db[instance], _ = gorm.Open("sqlite3", dbMock)
	db[instance].SingularTable(true)
	db[instance].LogMode(true)
	return sqlMock, db[instance].Close
}
