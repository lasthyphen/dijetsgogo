//go:build linux && amd64 && rocksdballowed
// +build linux,amd64,rocksdballowed

// ^ Only build this file if this computer runs Linux AND is AMD64 AND rocksdb is allowed
// Copyright (C) 2019-2021, Dijets Inc, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package manager

import (
	"path/filepath"
	"testing"

	"github.com/lasthyphen/dijetsgogo/database/rocksdb"
	"github.com/lasthyphen/dijetsgogo/utils/logging"
	"github.com/lasthyphen/dijetsgogo/version"
	"github.com/stretchr/testify/assert"
)

func TestNewSingleRocksDB(t *testing.T) {
	dir := t.TempDir()

	v1 := version.DefaultVersion1_0_0

	dbPath := filepath.Join(dir, v1.String())
	db, err := rocksdb.New(dbPath, nil, logging.NoLog{})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	manager, err := NewRocksDB(dir, nil, logging.NoLog{}, v1)
	if err != nil {
		t.Fatal(err)
	}

	semDB := manager.Current()
	cmp := semDB.Version.Compare(v1)
	assert.Equal(t, 0, cmp, "incorrect version on current database")

	_, exists := manager.Previous()
	assert.False(t, exists, "there should be no previous database")

	dbs := manager.GetDatabases()
	assert.Len(t, dbs, 1)

	err = manager.Close()
	assert.NoError(t, err)
}
