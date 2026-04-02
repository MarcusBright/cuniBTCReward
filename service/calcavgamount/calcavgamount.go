package calcavgamount

import (
	"cuniBTCReward/model"
	"fmt"

	"github.com/shopspring/decimal"
)

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

	return weightedSum.Div(decimal.NewFromInt(int64(totalBlocks))), nil
}
