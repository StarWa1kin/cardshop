package domain

import (
	"testing"

	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func prize(id uint, probability, stock, sortOrder int, enabled bool) Prize {
	return Prize{
		ID:          id,
		Name:        "prize",
		Type:        PrizeTypeBalance,
		Probability: probability,
		Stock:       stock,
		SortOrder:   sortOrder,
		Enabled:     enabled,
		Amount:      money.FromDecimal(decimal.NewFromInt(1)),
	}
}

func TestPickWalksProbabilityRangesInSortOrder(t *testing.T) {
	prizes := SortPrizes([]Prize{
		prize(2, 2000, StockUnlimited, 2, true),
		prize(1, 1000, StockUnlimited, 1, true),
	})

	cases := []struct {
		roll   int
		wantID uint
	}{
		{roll: 0, wantID: 1},
		{roll: 999, wantID: 1},
		{roll: 1000, wantID: 2},
		{roll: 2999, wantID: 2},
		{roll: 3000, wantID: 0},
		{roll: ProbabilityScale - 1, wantID: 0},
	}
	for _, testCase := range cases {
		hit := Pick(prizes, testCase.roll)
		if testCase.wantID == 0 {
			if hit != nil {
				t.Fatalf("roll %d: expected miss, got prize %d", testCase.roll, hit.ID)
			}
			continue
		}
		if hit == nil || hit.ID != testCase.wantID {
			t.Fatalf("roll %d: expected prize %d, got %v", testCase.roll, testCase.wantID, hit)
		}
	}
}

func TestPickSkipsSoldOutAndDisabledPrizes(t *testing.T) {
	prizes := SortPrizes([]Prize{
		prize(1, 5000, 0, 1, true),  // 售罄
		prize(2, 5000, 3, 2, false), // 已停用
		prize(3, 1000, 1, 3, true),
	})

	if hit := Pick(prizes, 0); hit == nil || hit.ID != 3 {
		t.Fatalf("expected the only drawable prize to win, got %v", hit)
	}
	// 售罄与停用奖品的概率归入未中奖，不会被分给剩下的奖品。
	if hit := Pick(prizes, 1000); hit != nil {
		t.Fatalf("expected miss once drawable probability is exhausted, got prize %d", hit.ID)
	}
	if total := DrawableProbability(prizes); total != 1000 {
		t.Fatalf("drawable probability = %d, want 1000", total)
	}
	if total := ConfiguredProbability(prizes); total != 6000 {
		t.Fatalf("configured probability = %d, want 6000 (disabled prize excluded)", total)
	}
}

func TestPickRejectsOutOfRangeRoll(t *testing.T) {
	prizes := []Prize{prize(1, ProbabilityScale, StockUnlimited, 1, true)}
	if hit := Pick(prizes, ProbabilityScale); hit != nil {
		t.Fatal("roll must be below ProbabilityScale")
	}
	if hit := Pick(prizes, -1); hit != nil {
		t.Fatal("negative roll must not win")
	}
}

func TestHasDrawablePrize(t *testing.T) {
	if HasDrawablePrize([]Prize{prize(1, 0, StockUnlimited, 1, true)}) {
		t.Fatal("zero probability prize is not drawable")
	}
	if !HasDrawablePrize([]Prize{prize(1, 1, StockUnlimited, 1, true)}) {
		t.Fatal("prize with probability and stock must be drawable")
	}
}

func TestRollStaysInRange(t *testing.T) {
	for index := 0; index < 200; index++ {
		value := Roll()
		if value < 0 || value >= ProbabilityScale {
			t.Fatalf("roll %d out of range", value)
		}
	}
}

func TestNormalizeConfigClampsOutOfRangeValues(t *testing.T) {
	config := NormalizeConfig(Config{
		DrawsPerOrder:  0,
		DailyDrawLimit: -5,
		ValidHours:     -1,
		MinOrderAmount: money.FromDecimal(decimal.NewFromInt(-10)),
	})
	defaults := DefaultConfig()
	if config.DrawsPerOrder != defaults.DrawsPerOrder {
		t.Fatalf("draws per order = %d, want %d", config.DrawsPerOrder, defaults.DrawsPerOrder)
	}
	if config.DailyDrawLimit != defaults.DailyDrawLimit {
		t.Fatalf("daily limit = %d, want %d", config.DailyDrawLimit, defaults.DailyDrawLimit)
	}
	if config.ValidHours != defaults.ValidHours {
		t.Fatalf("valid hours = %d, want %d", config.ValidHours, defaults.ValidHours)
	}
	if !config.MinOrderAmount.Decimal.Equal(decimal.Zero) {
		t.Fatalf("negative threshold must fall back to 0, got %s", config.MinOrderAmount.Decimal)
	}
	if config.ID != ConfigSingletonID {
		t.Fatalf("config id = %d, want %d", config.ID, ConfigSingletonID)
	}
}

func TestInitialDeliveryStatusDependsOnPrizeType(t *testing.T) {
	cases := map[string]string{
		PrizeTypeNone:    DeliveryStatusNone,
		PrizeTypeBalance: DeliveryStatusNone,
		PrizeTypeCoupon:  DeliveryStatusNone,
		PrizeTypeProduct: DeliveryStatusPending,
		PrizeTypeCustom:  DeliveryStatusPending,
	}
	for prizeType, want := range cases {
		if got := InitialDeliveryStatus(prizeType); got != want {
			t.Fatalf("%s: delivery status = %s, want %s", prizeType, got, want)
		}
	}
}
