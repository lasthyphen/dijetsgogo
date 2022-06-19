// Copyright (C) 2019-2021, Dijets Inc, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package poll

import (
	"fmt"

	"github.com/lasthyphen/dijetsgogo/ids"
	"github.com/lasthyphen/dijetsgogo/utils/formatting"
)

// Set is a collection of polls
type Set interface {
	fmt.Stringer

	Add(requestID uint32, vdrs ids.ShortBag) bool
	Vote(requestID uint32, vdr ids.ShortID, votes []ids.ID) []ids.UniqueBag
	Len() int
}

// Poll is an outstanding poll
type Poll interface {
	formatting.PrefixedStringer

	Vote(vdr ids.ShortID, votes []ids.ID)
	Finished() bool
	Result() ids.UniqueBag
}

// Factory creates a new Poll
type Factory interface {
	New(vdrs ids.ShortBag) Poll
}
