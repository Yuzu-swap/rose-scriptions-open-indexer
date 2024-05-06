package chain

import (
	"context"
	"encoding/hex"
	"math/big"
	"rose-scriptions-open-indexer/core/model"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"
)

type BlockchainClient struct {
	client *ethclient.Client
}

func NewBlockchainClient(ethURL string) (*BlockchainClient, error) {
	client, err := ethclient.Dial(ethURL)
	if err != nil {
		return nil, err
	}
	return &BlockchainClient{client: client}, nil
}

func (bc *BlockchainClient) GetBlock(blockNumber int64) (*types.Block, error) {
	block, err := bc.client.BlockByNumber(context.Background(), big.NewInt(blockNumber))
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (bc *BlockchainClient) GetBlockReceiptsByAPI(blockNumber int64) ([]*types.Receipt, error) {
	block, err := bc.client.BlockReceipts(context.Background(), rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(blockNumber)))
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (bc *BlockchainClient) GetBlockReceipts(block *types.Block) ([]*types.Receipt, error) {
	var res []*types.Receipt
	for _, tx := range block.Transactions() {
		if receipt, err := bc.client.TransactionReceipt(context.Background(), tx.Hash()); err != nil {
			logrus.Errorf("GetBlockReceipts %v err: %v", tx.Hash(), err)
			return nil, err
		} else {
			res = append(res, receipt)
		}

	}
	return res, nil
}

func (bc *BlockchainClient) GetLatestBlockNumber() (int64, error) {
	header, err := bc.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	return header.Number.Int64(), nil
}

func ConvertBlockToChainBlock(block *types.Block, receipts []*types.Receipt) *model.ChainBlock {
	chainBlock := &model.ChainBlock{
		Number:    block.Number().Uint64(),
		Timestamp: block.Time(),
	}
	for idx, tx := range block.Transactions() {
		from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			logrus.Fatal("Failed to get sender: %v", err)
			continue
		}

		var to string
		if tx.To() != nil {
			to = tx.To().Hex()
		}

		chainTx := &model.ChainTransaction{
			Id:        tx.Hash().Hex(),
			From:      from.Hex(),
			To:        to,
			Block:     block.Number().Uint64(),
			Idx:       uint32(idx),
			Timestamp: block.Time(),
			Input:     "0x" + hex.EncodeToString(tx.Data()),
		}
		chainBlock.Txs = append(chainBlock.Txs, chainTx)
	}
	for _, receipt := range receipts {
		chainReceipt := &model.ChainReceipt{
			Receipt:   receipt,
			Timestamp: block.Time(),
		}
		chainBlock.Receipts = append(chainBlock.Receipts, chainReceipt)
	}
	return chainBlock
}
