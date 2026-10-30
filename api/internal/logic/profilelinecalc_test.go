package logic

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"cuniBTCReward/api/internal/types"

	"github.com/shopspring/decimal"
)

func profileTestAmount(value string) decimal.Decimal {
	return decimal.RequireFromString(value).Shift(8)
}

func TestProfileLineRanges(t *testing.T) {
	for _, tc := range []struct {
		value        string
		days         int64
		points       int
		lastDuration int64
	}{
		{"24H", 1, 24, 3600}, {"7D", 7, 7, profileLineDay},
		{"30D", 30, 30, profileLineDay}, {"All", 365, 53, profileLineDay},
	} {
		t.Run(tc.value, func(t *testing.T) {
			const end int64 = 1800000123
			start, step, err := profileLineRange(tc.value, end)
			if err != nil || start != end-tc.days*profileLineDay {
				t.Fatalf("range: %d %d %v", start, step, err)
			}
			resp := buildProfileLine("b2uniBTC", tc.value, start, end, step, decimal.Zero, nil, nil, nil)
			if len(resp.Data) != tc.points || resp.StartTime != start || resp.EndTime != end {
				t.Fatalf("unexpected boundaries/count: %+v", resp)
			}
			last := resp.Data[len(resp.Data)-1]
			if last.EndTime-last.StartTime != tc.lastDuration || last.EndTime != end {
				t.Fatalf("last bucket: %+v", last)
			}
			for i, point := range resp.Data {
				if point.Apy != nil || point.Deposited != "0" || point.Rewards != "0" {
					t.Fatalf("empty point: %+v", point)
				}
				if i > 0 && resp.Data[i-1].EndTime != point.StartTime {
					t.Fatal("buckets must be contiguous")
				}
			}
		})
	}
	if _, _, err := profileLineRange("all", 1800000123); err == nil {
		t.Fatal("invalid range accepted")
	}
}

func TestProfileLineDepositsAndWeightedPrincipal(t *testing.T) {
	events := []profileLineEvent{
		{Timestamp: 100, Amount: profileTestAmount("1"), Deposited: profileTestAmount("1")},
		{Timestamp: 150, Amount: profileTestAmount("2"), Deposited: profileTestAmount("2")},
		{Timestamp: 175, Amount: profileTestAmount("-1")},
		{Timestamp: 200, Amount: profileTestAmount("4"), Deposited: profileTestAmount("4")},
		{Timestamp: 300, Amount: profileTestAmount("999"), Deposited: profileTestAmount("999")},
	}
	extras := []profileLineEvent{
		{Timestamp: 100, Amount: profileTestAmount("0.001")},
		{Timestamp: 200, Amount: profileTestAmount("0.002")},
		{Timestamp: 300, Amount: profileTestAmount("999")},
	}
	resp := buildProfileLine("b2uniBTC", "24H", 100, 300, 100, decimal.Zero, events, extras, nil)
	if resp.Deposited != "7" || resp.Data[0].Deposited != "3" || resp.Data[1].Deposited != "4" {
		t.Fatalf("deposit totals/boundaries: %+v", resp)
	}
	if resp.Data[0].EffectivePrincipal != "1.75" || resp.Data[1].EffectivePrincipal != "6" {
		t.Fatalf("time-weighted principal: %+v", resp.Data)
	}
	if resp.Data[0].AmountExtra != "0.001" || resp.Data[1].AmountExtra != "0.002" {
		t.Fatalf("extra boundaries: %+v", resp.Data)
	}
	if resp.Data[0].Rewards != "0.001" || resp.Data[1].Rewards != "0.002" {
		t.Fatal("extra rewards must be counted exactly once")
	}
}

func TestProfileLineBaseAllocationAndCompleteness(t *testing.T) {
	epochs := []profileLineEpoch{
		{OperateStart: 0, LockupStart: 0, LockupPeriod: 7, AmountBase: profileTestAmount("1"), Generated: true},
		{OperateStart: 7, LockupStart: 7, LockupPeriod: 7, Generated: false},
	}
	resp := buildProfileLine("b2uniBTC", "24H", 0, 14, 3, profileTestAmount("10"), nil, nil, epochs)
	total := decimal.Zero
	for _, point := range resp.Data {
		total = total.Add(decimal.RequireFromString(point.AmountBase))
		if point.BaseRewardComplete != (point.EndTime <= 7) {
			t.Fatalf("missing base reward status: %+v", point)
		}
		if point.Rewards != point.AmountBase {
			t.Fatal("total rewards must equal base + extra")
		}
	}
	if !total.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("allocation lost rounding remainder: %s", total)
	}
	partial := buildProfileLine("b2uniBTC", "24H", 2, 5, 3, profileTestAmount("10"), nil, nil, epochs)
	expected := decimal.NewFromInt(5).DivRound(decimal.NewFromInt(7), 24).
		Sub(decimal.NewFromInt(2).DivRound(decimal.NewFromInt(7), 24))
	actual := decimal.RequireFromString(partial.Data[0].AmountBase)
	if actual.Sub(expected).Abs().GreaterThan(decimal.New(1, -24)) {
		t.Fatalf("clipped epoch allocation: %s, expected %s", actual, expected)
	}
}

func TestProfileLineNoLockEarnsThroughoutEpoch(t *testing.T) {
	epochs := []profileLineEpoch{{OperateStart: 0, LockupStart: 100, LockupPeriod: 100,
		AmountBase: profileTestAmount("0.01"), Generated: true}}
	events := []profileLineEvent{
		{Timestamp: 50, Amount: profileTestAmount("2"), Deposited: profileTestAmount("2")},
		{Timestamp: 100, Amount: profileTestAmount("3"), Deposited: profileTestAmount("3")},
		{Timestamp: 150, Amount: profileTestAmount("5"), Deposited: profileTestAmount("5")},
	}
	resp := buildProfileLine("b2uniBTC", "24H", 0, 200, 100, decimal.Zero, events, nil, epochs)
	if resp.Data[0].EffectivePrincipal != "1" || resp.Data[0].AmountBase != "0.005" {
		t.Fatalf("noLock must earn during operate period: %+v", resp.Data[0])
	}
	if resp.Data[1].EffectivePrincipal != "7.5" || resp.Data[1].AmountBase != "0.005" || resp.Deposited != "10" {
		t.Fatalf("noLock must use changing balances throughout epoch: %+v", resp)
	}
}

func TestProfileLineAPY(t *testing.T) {
	principal := profileTestAmount("100")
	for _, seconds := range []int64{3600, profileLineDay, 7 * profileLineDay} {
		apy := profileLineAPY(profileTestAmount("0.01"), principal.Mul(decimal.NewFromInt(seconds)), seconds)
		want := math.Expm1(math.Log1p(0.0001) * float64(365*profileLineDay) / float64(seconds))
		if apy == nil || math.Abs(*apy-want) > 1e-12 {
			t.Fatalf("APY for %ds: %v, expected %v", seconds, apy, want)
		}
	}
	if profileLineAPY(principal, decimal.Zero, profileLineDay) != nil {
		t.Fatal("zero principal requires null APY")
	}
	if apy := profileLineAPY(decimal.Zero, principal, profileLineDay); apy == nil || *apy != 0 {
		t.Fatal("zero return with positive principal requires zero APY")
	}
	if profileLineAPY(principal, decimal.NewFromInt(1), 1) != nil {
		t.Fatal("overflow requires null APY")
	}
}

func TestProfileLineJSONContract(t *testing.T) {
	resp := buildProfileLine("b2uniBTC", "24H", 0, 3600, 3600, decimal.Zero, nil, nil, nil)
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"range":"24H","deposited":"0"`) || !strings.Contains(string(encoded), `"apy":null`) {
		t.Fatalf("unexpected response JSON: %s", encoded)
	}
}

func TestProfileLineOverlappingEpochs(t *testing.T) {
	epochs := []profileLineEpoch{
		{OperateStart: 50, LockupStart: 50, LockupPeriod: 100, AmountBase: profileTestAmount("1"), Generated: true},
		{OperateStart: 150, LockupStart: 150, LockupPeriod: 100, AmountBase: profileTestAmount("2"), Generated: true},
	}
	extras := []profileLineEvent{{Timestamp: 150, Amount: profileTestAmount("0.1")}}
	resp := buildProfileLine("b2uniBTC", "24H", 100, 200, 100, profileTestAmount("10"), nil, extras, epochs)
	point := resp.Data[0]
	if point.AmountBase != "1.5" || point.AmountExtra != "0.1" || point.Rewards != "1.6" || !point.BaseRewardComplete {
		t.Fatalf("cross-epoch allocation: %+v", point)
	}
}

func TestProfileLineAllFinalDayAPY(t *testing.T) {
	end := 365 * profileLineDay
	epochs := []profileLineEpoch{{LockupPeriod: end, AmountBase: profileTestAmount("3.65"), Generated: true}}
	resp := buildProfileLine("b2uniBTC", "All", 0, end, 7*profileLineDay, profileTestAmount("100"), nil, nil, epochs)
	last := resp.Data[len(resp.Data)-1]
	want := math.Expm1(math.Log1p(0.0001) * 365)
	if last.AmountBase != "0.01" || last.Apy == nil || math.Abs(*last.Apy-want) > 1e-12 {
		t.Fatalf("final partial week's daily APY: %+v", last)
	}
}

func TestProfileLineInvalidRequest(t *testing.T) {
	logic := NewProfileLineLogic(t.Context(), nil)
	for _, req := range []types.ProfileLineReq{{Symbol: "b2uniBTC", Range: "1D"}, {Range: "7D"}} {
		if _, err := logic.ProfileLine(&req); err == nil {
			t.Fatalf("invalid request accepted: %+v", req)
		}
	}
}

func TestProfileLineRejectsLockedStrategies(t *testing.T) {
	logic := NewProfileLineLogic(t.Context(), nil)
	for _, symbol := range []string{"cuniBTC", "suniBTC", "b1uniBTC", "unknown"} {
		_, err := logic.ProfileLine(&types.ProfileLineReq{Symbol: symbol, Range: "7D"})
		if err == nil || !strings.Contains(err.Error(), "only supports noLock") {
			t.Fatalf("expected noLock validation error for %s, got %v", symbol, err)
		}
	}
}
