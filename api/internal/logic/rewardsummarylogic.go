// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package logic

import (
	"context"

	"cuniBTCReward/api/internal/svc"
	"cuniBTCReward/api/internal/types"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type RewardSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRewardSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RewardSummaryLogic {
	return &RewardSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

type rewardSummaryRow struct {
	AmountBase  decimal.Decimal `gorm:"column:amount_base"`
	AmountExtra decimal.Decimal `gorm:"column:amount_extra"`
	Total       decimal.Decimal `gorm:"column:total_amount"`
	Claimed     decimal.Decimal `gorm:"column:claimed_amount"`
	UnClaimed   decimal.Decimal `gorm:"column:unclaimed_amount"`
}

func (l *RewardSummaryLogic) RewardSummary(req *types.RewardSummaryReq) (resp *types.RewardSummaryResp, err error) {
	chainID := l.svcCtx.Config.DefaultChainId
	sql := `WITH strat AS (
    SELECT * FROM strategies WHERE symbol = ? AND chain_id = ? AND deleted_at IS NULL
), top_epoches AS (
    SELECT e.* FROM epoches e
    JOIN strat s ON s.vault = e.contract AND s.chain_id = e.chain_id
    WHERE e.deleted_at IS NULL
), ` + pendingExtraRewardsCTE + `,
extra_summary AS (
    SELECT COALESCE(SUM(amount_extra), 0) AS amount_extra FROM (
        SELECT ar.amount_extra FROM air_drop_records ar
        JOIN strat s ON s.airdrop = ar.contract AND s.chain_id = ar.chain_id
        WHERE ar.deleted_at IS NULL
        UNION ALL
        SELECT amount_extra FROM pending_extra_rewards
    ) rewards
)
SELECT
    COALESCE(SUM(amount), 0) AS total_amount,
    COALESCE(SUM(a.amount_base), 0) AS amount_base,
    (SELECT amount_extra FROM extra_summary) AS amount_extra,
    COALESCE(SUM(CASE WHEN claimed = 1 THEN amount ELSE 0 END), 0) AS claimed_amount,
    COALESCE(SUM(CASE WHEN claimed = 0 THEN amount ELSE 0 END), 0) AS unclaimed_amount
FROM
    air_drop_records a
JOIN strat s ON s.airdrop = a.contract AND s.chain_id = a.chain_id
JOIN air_drop_epoches ae ON ae.contract = s.airdrop AND ae.epoch = a.epoch AND ae.chain_id = a.chain_id
WHERE s.deleted_at IS NULL AND a.deleted_at IS NULL AND ae.deleted_at IS NULL AND ae.root = ae.merkle_root`
	args := []interface{}{
		req.Symbol,
		chainID,
	}
	var row rewardSummaryRow
	err = l.svcCtx.Database.WithContext(l.ctx).Raw(sql, args...).Scan(&row).Error
	if err != nil {
		return
	}

	resp = &types.RewardSummaryResp{
		AmountBase:  row.AmountBase.Mul(decimal.New(1, -8)).String(),
		AmountExtra: row.AmountExtra.Mul(decimal.New(1, -8)).String(),
		Total:       row.Total.Mul(decimal.New(1, -8)).String(),
		Claimed:     row.Claimed.Mul(decimal.New(1, -8)).String(),
		UnClaimed:   row.UnClaimed.Mul(decimal.New(1, -8)).String(),
	}

	return
}
