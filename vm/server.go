// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"net/http"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/bianyuanop/oraclevm/actions"
	"github.com/bianyuanop/oraclevm/common"
	"github.com/bianyuanop/oraclevm/consts"
	"github.com/bianyuanop/oraclevm/storage"
)

const JSONRPCEndpoint = "/morpheusapi"

var _ api.HandlerFactory[api.VM] = (*jsonRPCServerFactory)(nil)

type jsonRPCServerFactory struct{}

func (jsonRPCServerFactory) New(vm api.VM) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(consts.Name, NewJSONRPCServer(vm))
	return api.Handler{
		Path:    JSONRPCEndpoint,
		Handler: handler,
	}, err
}

type JSONRPCServer struct {
	vm api.VM
}

func NewJSONRPCServer(vm api.VM) *JSONRPCServer {
	return &JSONRPCServer{vm: vm}
}

type GenesisReply struct {
	Genesis *genesis.DefaultGenesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = j.vm.Genesis().(*genesis.DefaultGenesis)
	return nil
}

type BalanceArgs struct {
	Address codec.Address `json:"address"`
}

type BalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) Balance(req *http.Request, args *BalanceArgs, reply *BalanceReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Balance")
	defer span.End()

	balance, err := storage.GetBalanceFromState(ctx, j.vm.ReadState, args.Address)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}

type FeedInfoArgs struct {
	FeedID uint64 `json:"feedID"`
}

type FeedInfoReply struct {
	Info *actions.RegisterFeed `json:"info"`
}

func (j *JSONRPCServer) FeedInfo(req *http.Request, args *FeedInfoArgs, reply *FeedInfoReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.FeedInfo")
	defer span.End()

	feedInfoRaw, err := storage.GetFeedFromState(ctx, j.vm.ReadState, args.FeedID)
	if err != nil {
		return err
	}
	feedInfo, err := actions.UnmarshalFeed(feedInfoRaw)
	if err != nil {
		return err
	}

	reply.Info = feedInfo
	return nil
}

type FeedResultArgs struct {
	FeedID uint64 `json:"feedID"`
	Round  uint64 `json:"round"`
}

type FeedResultReply struct {
	Value     []byte `json:"value"`
	ProgramID uint64 `json:"programID"`
	Finalized bool   `json:"finalized"`
}

func (j *JSONRPCServer) FeedResult(req *http.Request, args *FeedResultArgs, reply *FeedResultReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.FeedResult")
	defer span.End()

	feedInfoRaw, err := storage.GetFeedFromState(ctx, j.vm.ReadState, args.FeedID)
	if err != nil {
		return err
	}
	feedInfo, err := actions.UnmarshalFeed(feedInfoRaw)
	if err != nil {
		return err
	}

	rawFeed, err := storage.GetFeedResultFromState(ctx, j.vm.ReadState, args.FeedID, args.Round)
	if err != nil {
		return err
	}

	roundInfo, err := storage.GetFeedRoundFromState(ctx, j.vm.ReadState, args.FeedID)
	if err != nil {
		return err
	}

	reply.Value = rawFeed
	reply.ProgramID = feedInfo.ProgramID
	reply.Finalized = roundInfo.RoundNumber > args.Round
	return nil
}

type BribeInfoArgs struct {
	FeedID    uint64        `json:"feedID"`
	Round     uint64        `json:"round"`
	Recipient codec.Address `json:"recipient"`
}

type BribeInfoReply struct {
	Bribes []*common.BribeInfo `json:"bribes"`
}

func (j *JSONRPCServer) BribeInfo(req *http.Request, args *BribeInfoArgs, reply *BribeInfoReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.BribeInfo")
	defer span.End()

	bribes, err := storage.GetFeedBribesFromState(ctx, j.vm.ReadState, args.FeedID, args.Recipient, args.Round)
	if err != nil {
		return err
	}
	reply.Bribes = bribes
	return nil
}
