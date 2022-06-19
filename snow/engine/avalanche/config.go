// Copyright (C) 2019-2021, Dijets Inc, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package avalanche

import (
	"github.com/lasthyphen/dijetsgogo/snow"
	"github.com/lasthyphen/dijetsgogo/snow/consensus/avalanche"
	"github.com/lasthyphen/dijetsgogo/snow/engine/avalanche/vertex"
	"github.com/lasthyphen/dijetsgogo/snow/engine/common"
	"github.com/lasthyphen/dijetsgogo/snow/validators"
)

// Config wraps all the parameters needed for an avalanche engine
type Config struct {
	Ctx *snow.ConsensusContext
	common.AllGetsServer
	VM         vertex.DAGVM
	Manager    vertex.Manager
	Sender     common.Sender
	Validators validators.Set

	Params    avalanche.Parameters
	Consensus avalanche.Consensus
}
