package opao

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql" // MySQL database driver
	//_ "github.com/lib/pq"			// PostgreSQL database driver
	//_ "github.com/mattn/go-sqlite3"  // SQLite3 database driver
	"reflect"
	"sync"
)

// Database ORM Object
type Database struct {
	Db     *sql.DB                 // Database connection object
	Caches map[reflect.Type]*Cache // Mapping of object types to caches
	RWLock sync.RWMutex            // Read-write lock
}

// NewDatabase creates and initializes a new Database instance
func NewDatabase(driverName, dataSourceName string) (*Database, error) {
	var Db, err = sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	return &Database{Db: Db, Caches: map[reflect.Type]*Cache{}}, nil
}
