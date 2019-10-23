// Package api implements the API between Oasis ABCI application and Oasis core.
package api

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/abci/types"
	tmcmn "github.com/tendermint/tendermint/libs/common"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	tmp2p "github.com/tendermint/tendermint/p2p"

	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	"github.com/oasislabs/oasis-core/go/common/node"
	"github.com/oasislabs/oasis-core/go/tendermint/crypto"
)

// Conesnus Backend Name
const BackendName = "tendermint"

// Code is a status code for ABCI requests.
type Code uint32

// Status codes for the various ABCI requests.
const (
	CodeOK                 Code = Code(types.CodeTypeOK) // uint32(0)
	CodeInvalidApplication Code = Code(1)
	CodeNoCommittedBlocks  Code = Code(2)
	CodeInvalidFormat      Code = Code(3)
	CodeTransactionFailed  Code = Code(4)
	CodeInvalidQuery       Code = Code(5)
	CodeNotFound           Code = Code(6)
)

// ToInt returns an integer representation of the status code.
func (c Code) ToInt() uint32 {
	return uint32(c)
}

// String returns a string representation of the status code.
func (c Code) String() string {
	switch c {
	case CodeOK:
		return "ok"
	case CodeInvalidApplication:
		return "invalid application"
	case CodeNoCommittedBlocks:
		return "no committed blocks"
	case CodeInvalidFormat:
		return "invalid format"
	case CodeTransactionFailed:
		return "transaction failed"
	case CodeInvalidQuery:
		return "invalid query"
	case CodeNotFound:
		return "not found"
	default:
		return "unknown"
	}
}

var tagAppNameValue = []byte("1")

// VotingPower is the default voting power for all validator nodes.
const VotingPower = 1

// PublicKeyToValidatorUpdate converts an Oasis node public key to a
// tendermint validator update.
func PublicKeyToValidatorUpdate(id signature.PublicKey, power int64) types.ValidatorUpdate {
	pk, _ := id.MarshalBinary()

	return types.ValidatorUpdate{
		PubKey: types.PubKey{
			Type: types.PubKeyEd25519,
			Data: pk,
		},
		Power: power,
	}
}

// NodeToP2PAddr converts an Oasis node descriptor to a tendermint p2p
// address book entry.
func NodeToP2PAddr(n *node.Node) (*tmp2p.NetAddress, error) {
	// WARNING: p2p/transport.go:MultiplexTransport.upgrade() uses
	// a case senstive string comparsison to validate public keys,
	// because tendermint.

	if !n.HasRoles(node.RoleValidator) {
		return nil, fmt.Errorf("tendermint/api: node is not a validator")
	}

	pubKey := crypto.PublicKeyToTendermint(&n.ID)
	pubKeyAddrHex := strings.ToLower(pubKey.Address().String())

	if len(n.Consensus.Addresses) == 0 {
		// Should never happen, but check anyway.
		return nil, fmt.Errorf("tendermint/api: node has no consensus addresses")
	}
	coreAddress, _ := n.Consensus.Addresses[0].MarshalText()

	addr := pubKeyAddrHex + "@" + string(coreAddress)

	tmAddr, err := tmp2p.NewNetAddressString(addr)
	if err != nil {
		return nil, errors.Wrap(err, "tenderimt/api: failed to reformat validator")
	}

	return tmAddr, nil
}

const eventTypeOasis = "oasis"

// EventBuilder is a helper for constructing ABCI events.
type EventBuilder struct {
	app []byte
	ev  types.Event
}

// Attribute appends a key/value pair to the event.
func (bld *EventBuilder) Attribute(key, value []byte) *EventBuilder {
	bld.ev.Attributes = append(bld.ev.Attributes, tmcmn.KVPair{
		Key:   key,
		Value: value,
	})

	return bld
}

// Dirty returns true iff the EventBuilder has attributes.
func (bld *EventBuilder) Dirty() bool {
	return len(bld.ev.Attributes) > 0
}

// Event returns the event from the EventBuilder.
func (bld *EventBuilder) Event() types.Event {
	// Return a copy to support emitting incrementally.
	ev := types.Event{
		Type: bld.ev.Type,
		Attributes: []tmcmn.KVPair{
			tmcmn.KVPair{
				Key:   []byte("updated"),
				Value: tagAppNameValue,
			},
		},
	}
	ev.Attributes = append(ev.Attributes, bld.ev.Attributes...)

	return ev
}

// NewEventBuilder returns a new EventBuilder for the given ABCI app.
func NewEventBuilder(app string) *EventBuilder {
	return &EventBuilder{
		app: []byte(app),
		ev: types.Event{
			Type: EventTypeForApp(app),
		},
	}
}

// EventTypeForApp generates the ABCI event type for events belonging
// to the specified App.
func EventTypeForApp(eventApp string) string {
	return eventTypeOasis + "." + eventApp
}

// QueryForApp generates a tmquery.Query for events belonging to the
// specified App.
func QueryForApp(eventApp string) tmpubsub.Query {
	return tmquery.MustParse(fmt.Sprintf("%s.updated='%s'", EventTypeForApp(eventApp), tagAppNameValue))
}
