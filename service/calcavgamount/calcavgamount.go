package calcavgamount

import (
	"cuniBTCReward/model"
	"cuniBTCReward/pkg/gormz"
	"fmt"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// CalcAvgAmount startAmount is the amount < startBlock
func CalcAvgAmount(startAmount decimal.Decimal, transactions []*model.EvmTransaction, startBlock, endBlock uint64) (decimal.Decimal, error) {
	if startBlock >= endBlock {
		return decimal.Zero, fmt.Errorf("startBlock[%d] >= endBlock[%d]", startBlock, endBlock)
	}
	if startAmount.IsNegative() {
		return decimal.Zero, fmt.Errorf("startAmount[%s] is negative", startAmount.String())
	}
	if len(transactions) == 0 {
		return startAmount, nil
	}
	totalBlocks := endBlock - startBlock
	var weightedSum decimal.Decimal
	var previousAmount = startAmount
	var previousBlock = startBlock

	for _, tx := range transactions {
		if tx.BlockNumber < startBlock || tx.BlockNumber > endBlock {
			continue
		}
		blockDelta := tx.BlockNumber - previousBlock
		weightedSum = weightedSum.Add(previousAmount.Mul(decimal.NewFromInt(int64(blockDelta))))
		previousAmount = previousAmount.Add(tx.Amount)
		previousBlock = tx.BlockNumber
		if previousAmount.IsNegative() {
			return decimal.Zero, fmt.Errorf("previousAmount[%s] is negative, txHash[%s]", previousAmount.String(), tx.Hash)
		}
	}

	blockDelta := endBlock - previousBlock
	weightedSum = weightedSum.Add(previousAmount.Mul(decimal.NewFromInt(int64(blockDelta))))

	return weightedSum.Div(decimal.NewFromInt(int64(totalBlocks))).Floor(), nil
}

type Calulator struct {
	database *gorm.DB
	chainId  uint
}

func NewCalulator(database string, chainId uint) *Calulator {
	db, err := gorm.Open(mysql.Open(database))
	db.Logger = gormz.NewGormLogger()
	logx.Must(err)

	return &Calulator{
		database: db,
		chainId:  chainId,
	}
}

type AvgAmountResult struct {
	Address   string
	AvgAmount decimal.Decimal
}

func (c *Calulator) GetAvgAmount(startBlock, endBlock uint64) (result []AvgAmountResult, err error) {
	startAmount, err := c.getAllAddressStartAmount(startBlock)
	if err != nil {
		return nil, err
	}
	for _, item := range startAmount {
		if item.AvgAmount.IsNegative() {
			return nil, fmt.Errorf("startAmount[%s] is negative for address[%s]", item.AvgAmount.String(), item.Address)
		}
		transactions, err := c.getAddressTransactions(item.Address, startBlock, endBlock)
		if err != nil {
			return nil, err
		}
		avgAmount, err := CalcAvgAmount(item.AvgAmount, transactions, startBlock, endBlock)
		if err != nil {
			return nil, fmt.Errorf("calc avg amount failed for address[%s], err: %v", item.Address, err)
		}
		result = append(result, AvgAmountResult{
			Address:   item.Address,
			AvgAmount: avgAmount,
		})
	}
	return
}

func (c *Calulator) getAllAddressStartAmount(startBlock uint64) (result []AvgAmountResult, err error) {
	var distinctAddresses []string
	ret := c.database.Model(&model.EvmTransaction{}).Distinct("address").
		Where("chain_id = ?", c.chainId).Pluck("address", &distinctAddresses)
	if ret.Error != nil {
		return nil, ret.Error
	}

	var addressAmounts []AvgAmountResult
	ret = c.database.Model(&model.EvmTransaction{}).Select("address, sum(amount) as avg_amount").
		Where("chain_id = ? and block_number < ?", c.chainId, startBlock).Group("address").Scan(&addressAmounts)
	if ret.Error != nil {
		return nil, ret.Error
	}

	addressAmountMap := lo.SliceToMap(addressAmounts, func(item AvgAmountResult) (string, AvgAmountResult) {
		return item.Address, item
	})

	result = lo.Map(distinctAddresses, func(address string, index int) AvgAmountResult {
		if item, ok := addressAmountMap[address]; ok {
			return item
		}
		return AvgAmountResult{
			Address:   address,
			AvgAmount: decimal.Zero,
		}
	})
	return
}

func (c *Calulator) getAddressTransactions(address string, startBlock, endBlock uint64) (result []*model.EvmTransaction, err error) {
	ret := c.database.Model(&model.EvmTransaction{}).
		Where("chain_id = ? and address = ? and block_number >= ? and block_number <= ?", c.chainId, address, startBlock, endBlock).
		Order("block_number").
		Find(&result)
	if ret.Error != nil {
		return nil, ret.Error
	}
	return
}
