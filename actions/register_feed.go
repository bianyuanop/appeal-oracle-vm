package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk-starter-kit/consts"
)

const (
	RegisterComputeUnit = 1
	FeedMaxMemoSize     = 1024
)

var (
	ErrRequestedFeedIDNotLatest              = errors.New("requested feed ID not the latest")
	ErrFeedMemoTooLarge                      = errors.New("feed memo size too large")
	_                           chain.Action = (*RegisterFeed)(nil)
)

// A fee need to be paid for the feed registration
type RegisterFeed struct {
	FeedID         uint64 `serialize:"true" json:"feedID"`
	FeedName       string `serialize:"true" json:"feedName"`
	MinDeposit     uint64 `serialize:"true" json:"minDeposit"`
	AppealEffect   uint64 `serialize:"true" json:"appealEffect"`
	AppealMaxDelay uint64 `serialize:"true" json:"appealMaxDelay"`
	// the WASM for feed reports aggregation, this program can tell us the ones locate in 75% quantiles or within 3 sigma and out of that
	AggretatorWASM []byte `serialize:"true" json:"aggregatorWASM"`
	// Optional message to accompany transaction.
	Memo []byte `serialize:"true" json:"memo"`
}

func (*RegisterFeed) GetTypeID() uint8 {
	return mconsts.RegisterFeedID
}

func (rf *RegisterFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.FeedIDKey()):        state.All,
		string(storage.FeedKey(rf.FeedID)): state.All,
	}
}

func (rf *RegisterFeed) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	highestFeedID, err := storage.GetHighestFeedID(ctx, mu)
	if err != nil {
		return nil, err
	}
	if len(rf.Memo) > FeedMaxMemoSize {
		return nil, ErrFeedMemoTooLarge
	}

	if rf.FeedID != highestFeedID {
		return nil, ErrRequestedFeedIDNotLatest
	}

	rfRaw, err := rf.Serialize()
	if err != nil {
		return nil, err
	}

	// set up feed and increment highest feed id
	if err := storage.SetFeed(ctx, mu, rf.FeedID, rfRaw); err != nil {
		return nil, err
	}
	if err := storage.IncrementFeedID(ctx, mu); err != nil {
		return nil, err
	}

	return &RegisterFeedResult{
		FeedID: rf.FeedID,
	}, nil
}

func (*RegisterFeed) ComputeUnits(chain.Rules) uint64 {
	return RegisterComputeUnit
}

func (*RegisterFeed) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*RegisterFeedResult)(nil)

type RegisterFeedResult struct {
	FeedID uint64 `serialize:"true" json:"feedID"`
}

func (*RegisterFeedResult) GetTypeID() uint8 {
	return mconsts.RegisterFeedID // Common practice is to use the action ID
}

// TODO: to be implemented
func (*RegisterFeed) Serialize() ([]byte, error) {
	return nil, nil
}
