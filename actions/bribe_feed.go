package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/bianyuanop/oraclevm/common"
	mconsts "github.com/bianyuanop/oraclevm/consts"
	"github.com/bianyuanop/oraclevm/storage"
)

const (
	BribeInsert = iota
	BribeDelete
)

const (
	BribeFeedComputeUnit = 1
)

var (
	ErrBribeFeedGreaterThanHighest = errors.New("bribing feedID is greater than highest")
	ErrBalanceBelowBribeAmount     = errors.New("balance less than bribing amount")
	ErrRoundPassed                 = errors.New("round already finalized")
	ErrBribeOpCodeNotExists        = errors.New("bribe opcode not exists")
)

var (
	_ chain.Action = (*BribeFeed)(nil)
)

// Note this action is only used for implementing attacks in place in e2e tests
// the action only works when there's only one bribe provider and withdraw is not implemented
type BribeFeed struct {
	FeedID    uint64            `serialize:"true" json:"feedID"`
	Recipient codec.Address     `serialize:"true" json:"recipient"`
	Round     uint64            `serialize:"true" json:"round"`
	OpCode    uint64            `serialize:"true" json:"opcode"`
	Info      *common.BribeInfo `serialize:"true" json:"info"`
}

func (*BribeFeed) GetTypeID() uint8 {
	return mconsts.BribeID
}

func (rf *BribeFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.FeedRoundKey(rf.FeedID)):                         state.Read,
		string(storage.FeedBribeKey(rf.FeedID, rf.Recipient, rf.Round)): state.All,
		string(storage.BalanceKey(actor)):                               state.Read | state.Write,
	}
}

func (rf *BribeFeed) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {

	switch rf.OpCode {
	case BribeInsert:
		roundInfo, err := storage.GetFeedRound(ctx, mu, rf.FeedID)
		if err != nil {
			return nil, err
		}

		if roundInfo.RoundNumber > rf.Round {
			return nil, ErrRoundPassed
		}

		if _, err := storage.SubBalance(ctx, mu, actor, rf.Info.Amount); err != nil {
			return nil, err
		}

		if err := storage.AddFeedBribe(ctx, mu, rf.FeedID, rf.Recipient, rf.Round, rf.Info); err != nil {
			return nil, err
		}
		return &BribeFeedResult{
			Opcode:    rf.OpCode,
			Round:     rf.Round,
			Amount:    rf.Info.Amount,
			Recipient: rf.Recipient,
		}, nil
	case BribeDelete:
		bribe, err := storage.RemoveFeedBribe(ctx, mu, rf.FeedID, rf.Recipient, rf.Round, actor)
		if err != nil {
			return nil, err
		}
		if _, err := storage.AddBalance(ctx, mu, actor, bribe.Amount); err != nil {
			return nil, err
		}
		return &BribeFeedResult{
			Opcode:    rf.OpCode,
			Round:     rf.Round,
			Amount:    bribe.Amount,
			Recipient: rf.Recipient,
		}, nil
	default:
		return nil, ErrBribeOpCodeNotExists
	}
}

func (*BribeFeed) ComputeUnits(chain.Rules) uint64 {
	return RegisterComputeUnit
}

func (*BribeFeed) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*BribeFeedResult)(nil)

type BribeFeedResult struct {
	Opcode    uint64        `serialize:"true" json:"opcode"`
	Round     uint64        `serialize:"true" json:"round"`
	Amount    uint64        `serialize:"true" json:"amount"`
	Recipient codec.Address `serialize:"true" json:"recipient"`
}

func (*BribeFeedResult) GetTypeID() uint8 {
	return mconsts.BribeID // Common practice is to use the action ID
}
