// Copyright (C) 2019-2021, Dijets Inc, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package djtx

import (
	"errors"

	"github.com/lasthyphen/dijetsgogo/ids"
	"github.com/lasthyphen/dijetsgogo/vms/components/verify"
)

var (
	errNilAssetID   = errors.New("nil asset ID is not valid")
	errEmptyAssetID = errors.New("empty asset ID is not valid")

	_ verify.Verifiable = &Asset{}
)

type Asset struct {
	ID ids.ID `serialize:"true" json:"assetID"`
}

// AssetID returns the ID of the contained asset
func (asset *Asset) AssetID() ids.ID { return asset.ID }

func (asset *Asset) Verify() error {
	switch {
	case asset == nil:
		return errNilAssetID
	case asset.ID == ids.Empty:
		return errEmptyAssetID
	default:
		return nil
	}
}
