package main

import (
	"rose-scriptions-open-indexer/chain"
	"rose-scriptions-open-indexer/core"
	"rose-scriptions-open-indexer/core/model"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	EnvChainUrl = "CHAIN_URL"
)

func main() {
	chainUrl := "https://emerald.oasis.dev"
	bc, err := chain.NewBlockchainClient(chainUrl)
	if err != nil {
		logrus.Fatalf("Failed to create client: %v", err)
	}

	var wg sync.WaitGroup

	wg.Add(1)

	go startChainFetcher(bc, &wg)

	wg.Wait()
}

func getBlockInfo(bc *chain.BlockchainClient, bcNumber uint64) (*model.ChainBlock, error) {
	if blockInfo, err := bc.GetBlock(int64(bcNumber)); err != nil {
		logrus.Errorf("GetBlock %d err: %v", bcNumber, err)
		return nil, err
	} else {
		if receipts, err := bc.GetBlockReceipts(blockInfo); err != nil {
			logrus.Errorf("GetBlockReceipt %d err: %v", bcNumber, err)
			return nil, err
		} else {
			return chain.ConvertBlockToChainBlock(blockInfo, receipts), nil
		}
	}
}

func startChainFetcher(bc *chain.BlockchainClient, wg *sync.WaitGroup) {
	for {
		bcNumber, err := bc.GetLatestBlockNumber()
		if err != nil {
			logrus.Errorf("GetLatestBlockNumber err: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		logrus.Infof("lastDBNumber: %d, latestChainNumber: %d", core.LatestBlockNumber, bcNumber)
		if core.LatestBlockNumber == uint64(bcNumber) {
			time.Sleep(3 * time.Second)
			continue
		}

		for i := core.LatestBlockNumber + 1; i <= uint64(bcNumber); i++ {
			if bcinfo, err := getBlockInfo(bc, i); err != nil {
				logrus.Errorf("GetBlock %d err: %v", i, err)
				time.Sleep(1 * time.Second)
				continue
			} else {
				logrus.Infof("HandleNewBlock %d, trx %d,receipts len %d, receipts %v ", i, len(bcinfo.Txs), len(bcinfo.Receipts), bcinfo.Receipts)
				if err := core.HandleNewBlock(bcinfo); err != nil {
					logrus.Errorf("HandleNewBlock %d err: %v", i, err)
					time.Sleep(1 * time.Second)
					break
				} else {
					logrus.Infof("HandleNewBlock %d success", i)
				}
			}
		}
	}
	wg.Add(-1)
}
