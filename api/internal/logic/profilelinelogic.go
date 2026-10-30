// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cuniBTCReward/api/internal/svc"
	"cuniBTCReward/api/internal/types"
	"cuniBTCReward/model"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ProfileLineLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProfileLineLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfileLineLogic {
	return &ProfileLineLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProfileLineLogic) ProfileLine(req *types.ProfileLineReq) (resp *types.ProfileLineResp, err error) {
	end := time.Now().Unix()
	start, step, err := profileLineRange(req.Range, end)
	if err != nil {
		return nil, err
	}
	if req.Symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	// Same noLock strategy selection as DistributeRewards.
	if req.Symbol != "b2uniBTC" {
		return nil, fmt.Errorf("profileline only supports noLock strategies (b2uniBTC), got %q", req.Symbol)
	}
	var strategy model.Strategy
	var epochs []profileLineEpoch
	var events, extras []profileLineEvent
	var opening decimal.Decimal
	// Keep the opening balance, events and reward snapshots consistent during distribution/scanning.
	err = l.svcCtx.Database.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		chainID := l.svcCtx.Config.DefaultChainId
		if err := tx.Where("chain_id = ? AND symbol = ?", chainID, req.Symbol).First(&strategy).Error; err != nil {
			return err
		}
		epochSQL := `SELECT e.epoch, e.operate_start, e.lockup_start, e.lockup_period,
    COALESCE(a.amount_base, 0) AS amount_base, COALESCE(a.record_count, 0) > 0 AS base_generated
FROM epoches e
LEFT JOIN (
    SELECT epoch, SUM(amount_base) AS amount_base, COUNT(*) AS record_count
    FROM air_drop_records
    WHERE chain_id = ? AND contract = ? AND deleted_at IS NULL
    GROUP BY epoch
) a ON a.epoch = e.epoch
WHERE e.chain_id = ? AND e.contract = ? AND e.deleted_at IS NULL
  AND e.operate_start < ? AND e.lockup_start + e.lockup_period > ?
ORDER BY e.operate_start, e.epoch`
		if err := tx.Raw(epochSQL, chainID, strategy.Airdrop, chainID, strategy.Vault, end, start).Scan(&epochs).Error; err != nil {
			return err
		}
		// Minted is the deposit event; the router records redemptions as negative amounts.
		ledger := tx.Model(&model.EvmTransaction{}).Where("deleted_at IS NULL AND chain_id = ? AND contract IN ?", chainID,
			[]string{strategy.Vault, strategy.DelayRedeemRouter})
		if err := ledger.Session(&gorm.Session{}).Select("COALESCE(SUM(amount), 0)").
			Where("block_timestamp < ?", start).Scan(&opening).Error; err != nil {
			return err
		}
		if err := ledger.Session(&gorm.Session{}).Select(`block_timestamp AS timestamp, SUM(amount) AS amount,
SUM(CASE WHEN contract = ? AND log_method = 'Minted' AND amount > 0 THEN amount ELSE 0 END) AS deposited`, strategy.Vault).
			Where("block_timestamp >= ? AND block_timestamp < ?", start, end).
			Group("block_timestamp").Order("block_timestamp").Scan(&events).Error; err != nil {
			return err
		}
		// Always use original extra reward timestamps, including after airdrop generation.
		return tx.Model(&model.ExtraRewardDis{}).Select("timestamp, SUM(amount) AS amount").
			Where("deleted_at IS NULL AND symbol = ? AND timestamp >= ? AND timestamp < ?", req.Symbol, start, end).
			Group("timestamp").Order("timestamp").Scan(&extras).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return buildProfileLine(req.Symbol, req.Range, start, end, step, opening, events, extras, epochs), nil
}
