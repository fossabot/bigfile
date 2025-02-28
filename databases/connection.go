//  Copyright 2019 The bigfile Authors. All rights reserved.
//  Use of this source code is governed by a MIT-style
//  license that can be found in the LICENSE file.

// Package databases provides capacity to interact with database
package databases

import (
	"sync"

	"github.com/bigfile/bigfile/config"
	"github.com/jinzhu/gorm"

	// import mysql database driver
	_ "github.com/jinzhu/gorm/dialects/mysql"
	// import postgres database driver
	_ "github.com/jinzhu/gorm/dialects/postgres"
	// import sqlite database driver
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var (
	connection *gorm.DB
	once       sync.Once
)

// NewConnection will initialize a connection to database. But, if connection
// has already existed, it will be used.
func NewConnection(dbConfig *config.Database) (*gorm.DB, error) {
	var (
		err error
		dsn string
	)

	if dbConfig == nil {
		dbConfig = &config.DefaultConfig.Database
	}

	once.Do(func() {
		if dsn, err = dbConfig.DSN(); err != nil {
			return
		}
		if connection, err = gorm.Open(dbConfig.Driver, dsn); err != nil {
			return
		}
	})

	return connection, err
}

// MustNewConnection just call NewConnection, but, when something goes wrong, it will
// raise a panic
func MustNewConnection(dbConfig *config.Database) *gorm.DB {
	var (
		conn *gorm.DB
		err  error
	)
	if conn, err = NewConnection(dbConfig); err != nil {
		panic(err)
	}
	return conn
}
