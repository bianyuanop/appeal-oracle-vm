package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	mconsts "github.com/ava-labs/hypersdk-starter-kit/consts"
	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

const (
	DepositComputeUnit = 1
)

var (
	ErrDepositFeedGreaterThanHighest = errors.New("depositing feedID is greater than highest")
	ErrBalanceBelowDepositAmount     = errors.New("balance less than deposit amount")
)

var (
	_ chain.Action = (*DepositFeed)(nil)
)

// A fee need to be paid for the feed registration
type DepositFeed struct {
	FeedID uint64 `serialize:"true" json:"feedID"`
	Amount uint64 `serialize:"true" json:"amount"`
}

func (*DepositFeed) GetTypeID() uint8 {
	return mconsts.DepositFeedID
}

func (rf *DepositFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.FeedKey(rf.FeedID)):               state.Read,
		string(storage.FeedDepositKey(rf.FeedID, actor)): state.All,
		string(storage.BalanceKey(actor)):                state.Read | state.Write,
	}
}

func (rf *DepositFeed) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	highestFeedID, err := storage.GetHighestFeedID(ctx, mu)
	if err != nil {
		return nil, err
	}

	if rf.FeedID > highestFeedID {
		return nil, ErrDepositFeedGreaterThanHighest
	}

	balance, err := storage.GetBalance(ctx, mu, actor)
	if err != nil {
		return nil, err
	}

	if balance < rf.Amount {
		return nil, ErrBalanceBelowDepositAmount
	}

	// deduct amount and set deposit storage
	balance -= rf.Amount
	if err := storage.SetBalance(ctx, mu, actor, balance); err != nil {
		return nil, err
	}
	if err := storage.SetFeedDeposit(ctx, mu, rf.FeedID, actor, rf.Amount); err != nil {
		return nil, err
	}

	return &DepositFeedResult{
		FeedID:  rf.FeedID,
		Amount:  rf.Amount,
		Address: actor,
	}, nil
}

func (*DepositFeed) ComputeUnits(chain.Rules) uint64 {
	return RegisterComputeUnit
}

func (*DepositFeed) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*DepositFeedResult)(nil)

type DepositFeedResult struct {
	FeedID  uint64        `serialize:"true" json:"feedID"`
	Amount  uint64        `serialize:"true" json:"amount"`
	Address codec.Address `serialize:"true" json:"address"`
}

func (*DepositFeedResult) GetTypeID() uint8 {
	return mconsts.DepositFeedID // Common practice is to use the action ID
}

// TODO: to be implemented
func (*DepositFeed) Marshal() ([]byte, error) {
	return nil, nil
}
