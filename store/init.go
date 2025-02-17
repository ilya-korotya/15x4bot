package store

import (
	"database/sql"
	"github.com/alexkarlov/15x4bot/config"
)

var dbConn *sql.DB

// Conf is a DB configuration
var Conf config.DB

// Init initializes db connection, pings the server and saves it to the dbConn
func Init() error {
	var err error
	dbConn, err = sql.Open("postgres", Conf.DSN)
	if err != nil {
		return err
	}
	err = dbConn.Ping()
	if err != nil {
		return err
	}
	return nil
}
