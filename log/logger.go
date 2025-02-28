//  Copyright 2019 The bigfile Authors. All rights reserved.
//  Use of this source code is governed by a MIT-style
//  license that can be found in the LICENSE file.

// Package log is used to collect information of running application
// and then transport them to destination.
package log

import (
	"os"
	"strings"
	"sync"

	"github.com/bigfile/bigfile/config"
	"github.com/op/go-logging"
)

var (
	log  *logging.Logger
	once sync.Once
)

// NewLogger is used to get a log collector, there is only a single
// instance globally
func NewLogger(logConfig *config.Log) (*logging.Logger, error) {
	var err error

	once.Do(func() {
		if logConfig == nil {
			logConfig = &config.DefaultConfig.Log
		}

		var (
			ok                  bool
			module              = "bigfile"
			level               logging.Level
			backend             []logging.Backend
			consoleBackend      logging.Backend
			fileBackend         logging.Backend
			fileHandler         *AutoRotateWriter
			consoleLevelBackend logging.LeveledBackend
			fileLevelBackend    logging.LeveledBackend
			leveledBackend      logging.LeveledBackend
		)

		log = logging.MustGetLogger(module)

		// output log to stdout
		if logConfig.Console.Enable {
			consoleBackend = logging.NewBackendFormatter(
				logging.NewLogBackend(os.Stdout, "", 0),
				logging.MustStringFormatter(logConfig.Console.Format),
			)
			consoleLevelBackend = logging.AddModuleLevel(consoleBackend)
			if level, ok = config.NameToLevel[strings.ToUpper(logConfig.Console.Level)]; !ok {
				level = logging.DEBUG
			}
			consoleLevelBackend.SetLevel(level, module)
			backend = append(backend, consoleLevelBackend)
		}

		// file output handler
		if logConfig.File.Enable {
			fileHandler, err = NewAutoRotateWriter(logConfig.File.Path, logConfig.File.MaxBytesPerFile)
			if err != nil {
				return
			}
			fileBackend = logging.NewBackendFormatter(
				logging.NewLogBackend(fileHandler, "", 0),
				logging.MustStringFormatter(logConfig.File.Format),
			)
			fileLevelBackend = logging.AddModuleLevel(fileBackend)
			if level, ok = config.NameToLevel[strings.ToUpper(logConfig.File.Level)]; !ok {
				level = logging.WARNING
			}
			fileLevelBackend.SetLevel(level, module)
			backend = append(backend, fileLevelBackend)
		}

		leveledBackend = logging.MultiLogger(backend...)
		log.SetBackend(leveledBackend)
	})

	return log, err
}

// MustNewLogger just call NewLogger, but if there are errors, process will
func MustNewLogger(logConfig *config.Log) *logging.Logger {
	var (
		logger *logging.Logger
		err    error
	)
	if logger, err = NewLogger(logConfig); err != nil {
		panic(err)
	}
	return logger
}
