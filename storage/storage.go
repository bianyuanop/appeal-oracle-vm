// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk-starter-kit/common"
	mconsts "github.com/ava-labs/hypersdk-starter-kit/consts"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

func innerGetValue(
	v []byte,
	err error,
) ([]byte, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

// State
// 0x0/ (hypersdk-height)
// 0x1/ (hypersdk-round)
// 0x2/ (hypersdk-fee)
//
// 0x3/ (balance)
//   -> [owner] => balance

const balancePrefix byte = metadata.DefaultMinimumPrefix

const BalanceChunks uint16 = 1

// [balancePrefix] + [address]
func BalanceKey(addr codec.Address) (k []byte) {
	k = make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = balancePrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], BalanceChunks)
	return
}

// If locked is 0, then account does not exist
func GetBalance(
	ctx context.Context,
	im state.Immutable,
	addr codec.Address,
) (uint64, error) {
	_, bal, _, err := getBalance(ctx, im, addr)
	return bal, err
}

func getBalance(
	ctx context.Context,
	im state.Immutable,
	addr codec.Address,
) ([]byte, uint64, bool, error) {
	k := BalanceKey(addr)
	bal, exists, err := innerGetBalance(im.GetValue(ctx, k))
	return k, bal, exists, err
}

// Used to serve RPC queries
func GetBalanceFromState(
	ctx context.Context,
	f ReadState,
	addr codec.Address,
) (uint64, error) {
	k := BalanceKey(addr)
	values, errs := f(ctx, [][]byte{k})
	bal, _, err := innerGetBalance(values[0], errs[0])
	return bal, err
}

func innerGetBalance(
	v []byte,
	err error,
) (uint64, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	val, err := database.ParseUInt64(v)
	if err != nil {
		return 0, false, err
	}
	return val, true, nil
}

func SetBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	balance uint64,
) error {
	k := BalanceKey(addr)
	return setBalance(ctx, mu, k, balance)
}

func setBalance(
	ctx context.Context,
	mu state.Mutable,
	key []byte,
	balance uint64,
) error {
	return mu.Insert(ctx, key, binary.BigEndian.AppendUint64(nil, balance))
}

func AddBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
) (uint64, error) {
	key, bal, _, err := getBalance(ctx, mu, addr)
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Add(bal, amount)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: could not add balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
}

func SubBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
) (uint64, error) {
	key, bal, ok, err := getBalance(ctx, mu, addr)
	if !ok {
		return 0, ErrInvalidBalance
	}
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: could not subtract balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	if nbal == 0 {
		// If there is no balance left, we should delete the record instead of
		// setting it to 0.
		return 0, mu.Remove(ctx, key)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
}

const feedIDPrefix byte = metadata.DefaultMinimumPrefix + 1
const feedIDChunks uint16 = 1

// feed id key is used to store the highest feed id available
// [feedIDPrefix]
func FeedIDKey() (k []byte) {
	k = make([]byte, 1+consts.Uint16Len)
	k[0] = feedIDPrefix
	binary.BigEndian.PutUint16(k[1:], feedIDChunks)
	return
}

// If locked is 0, then account does not exist
func GetHighestFeedID(
	ctx context.Context,
	im state.Immutable,
) (uint64, error) {
	_, bal, _, err := getHighestFeedID(ctx, im)
	return bal, err
}

func getHighestFeedID(
	ctx context.Context,
	im state.Immutable,
) ([]byte, uint64, bool, error) {
	k := FeedIDKey()
	feedID, exists, err := innerGetHighestFeedID(im.GetValue(ctx, k))
	return k, feedID, exists, err
}

// Used to serve RPC queries
func GetHighestFeedIDFromState(
	ctx context.Context,
	f ReadState,
	addr codec.Address,
) (uint64, error) {
	k := FeedIDKey()
	values, errs := f(ctx, [][]byte{k})
	bal, _, err := innerGetHighestFeedID(values[0], errs[0])
	return bal, err
}

func innerGetHighestFeedID(
	v []byte,
	err error,
) (uint64, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	val, err := database.ParseUInt64(v)
	if err != nil {
		return 0, false, err
	}
	return val, true, nil
}

func IncrementFeedID(
	ctx context.Context,
	mu state.Mutable,
) error {
	k := FeedIDKey()
	_, prev, exists, err := getHighestFeedID(ctx, mu)
	if err != nil {
		return err
	}
	if !exists {
		return setHighestFeedID(ctx, mu, k, mconsts.StartingFeedID)
	}
	return setHighestFeedID(ctx, mu, k, prev+1)
}

// SetFeed sets the feed value for a given feedID.
func SetHighestFeedID(
	ctx context.Context,
	mu state.Mutable,
	feedID uint64,
) error {
	k := FeedIDKey()
	return setHighestFeedID(ctx, mu, k, feedID)
}

func setHighestFeedID(
	ctx context.Context,
	mu state.Mutable,
	key []byte,
	feedID uint64,
) error {
	return mu.Insert(ctx, key, binary.BigEndian.AppendUint64(nil, feedID))
}

// feed key is used to index individual feed information
const feedPrefix byte = metadata.DefaultMinimumPrefix + 2
const FeedChunks uint16 = 1

// [feedPrefix] + [feedID]
func FeedKey(feedID uint64) (k []byte) {
	k = make([]byte, 1+consts.Uint64Len+consts.Uint16Len)
	k[0] = feedPrefix
	binary.BigEndian.PutUint64(k[1:], feedID)
	binary.BigEndian.PutUint16(k[1+consts.Uint64Len:], FeedChunks)
	return
}

// GetFeed retrieves the feed value indexed by the feedID.
func GetFeed(
	ctx context.Context,
	im state.Immutable,
	feedID uint64,
) ([]byte, error) {
	k := FeedKey(feedID)
	return innerGetFeed(im.GetValue(ctx, k))
}

// GetFeedFromState retrieves the feed value from ReadState, used for RPC queries.
func GetFeedFromState(
	ctx context.Context,
	f ReadState,
	feedID uint64,
) ([]byte, error) {
	k := FeedKey(feedID)
	values, errs := f(ctx, [][]byte{k})
	return innerGetFeed(values[0], errs[0])
}

func innerGetFeed(
	v []byte,
	err error,
) ([]byte, error) {
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

// SetFeed sets the feed value for a given feedID.
func SetFeed(
	ctx context.Context,
	mu state.Mutable,
	feedID uint64,
	value []byte,
) error {
	k := FeedKey(feedID)
	return setFeed(ctx, mu, k, value)
}

func setFeed(
	ctx context.Context,
	mu state.Mutable,
	key []byte,
	value []byte,
) error {
	return mu.Insert(ctx, key, value)
}

// store report indexes, i.e. an array of raw bytes of codec.Address
const feedRoundPrefix byte = metadata.DefaultMinimumPrefix + 3
const FeedRoundChunks uint16 = 1

// [feedRoundPrefix] + [feedID]
func FeedRoundKey(feedID uint64) (k []byte) {
	k = make([]byte, 1+consts.Uint64Len+consts.Uint16Len)
	k[0] = feedRoundPrefix
	binary.BigEndian.PutUint64(k[1:], feedID)
	binary.BigEndian.PutUint16(k[1+consts.Uint64Len:], FeedRoundChunks)
	return
}

func GetFeedRound(
	ctx context.Context,
	im state.Immutable,
	feedID uint64,
) (*common.RoundInfo, error) {
	k := FeedRoundKey(feedID)
	value, exists, err := innerGetValue(im.GetValue(ctx, k))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}

	return common.UnmarshalRoundInfo(value)
}

func SetFeedRound(
	ctx context.Context,
	mu state.Mutable,
	feedID uint64,
	round *common.RoundInfo,
) error {
	k := FeedRoundKey(feedID)
	v, err := round.Marshal()
	if err != nil {
		return err
	}
	return mu.Insert(ctx, k, v)
}

func GetFeedRoundFromState(
	ctx context.Context,
	f ReadState,
	feedID uint64,
) (*common.RoundInfo, error) {
	k := FeedRoundKey(feedID)
	values, errs := f(ctx, [][]byte{k})
	value, exists, err := innerGetValue(values[0], errs[0])
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}
	return common.UnmarshalRoundInfo(value)
}

// store report indexes, i.e. an array of raw bytes of codec.Address
const reportAddressesIndex byte = metadata.DefaultMinimumPrefix + 4
const reportAddressesChunks uint16 = 20

// [reportIndexPrefix] + [feedID] + [round]
func ReportIndexKey(feedID uint64, round uint64) (k []byte) {
	k = make([]byte, 1+consts.Uint64Len+consts.Uint64Len+consts.Uint16Len)
	k[0] = reportAddressesIndex
	binary.BigEndian.PutUint64(k[1:], feedID)
	binary.BigEndian.PutUint64(k[1+consts.Uint64Len:], round)
	binary.BigEndian.PutUint16(k[1+consts.Uint64Len+consts.Int64Len:], reportAddressesChunks)
	return
}

func GetReportAddresses(
	ctx context.Context,
	im state.Immutable,
	round uint64,
	feedID uint64,
) ([]codec.Address, error) {
	k := ReportIndexKey(feedID, round)
	value, exists, err := innerGetValue(im.GetValue(ctx, k))
	if err != nil && err != database.ErrNotFound {
		return nil, err
	} else if !exists {
		return nil, nil
	}
	return common.DecodeAddresses(value)
}

func SetReportAddresses(
	ctx context.Context,
	mu state.Mutable,
	round uint64,
	feedID uint64,
	addrs []codec.Address,
) error {
	k := ReportIndexKey(feedID, round)
	value, err := common.EncodeAddresses(addrs)
	if err != nil {
		return err
	}
	return mu.Insert(ctx, k, value)
}

func GetReportAddressesFromState(
	ctx context.Context,
	f ReadState,
	round uint64,
	feedID uint64,
) ([]codec.Address, error) {
	k := ReportIndexKey(feedID, round)
	values, errs := f(ctx, [][]byte{k})
	value, exists, err := innerGetValue(values[0], errs[0])
	if err != nil && err != database.ErrNotFound {
		return nil, err
	} else if !exists {
		return nil, nil
	}

	return common.DecodeAddresses(value)
}

// store individual reports
const reportPrefix byte = metadata.DefaultMinimumPrefix + 5
const ReportChunks uint16 = 1

// [reportPrefix] + [feedID] + [round] + [address]
func ReportKey(feedID uint64, round uint64, addr codec.Address) (k []byte) {
	k = make([]byte, 1+consts.Uint64Len+consts.Uint64Len+codec.AddressLen+consts.Uint16Len)
	k[0] = reportPrefix
	binary.BigEndian.PutUint64(k[1:], feedID)
	binary.BigEndian.PutUint64(k[1+consts.Uint64Len:], round)
	copy(k[1+consts.Uint64Len+consts.Uint64Len:], addr[:])
	binary.BigEndian.PutUint16(k[1+consts.Uint64Len+consts.Uint64Len+codec.AddressLen:], ReportChunks)
	return
}

func GetReport(
	ctx context.Context,
	im state.Immutable,
	feedID uint64,
	round uint64,
	addr codec.Address,
) ([]byte, error) {
	k := ReportKey(feedID, round, addr)
	value, exists, err := innerGetValue(im.GetValue(ctx, k))
	if !exists {
		return nil, database.ErrNotFound
	}
	return value, err
}

func SetReport(
	ctx context.Context,
	mu state.Mutable,
	feedID uint64,
	round uint64,
	addr codec.Address,
	value []byte,
) error {
	k := ReportKey(feedID, round, addr)
	return mu.Insert(ctx, k, value)
}

func GetReportFromState(
	ctx context.Context,
	f ReadState,
	round uint64,
	feedID uint64,
	addr codec.Address,
) ([]byte, error) {
	k := ReportKey(feedID, round, addr)
	values, errs := f(ctx, [][]byte{k})
	value, _, err := innerGetValue(values[0], errs[0])
	return value, err
}

// store latest few feed results, e.g. last 10 feed results that can be unmarshalled into an array
const feedResultPrefix byte = metadata.DefaultMinimumPrefix + 6
const FeedResultChunks uint16 = 1

// [feedResultPrefix] + [feedID] + [round]
func FeedResultKey(feedID uint64, round uint64) (k []byte) {
	k = make([]byte, 1+consts.Uint64Len+consts.Uint64Len+consts.Uint16Len)
	k[0] = feedResultPrefix
	binary.BigEndian.PutUint64(k[1:], feedID)
	binary.BigEndian.PutUint64(k[1+consts.Uint64Len:], round)
	binary.BigEndian.PutUint16(k[1+consts.Uint64Len+consts.Uint64Len:], FeedResultChunks)
	return
}

func GetFeedResult(
	ctx context.Context,
	im state.Immutable,
	feedID uint64,
	round uint64,
) ([]byte, error) {
	k := FeedResultKey(feedID, round)
	value, exists, err := innerGetValue(im.GetValue(ctx, k))
	if !exists {
		return nil, database.ErrNotFound
	}
	return value, err
}

func SetFeedResult(
	ctx context.Context,
	mu state.Mutable,
	feedID uint64,
	round uint64,
	value []byte,
) error {
	k := FeedResultKey(feedID, round)
	return mu.Insert(ctx, k, value)
}

func GetFeedResultFromState(
	ctx context.Context,
	f ReadState,
	feedID uint64,
	round uint64,
) ([]byte, error) {
	k := FeedResultKey(feedID, round)
	values, errs := f(ctx, [][]byte{k})
	value, _, err := innerGetValue(values[0], errs[0])
	return value, err
}

// store stake for given feed
const feedDepositPrefix byte = metadata.DefaultMinimumPrefix + 7
const FeedDepositChunks uint16 = 1

// [feedDepositPrefix] + [feedID] + [account]
func FeedDepositKey(feedID uint64, account codec.Address) (k []byte) {
	k = make([]byte, 1+consts.Uint64Len+codec.AddressLen+consts.Uint16Len)
	k[0] = feedDepositPrefix
	binary.BigEndian.PutUint64(k[1:], feedID)
	copy(k[1+consts.Uint64Len:], account[:])
	binary.BigEndian.PutUint16(k[1+consts.Uint64Len+codec.AddressLen:], FeedDepositChunks)
	return
}

func GetFeedDeposit(
	ctx context.Context,
	im state.Immutable,
	feedID uint64,
	acct codec.Address,
) (uint64, error) {
	k := FeedDepositKey(feedID, acct)
	value, _, err := innerGetBalance(im.GetValue(ctx, k))
	return value, err
}

func SetFeedDeposit(
	ctx context.Context,
	mu state.Mutable,
	feedID uint64,
	acct codec.Address,
	amount uint64,
) error {
	k := FeedDepositKey(feedID, acct)
	return setBalance(ctx, mu, k, amount)
}

func GetFeedDepositFromState(
	ctx context.Context,
	f ReadState,
	feedID uint64,
	acct codec.Address,
) (uint64, error) {
	k := FeedDepositKey(feedID, acct)
	values, errs := f(ctx, [][]byte{k})
	deposit, _, err := innerGetBalance(values[0], errs[0])
	return deposit, err
}
