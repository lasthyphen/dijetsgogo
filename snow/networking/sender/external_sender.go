// Copyright (C) 2019-2021, Dijets Inc, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package sender

import (
	"github.com/lasthyphen/dijetsgogo/ids"
	"github.com/lasthyphen/dijetsgogo/message"
)

// ExternalSender sends consensus messages to other validators
// Right now this is implemented in the networking package
type ExternalSender interface {

	// Send a message to a specific set of nodes
	Send(
		msg message.OutboundMessage,
		nodeIDs ids.ShortSet,
		subnetID ids.ID,
		validatorOnly bool,
	) ids.ShortSet

	// Send a message to a random group of nodes in a subnet.
	// Nodes are sampled based on their validator status.
	Gossip(
		msg message.OutboundMessage,
		subnetID ids.ID,
		validatorOnly bool,
		numValidatorsToSend int,
		numNonValidatorsToSend int,
	) ids.ShortSet
}
