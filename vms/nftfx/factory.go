// Copyright (C) 2019-2021, Dijets Inc, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package nftfx

import (
	"github.com/lasthyphen/dijetsgogo/ids"
	"github.com/lasthyphen/dijetsgogo/snow"
)

// ID that this Fx uses when labeled
var (
	ID = ids.ID{'n', 'f', 't', 'f', 'x'}
)

type Factory struct{}

func (f *Factory) New(*snow.Context) (interface{}, error) { return &Fx{}, nil }
