// Copyright (C) 2019-2021, Dijets Inc, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snowstorm

import (
	"github.com/lasthyphen/dijetsgogo/ids"
	"github.com/lasthyphen/dijetsgogo/snow/events"
	"github.com/lasthyphen/dijetsgogo/utils/wrappers"
)

var _ events.Blockable = &rejector{}

type rejector struct {
	g        *Directed
	errs     *wrappers.Errs
	deps     ids.Set
	rejected bool // true if the tx has been rejected
	txID     ids.ID
}

func (r *rejector) Dependencies() ids.Set { return r.deps }

func (r *rejector) Fulfill(ids.ID) {
	if r.rejected || r.errs.Errored() {
		return
	}
	r.rejected = true
	asSet := ids.NewSet(1)
	asSet.Add(r.txID)
	r.errs.Add(r.g.reject(asSet))
}

func (*rejector) Abandon(ids.ID) {}
func (*rejector) Update()        {}
