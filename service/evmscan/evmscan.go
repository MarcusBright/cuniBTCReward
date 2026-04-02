package evmscan

import (
	"context"
	"cuniBTCReward/clientrpc/goeth"
	"cuniBTCReward/model"
	"cuniBTCReward/pkg/gormz"
	"cuniBTCReward/pkg/slack"
	"cuniBTCReward/service/contract/cunibtcvault"
	"cuniBTCReward/service/contract/delayredeemrouter"
	"cuniBTCReward/service/evmscan/config"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Scanner struct {
	database        *gorm.DB
	config          *config.EvmScanConf
	evmClients      []*EvmClient
	cuniBTCVaultAbi *abi.ABI
	redeemRouterAbi *abi.ABI
}

type EvmClient struct {
	Client       *ethclient.Client
	CuniBTCVault *cunibtcvault.Cunibtcvault
	RedeemRouter *delayredeemrouter.Delayredeemrouter
}

func NewScanner(c *config.EvmScanConf) *Scanner {
	db, err := gorm.Open(mysql.Open(c.DataSource))
	if c.SqlLog {
		db.Logger = gormz.NewGormLogger()
	}
	logx.Must(err)
	evmClients := lo.Map(c.ChainInfo, func(item config.ChainInfo, index int) *EvmClient {
		client := goeth.NewClient(item.Client.Host, item.Client.Request, item.Client.PeriodSec)
		cunibtcvault, err := cunibtcvault.NewCunibtcvault(common.HexToAddress(item.CuniBTCVault), client)
		logx.Must(err)
		redeemRouter, err := delayredeemrouter.NewDelayredeemrouter(common.HexToAddress(item.DelayRedeemRouter), client)
		logx.Must(err)
		return &EvmClient{
			Client:       client,
			CuniBTCVault: cunibtcvault,
			RedeemRouter: redeemRouter,
		}
	})

	cuniBTCVaultAbi, _ := cunibtcvault.CunibtcvaultMetaData.GetAbi()
	redeemRouterAbi, _ := delayredeemrouter.DelayredeemrouterMetaData.GetAbi()

	return &Scanner{
		database:        db,
		config:          c,
		evmClients:      evmClients,
		cuniBTCVaultAbi: cuniBTCVaultAbi,
		redeemRouterAbi: redeemRouterAbi,
	}
}

func (s *Scanner) LogScan() {
	for k, chain := range s.config.ChainInfo {
		logx.Infof("chain: %v, name: %v, logscan", chain.Client.ChainId, chain.Client.ChainName)

		cursor, err := s.getCursor(uint64(chain.Client.ChainId))
		if err != nil {
			logx.Errorf("get chain: %v, cursor failed, err: %v", chain.Client.ChainId, err)
			return
		}

		start, end, err := s.getScanRange(s.evmClients[k].Client, cursor.BlockNumber)
		if err != nil {
			logx.Errorf("get chain: %v, latest block number failed, err: %v", chain.Client.ChainId, err)
			continue
		}
		if start == 0 || end == 0 {
			logx.Infof("chain: %v, no new block", chain.Client.ChainId)
			continue
		}
		logx.Infof("chain: %v, need scan blocks start:%v, end:%v", chain.Client.ChainId, start, end)

		logs, err := s.fetchLogs(s.evmClients[k].Client, start, end, chain.CuniBTCVault, chain.DelayRedeemRouter)
		if err != nil {
			logx.Errorf("get chain: %v, filter logs failed, err: %v", chain.Client.ChainId, err)
			continue
		}

		events, err := s.processLogs(logs, s.evmClients[k], chain)
		if err != nil {
			logx.Errorf("process logs failed, err: %v", err)
			continue
		}

		if err := s.saveScanResult(events, chain, cursor, end); err != nil {
			logx.Errorf("save events failed, err: %v", err)
			continue
		}
	}
}

func (s *Scanner) processLogs(logs []types.Log, evmClient *EvmClient, chainInfo config.ChainInfo) ([]*model.EvmTransaction, error) {
	events := make([]*model.EvmTransaction, 0)
	for _, log := range logs {
		if log.Removed {
			logx.Errorf("log removed, hash:%v, blockNumber:%v, blockHash:%v", log.TxHash, log.BlockNumber, log.BlockHash)
			return nil, fmt.Errorf("log removed")
		}
		transactionRecipient, err := evmClient.Client.TransactionReceipt(context.Background(), log.TxHash)
		if err != nil {
			logx.Errorf("get transaction receipt failed, err: %v", err)
			return nil, err
		}
		if transactionRecipient.Status != types.ReceiptStatusSuccessful {
			logx.Infof("transaction status not successful, hash: %v, status: %v", log.TxHash.Hex(), transactionRecipient.Status)
			continue
		}
		switch log.Address {
		case common.HexToAddress(chainInfo.CuniBTCVault):
			// Process CuniBTCVault events
			evmTransaction, err := s.processCuniBTCVaultLog(log, chainInfo, evmClient)
			if err != nil {
				logx.Errorf("processCuniBTCVaultLog error, err: %v", err)
				return nil, err
			}
			if evmTransaction != nil {
				events = append(events, evmTransaction)
			}

		case common.HexToAddress(chainInfo.DelayRedeemRouter):
			// Process DelayRedeemRouter events
			evmTransaction, err := s.processDelayRedeemRouterLog(log, chainInfo, evmClient)
			if err != nil {
				logx.Errorf("processDelayRedeemRouterLog error, err: %v", err)
				return nil, err
			}
			if evmTransaction != nil {
				events = append(events, evmTransaction)
			}
		}
	}
	return events, nil
}

func (s *Scanner) processCuniBTCVaultLog(log types.Log, chainInfo config.ChainInfo, evmClient *EvmClient) (*model.EvmTransaction, error) {
	eventName, err := s.cuniBTCVaultAbi.EventByID(log.Topics[0])
	if err != nil {
		logx.Errorf("get event name failed, maybe upgraded hash: %v, err: %v", log.TxHash, err)
		return nil, nil
	}
	switch eventName.Name {
	case "Minted":
		mintedEvent, err := evmClient.CuniBTCVault.ParseMinted(log)
		if err != nil {
			logx.Errorf("parse minted event failed, err: %v", err)
			return nil, err
		}
		return &model.EvmTransaction{
			Address:        mintedEvent.Sender.String(),
			ChainId:        chainInfo.Client.ChainId,
			Hash:           log.TxHash.String(),
			BlockNumber:    log.BlockNumber,
			BlockTimestamp: log.BlockTimestamp,
			Amount:         decimal.NewFromBigInt(mintedEvent.Amount, 0),
			LogMethod:      "Minted",
		}, nil
	}
	return nil, nil
}

func (s *Scanner) processDelayRedeemRouterLog(log types.Log, chainInfo config.ChainInfo, evmClient *EvmClient) (*model.EvmTransaction, error) {
	eventName, err := s.redeemRouterAbi.EventByID(log.Topics[0])
	if err != nil {
		logx.Errorf("get event name failed, maybe upgraded hash: %v, err: %v", log.TxHash, err)
		return nil, nil
	}
	switch eventName.Name {
	case "DelayedRedeemCreated":
		redeemCreatedEvent, err := evmClient.RedeemRouter.ParseDelayedRedeemCreated(log)
		if err != nil {
			logx.Errorf("parse delayed redeem created event failed, err: %v", err)
			return nil, err
		}
		return &model.EvmTransaction{
			Address:        redeemCreatedEvent.Recipient.String(),
			ChainId:        chainInfo.Client.ChainId,
			Hash:           log.TxHash.String(),
			BlockNumber:    log.BlockNumber,
			BlockTimestamp: log.BlockTimestamp,
			Amount:         decimal.NewFromBigInt(redeemCreatedEvent.Amount, 0).Neg(),
			LogMethod:      "DelayedRedeemCreated",
		}, nil
	}
	return nil, nil
}

func (s *Scanner) getCursor(chainID uint64) (*model.Cursor, error) {
	var cursor model.Cursor
	err := s.database.Model(&model.Cursor{}).Where("chain_id = ?", chainID).First(&cursor).Error
	return &cursor, err
}

func (s *Scanner) getScanRange(client *ethclient.Client, cursorBlock uint64) (int64, int64, error) {
	latestBlockNumber, err := client.BlockNumber(context.Background())
	if err != nil {
		return 0, 0, err
	}
	latestBlockNumber = latestBlockNumber - 30 //delay block
	needScanBlocksNumber := int64(latestBlockNumber) - int64(cursorBlock)
	if needScanBlocksNumber <= 0 {
		return 0, 0, nil
	}
	if needScanBlocksNumber > 1000 {
		needScanBlocksNumber = 1000
	}
	start := int64(cursorBlock + 1)
	end := int64(cursorBlock) + needScanBlocksNumber
	return start, end, nil
}

func (s *Scanner) fetchLogs(client *ethclient.Client, start, end int64, cuniBTC string, redeemRouter string) ([]types.Log, error) {
	return client.FilterLogs(context.Background(), ethereum.FilterQuery{
		FromBlock: big.NewInt(start),
		ToBlock:   big.NewInt(end),
		Addresses: []common.Address{common.HexToAddress(cuniBTC), common.HexToAddress(redeemRouter)},
	})
}

func (s *Scanner) saveScanResult(events []*model.EvmTransaction, chain config.ChainInfo, cursor *model.Cursor, end int64) error {
	return s.database.Transaction(func(tx *gorm.DB) error {
		ret := s.database.CreateInBatches(events, 100)
		if ret.Error != nil {
			logx.Errorf("create evm transactions failed, err: %v", ret.Error)
			return ret.Error
		}
		if ret.RowsAffected != int64(len(events)) {
			logx.Errorf("create evm transactions failed, expected %d, actual %d", len(events), ret.RowsAffected)
			return fmt.Errorf("create evm transactions failed")
		}
		//save cursor
		cursor.BlockNumber = uint64(end)
		if err := tx.Save(cursor).Error; err != nil {
			logx.Errorf("save cursor failed, err: %v", err)
			return err
		}
		if ret.RowsAffected != 0 {
			slack.SendTo(s.config.NotifySlack, fmt.Sprintf("[%s] chain[%d], evm transactions saved[%d]", s.config.Name, chain.Client.ChainId, ret.RowsAffected))
		}
		return nil
	})
}
