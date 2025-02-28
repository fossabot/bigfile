//  Copyright 2019 The bigfile Authors. All rights reserved.
//  Use of this source code is governed by a MIT-style
//  license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bigfile/bigfile/config"
	"github.com/bigfile/bigfile/internal/util"
	"github.com/jinzhu/gorm"
)

// ChunkSize represent chunk size, default: 1MB
const ChunkSize = 1 << 20

var (
	// ErrInvalidChunkID when try to get the path of chunk, if the id of chunk
	// is less than 10000, this error will be panic
	ErrInvalidChunkID = errors.New("invalid chunk id")
	// ErrChunkExceedLimit the size of chunk is limited, if the content that is
	// more than ChunkSize is appended to chunk, it will panic this error
	ErrChunkExceedLimit = fmt.Errorf("total length exceed limit: %d bytes", ChunkSize)
)

// Chunk represents every chunk of file
type Chunk struct {
	ID        uint64    `gorm:"type:BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT;primary_key"`
	Size      int       `gorm:"type:int;column:size"`
	Hash      string    `gorm:"type:CHAR(64) NOT NULL;UNIQUE;column:hash"`
	CreatedAt time.Time `gorm:"type:TIMESTAMP(6) NOT NULL;DEFAULT:CURRENT_TIMESTAMP(6);column:createdAt"`
	UpdatedAt time.Time `gorm:"type:TIMESTAMP(6) NOT NULL;DEFAULT:CURRENT_TIMESTAMP(6);column:updatedAt"`
}

// TableName represent table name
func (c Chunk) TableName() string {
	return "chunks"
}

// Reader return a reader with buffer
func (c *Chunk) Reader(rootPath *string) (*os.File, error) {
	var (
		err  error
		file *os.File
	)
	if file, err = os.Open(c.Path(rootPath)); err != nil {
		return nil, err
	}
	return file, nil
}

// Path represent the actual storage path
func (c Chunk) Path(rootPath *string) string {

	if rootPath == nil {
		rootPath = &config.DefaultConfig.Chunk.RootPath
	}
	if c.ID < 10000 {
		panic(ErrInvalidChunkID)
	}
	idStr := strconv.FormatUint(c.ID, 10)
	parts := make([]string, (len(idStr)/3)+1)
	index := 0
	for ; len(idStr) > 3; index++ {
		parts[index] = util.SubStrFromToEnd(idStr, -3)
		idStr = util.SubStrFromTo(idStr, 0, -3)
	}
	parts[index] = idStr
	parts = parts[1:]
	util.ReverseSlice(parts)
	dir := fmt.Sprintf("%s/%s", strings.TrimSuffix(*rootPath, "/"), filepath.Join(parts...))
	if !util.IsDir(dir) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			panic(err)
		}
	}
	return fmt.Sprintf("%s/%s", dir, strconv.FormatUint(c.ID, 10))
}

// AppendBytes is used to append bytes to chunk. Firstly, this function will check whether
// there is already a chunk its hash value is equal to the hash of complete content. If exist,
// return it, otherwise, append content to origin chunk.
func (c *Chunk) AppendBytes(p []byte, rootPath *string, db *gorm.DB) (*Chunk, int, error) {
	var (
		file       *os.File
		err        error
		writeCount int
		buf        bytes.Buffer
		oldContent []byte
		hash       string
	)

	if len(p) > ChunkSize-c.Size {
		panic(ErrChunkExceedLimit)
	}

	if oldContent, err = ioutil.ReadFile(c.Path(rootPath)); err != nil {
		return nil, 0, err
	}
	buf.Write(oldContent)
	buf.Write(p)

	// calculate the hash value of complete content
	if hash, err = util.Sha256Hash2String(buf.Bytes()); err != nil {
		return nil, 0, err
	}

	// find chunk by the hash value of complete content
	if chunk, err := FindChunkByHash(hash, db); err == nil && chunk.ID > 0 && util.IsFile(chunk.Path(rootPath)) {
		return chunk, len(p), err
	}

	// if the current chunk is referenced by other objects, it should be copied and appended
	if count, err := CountObjectChunkByChunkID(c.ID, db); err != nil {
		return nil, 0, err
	} else if count > 1 {
		newChunk, err := CreateChunkFromBytes(buf.Bytes(), rootPath, db)
		if err != nil {
			return nil, 0, err
		}
		return newChunk, len(p), nil
	}

	c.Size = buf.Len()
	c.Hash = hash

	if file, err = os.OpenFile(c.Path(rootPath), os.O_APPEND|os.O_WRONLY, 0644); err != nil {
		return nil, 0, err
	}
	defer file.Close()

	if writeCount, err = file.Write(p); err != nil {
		return nil, 0, err
	}

	if err = db.Save(c).Error; err != nil {
		return nil, 0, err
	}

	return c, writeCount, err
}

// CreateChunkFromBytes will crate a chunk from the specify byte content
func CreateChunkFromBytes(p []byte, rootPath *string, db *gorm.DB) (*Chunk, error) {
	var (
		chunk   *Chunk
		err     error
		hashStr string
		size    int
	)

	if size = len(p); int64(size) > ChunkSize {
		return nil, fmt.Errorf("the size of chunk must be less than %d bytes", ChunkSize)
	}

	if hashStr, err = util.Sha256Hash2String(p); err != nil {
		return nil, err
	}

	if chunk, err = FindChunkByHash(hashStr, db); err == nil && chunk.ID > 0 && util.IsFile(chunk.Path(rootPath)) {
		return chunk, err
	}

	chunk = &Chunk{
		Size: size,
		Hash: hashStr,
	}
	if err = db.Create(chunk).Error; err != nil {
		return nil, err
	}

	if err = ioutil.WriteFile(chunk.Path(rootPath), p, 0644); err != nil {
		return nil, err
	}

	return chunk, nil
}

// FindChunkByHash will find chunk by the specify hash
func FindChunkByHash(h string, db *gorm.DB) (*Chunk, error) {
	var chunk Chunk
	var err = db.Where("hash = ?", h).First(&chunk).Error
	return &chunk, err
}

// CreateEmptyContentChunk is used to create a chunk with empty content
func CreateEmptyContentChunk(rootPath *string, db *gorm.DB) (*Chunk, error) {
	var (
		chunk            *Chunk
		err              error
		emptyContentHash string
	)
	if emptyContentHash, err = util.Sha256Hash2String(nil); err != nil {
		return nil, err
	}

	if chunk, err = FindChunkByHash(emptyContentHash, db); err == nil && chunk.ID > 0 && util.IsFile(chunk.Path(rootPath)) {
		return chunk, err
	}

	chunk = &Chunk{
		Size: 0,
		Hash: emptyContentHash,
	}

	if err = db.Create(chunk).Error; err != nil {
		return nil, err
	}

	if err = ioutil.WriteFile(chunk.Path(rootPath), nil, 0644); err != nil {
		return nil, err
	}

	return chunk, nil
}
