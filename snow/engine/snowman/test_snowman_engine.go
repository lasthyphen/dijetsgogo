// Copyright (C) 2019-2021, Dijets Inc, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snowman

import (
	"errors"

	"github.com/lasthyphen/dijetsgogo/ids"
	"github.com/lasthyphen/dijetsgogo/snow/consensus/snowman"
	"github.com/lasthyphen/dijetsgogo/snow/engine/common"
)

var (
	_ Engine = &EngineTest{}

	errGetBlock = errors.New("unexpectedly called GetBlock")
)

// EngineTest is a test engine
type EngineTest struct {
	common.EngineTest

	CantGetBlock bool
	GetBlockF    func(ids.ID) (snowman.Block, error)
}

func (e *EngineTest) Default(cant bool) {
	e.EngineTest.Default(cant)
	e.CantGetBlock = false
}

func (e *EngineTest) GetBlock(blkID ids.ID) (snowman.Block, error) {
	if e.GetBlockF != nil {
		return e.GetBlockF(blkID)
	}
	if e.CantGetBlock && e.T != nil {
		e.T.Fatalf("Unexpectedly called GetBlock")
	}
	return nil, errGetBlock
}
