// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bootstrap

import (
	"github.com/lasthyphen/dijetsgogo/snow/engine/avalanche/vertex"
	"github.com/lasthyphen/dijetsgogo/snow/engine/common"
	"github.com/lasthyphen/dijetsgogo/snow/engine/common/queue"
	"github.com/lasthyphen/dijetsgogo/snow/engine/common/tracker"
)

type Config struct {
	common.Config
	common.AllGetsServer

	// VtxBlocked tracks operations that are blocked on vertices
	VtxBlocked *queue.JobsWithMissing
	// TxBlocked tracks operations that are blocked on transactions
	TxBlocked *queue.Jobs

	Manager       vertex.Manager
	VM            vertex.DAGVM
	WeightTracker tracker.WeightTracker
}
