package model

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Cursor struct {
	gorm.Model
	// ChainId used for evm chain
	ChainId   uint   `gorm:"not null;default:0;index:t_chainid_name,unique"`
	ChainName string `gorm:"size:255;index:t_chainid_name"`
	// BlockNumber when we scan from_blockNUmber to to_blockNumber, has processed
	BlockNumber uint64 `gorm:"default:0"`
}

// EvmTransaction
// if someone deposit and withdrawal in one transaction, confliction will happen.
type EvmTransaction struct {
	gorm.Model
	// effect address, such as from or to
	Address        string `gorm:"size:255;index:t_address_chainid_hash,unique"`
	ChainId        uint   `gorm:"not null;default:0;index:t_address_chainid_hash"`
	Hash           string `gorm:"size:255;index:t_address_chainid_hash"`
	BlockNumber    uint64 `gorm:"default:0;index:t_blocknumber"`
	BlockTimestamp uint64 `gorm:"default:0;index:t_blocktimestamp"`
	// + for income, - for outcome
	Amount    decimal.Decimal `gorm:"type:decimal(38);default:0"`
	LogMethod string          `gorm:"size:255"`
	Memo      string          `gorm:"size:255"`
}

func GetCursor(database *gorm.DB, chainID uint) (*Cursor, error) {
	var cursor Cursor
	err := database.Model(&Cursor{}).Where("chain_id = ?", chainID).First(&cursor).Error
	return &cursor, err
}
