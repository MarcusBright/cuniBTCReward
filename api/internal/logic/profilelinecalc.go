package logic

import (
	"fmt"
	"math"

	"cuniBTCReward/api/internal/types"

	"github.com/shopspring/decimal"
)

const profileLineDay int64 = 86400

type profileLineEvent struct {
	Timestamp int64
	Amount    decimal.Decimal
	Deposited decimal.Decimal
}

type profileLineEpoch struct {
	Epoch        uint64
	OperateStart int64
	LockupStart  int64
	LockupPeriod int64
	AmountBase   decimal.Decimal
	Generated    bool
}

func profileLineRange(value string, end int64) (start, step int64, err error) {
	var duration int64
	switch value {
	case "24H":
		duration, step = profileLineDay, 3600
	case "7D":
		duration, step = 7*profileLineDay, profileLineDay
	case "30D":
		duration, step = 30*profileLineDay, profileLineDay
	case "All":
		duration, step = 365*profileLineDay, 7*profileLineDay
	default:
		return 0, 0, fmt.Errorf("invalid range %q: expected 24H, 7D, 30D or All", value)
	}
	return end - duration, step, nil
}

// Inputs are ordered by timestamp. Amounts stay in raw token units until serialization.
func buildProfileLine(symbol, rangeValue string, start, end, step int64, noLock bool,
	opening decimal.Decimal, events, extras []profileLineEvent, epochs []profileLineEpoch,
) *types.ProfileLineResp {
	resp := &types.ProfileLineResp{
		Symbol: symbol, Range: rangeValue, Deposited: "0", StartTime: start, EndTime: end,
		Data: make([]types.ProfileLinePoint, 0, (end-start+step-1)/step),
	}
	totalDeposited := decimal.Zero
	for bucketStart := start; bucketStart < end; bucketStart += step {
		bucketEnd := min(bucketStart+step, end)
		base, extra, deposited, principalSeconds := decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero
		complete := true
		for _, epoch := range epochs {
			earningStart := epoch.LockupStart
			if noLock {
				earningStart = epoch.OperateStart
			}
			earningEnd := epoch.LockupStart + epoch.LockupPeriod
			left, right := max(bucketStart, earningStart), min(bucketEnd, earningEnd)
			if left >= right {
				continue
			}
			if !epoch.Generated {
				complete = false
			} else {
				// Cumulative allocation makes adjacent buckets telescope, preserving the
				// epoch total even when its duration is not divisible by bucket size.
				allocation := func(at int64) decimal.Decimal {
					return epoch.AmountBase.Mul(decimal.NewFromInt(at-earningStart)).
						DivRound(decimal.NewFromInt(earningEnd-earningStart), 24)
				}
				base = base.Add(allocation(right).Sub(allocation(left)))
			}
			if !noLock {
				// Locked strategies calculate their eligible shares from transactions
				// strictly before lockup_start; deposits during lockup are queued.
				balance := profileLineBalanceBefore(opening, events, epoch.LockupStart)
				if balance.IsPositive() {
					principalSeconds = principalSeconds.Add(balance.Mul(decimal.NewFromInt(right - left)))
				}
			}
		}
		if noLock {
			principalSeconds = profileLinePrincipalSeconds(opening, events, bucketStart, bucketEnd)
		}
		for _, event := range events {
			if event.Timestamp >= bucketStart && event.Timestamp < bucketEnd {
				deposited = deposited.Add(event.Deposited)
			}
		}
		for _, event := range extras {
			if event.Timestamp >= bucketStart && event.Timestamp < bucketEnd {
				extra = extra.Add(event.Amount)
			}
		}
		duration := bucketEnd - bucketStart
		principal := principalSeconds.DivRound(decimal.NewFromInt(duration), 24)
		rewards := base.Add(extra)
		resp.Data = append(resp.Data, types.ProfileLinePoint{
			StartTime: bucketStart, EndTime: bucketEnd,
			Apy:        profileLineAPY(rewards, principalSeconds, duration),
			AmountBase: profileLineAmount(base), AmountExtra: profileLineAmount(extra),
			Rewards: profileLineAmount(rewards), Deposited: profileLineAmount(deposited),
			EffectivePrincipal: profileLineAmount(principal), BaseRewardComplete: complete,
		})
		totalDeposited = totalDeposited.Add(deposited)
	}
	resp.Deposited = profileLineAmount(totalDeposited)
	return resp
}

func profileLineBalanceBefore(opening decimal.Decimal, events []profileLineEvent, at int64) decimal.Decimal {
	balance := opening
	for _, event := range events {
		if event.Timestamp >= at {
			break
		}
		balance = balance.Add(event.Amount)
	}
	return balance
}

func profileLinePrincipalSeconds(opening decimal.Decimal, events []profileLineEvent, start, end int64) decimal.Decimal {
	balance := profileLineBalanceBefore(opening, events, start)
	cursor := start
	total := decimal.Zero
	for _, event := range events {
		if event.Timestamp < start {
			continue
		}
		if event.Timestamp >= end {
			break
		}
		if balance.IsPositive() {
			total = total.Add(balance.Mul(decimal.NewFromInt(event.Timestamp - cursor)))
		}
		balance = balance.Add(event.Amount)
		cursor = event.Timestamp
	}
	if balance.IsPositive() {
		total = total.Add(balance.Mul(decimal.NewFromInt(end - cursor)))
	}
	return total
}

func profileLineAmount(amount decimal.Decimal) string {
	return amount.Shift(-8).String()
}

func profileLineAPY(rewards, principalSeconds decimal.Decimal, duration int64) *float64 {
	if !principalSeconds.IsPositive() {
		return nil
	}
	// Compound return using the actual bucket duration, including All's final day.
	ratio, _ := rewards.Mul(decimal.NewFromInt(duration)).DivRound(principalSeconds, 32).Float64()
	if ratio < -1 {
		return nil
	}
	apy := math.Expm1(math.Log1p(ratio) * float64(365*profileLineDay) / float64(duration))
	if math.IsNaN(apy) || math.IsInf(apy, 0) {
		return nil // JSON cannot represent non-finite values.
	}
	return &apy
}
