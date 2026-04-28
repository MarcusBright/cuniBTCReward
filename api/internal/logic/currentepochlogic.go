// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package logic

import (
	"context"
	"errors"

	"cuniBTCReward/api/internal/svc"
	"cuniBTCReward/api/internal/types"
	"cuniBTCReward/model"

	"github.com/samber/lo"
	"github.com/zeromicro/go-zero/core/logx"
)

type CurrentEpochLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCurrentEpochLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CurrentEpochLogic {
	return &CurrentEpochLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func getVaultContract(symbol string, stratedy []model.Strategy) string {
	for _, v := range stratedy {
		if v.Symbol == symbol {
			return v.Vault
		}
	}
	return ""
}
func getVaultSymbol(vaultContract string, stratedy []model.Strategy) string {
	for _, v := range stratedy {
		if v.Vault == vaultContract {
			return v.Symbol
		}
	}
	return ""
}

func (l *CurrentEpochLogic) CurrentEpoch(req *types.CurrentEpochReq) (resp []types.CurrentEpochResp, err error) {
	// todo: add your logic here and delete this line
	//find all stratedy
	var stratedy []model.Strategy
	err = l.svcCtx.Database.Model(&model.Strategy{}).Where("chain_id = ?", l.svcCtx.Config.DefaultChainId).Find(&stratedy).Error
	if err != nil {
		return
	}
	if len(stratedy) == 0 {
		return resp, errors.New("no stratedy")
	}
	var latestEpoches []model.Epoch
	if req.Symbol == "" {
		err = l.svcCtx.Database.Raw(`SELECT * FROM epoches 
WHERE (contract, epoch) IN (
    SELECT contract, MAX(epoch) 
    FROM epoches 
    WHERE deleted_at IS NULL 
    GROUP BY contract
) AND deleted_at IS NULL`).Scan(&latestEpoches).Error
		if err != nil {
			return
		}
		if len(latestEpoches) == 0 {
			return resp, errors.New("no stratedy")
		}
	} else {
		err = l.svcCtx.Database.Raw(`SELECT * FROM epoches 
WHERE (contract, epoch) IN (
    SELECT contract, MAX(epoch) 
    FROM epoches 
    WHERE contract = ? AND deleted_at IS NULL 
    GROUP BY contract
) AND deleted_at IS NULL`, getVaultContract(req.Symbol, stratedy)).Scan(&latestEpoches).Error
		if err != nil {
			return
		}
		if len(latestEpoches) == 0 {
			return resp, errors.New("no stratedy")
		}
	}

	currentEpoch := lo.Map(latestEpoches, func(item model.Epoch, index int) types.CurrentEpochResp {
		epoch := types.CurrentEpochResp{
			Epoch:                 item.Epoch,
			OperateStartTimestamp: item.OperateStartTimestamp,
			OperatePeriod:         item.OperatePeriod * 12,
			LockupStartTimestamp:  item.LockupStartTimestamp,
			LockupPeriod:          item.LockupPeriod * 12,
			Symbol:                getVaultSymbol(item.Contract, stratedy),
		}
		if epoch.LockupStartTimestamp == 0 {
			epoch.LockupStartTimestamp = epoch.OperateStartTimestamp + epoch.OperatePeriod*12
		}
		return epoch
	})

	resp = currentEpoch
	return
}
