package model

import (
	"github.com/ethereum/go-ethereum/core/types"
)

type ChainBlock struct {
	Number    uint64
	Txs       []*ChainTransaction
	Receipts  []*ChainReceipt
	Timestamp uint64
}

type ChainTransaction struct {
	Id        string
	From      string
	To        string
	Block     uint64
	Idx       uint32
	Timestamp uint64
	Input     string
}

type ChainReceipt struct {
	*types.Receipt
	Timestamp uint64
}
