package driver

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func OpenDB(connectionString string) (*sql.DB, error) {
	db, err := sql.Open("mysql", connectionString)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return db, nil
}
