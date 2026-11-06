package application

import (
	"errors"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

type fakeRepo struct {
	config lotterydomain.Config
	prizes []lotterydomain.Prize
	draws  []lotterydomain.Draw
	nextID uint
}

func (f *fakeRepo) GetConfig() (*lotterydomain.Config, error) {
	config := f.config
	return &config, nil
}

func (f *fakeRepo) SaveConfig(config *lotterydomain.Config) error {
	f.config = *config
	return nil
}

func (f *fakeRepo) ListPrizes(includeDisabled bool) ([]lotterydomain.Prize, error) {
	result := make([]lotterydomain.Prize, 0, len(f.prizes))
	for _, prize := range f.prizes {
		if !includeDisabled && !prize.Enabled {
			continue
		}
		result = append(result, prize)
	}
	return result, nil
}

func (f *fakeRepo) GetPrizeByID(id uint) (*lotterydomain.Prize, error) {
	for index := range f.prizes {
		if f.prizes[index].ID == id {
			prize := f.prizes[index]
			return &prize, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) CreatePrize(prize *lotterydomain.Prize) error {
	f.nextID++
	prize.ID = f.nextID
	f.prizes = append(f.prizes, *prize)
	return nil
}

func (f *fakeRepo) UpdatePrize(prize *lotterydomain.Prize) error {
	for index := range f.prizes {
		if f.prizes[index].ID == prize.ID {
			f.prizes[index] = *prize
			return nil
		}
	}
	return lotterydomain.ErrPrizeNotFound
}

func (f *fakeRepo) DeletePrize(id uint) error {
	for index := range f.prizes {
		if f.prizes[index].ID == id {
			f.prizes = append(f.prizes[:index], f.prizes[index+1:]...)
			return nil
		}
	}
	return lotterydomain.ErrPrizeNotFound
}

func (f *fakeRepo) ListDraws(filter lotterycontract.DrawListFilter) ([]lotterydomain.Draw, int64, error) {
	result := make([]lotterydomain.Draw, 0, len(f.draws))
	for _, draw := range f.draws {
		if filter.UserID > 0 && draw.UserID != filter.UserID {
			continue
		}
		if filter.OrderNo != "" && draw.OrderNo != filter.OrderNo {
			continue
		}
		result = append(result, draw)
	}
	return result, int64(len(result)), nil
}

func (f *fakeRepo) GetDrawByID(id uint) (*lotterydomain.Draw, error) {
	for index := range f.draws {
		if f.draws[index].ID == id {
			draw := f.draws[index]
			return &draw, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) UpdateDraw(draw *lotterydomain.Draw) error {
	for index := range f.draws {
		if f.draws[index].ID == draw.ID {
			f.draws[index] = *draw
			return nil
		}
	}
	return lotterydomain.ErrDrawNotFound
}

func (f *fakeRepo) CountDrawsByOrder(orderID uint) (int, error) {
	count := 0
	for _, draw := range f.draws {
		if draw.OrderID == orderID {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepo) CountDrawsByUserSince(userID uint, since time.Time) (int, error) {
	count := 0
	for _, draw := range f.draws {
		if draw.UserID == userID && !draw.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepo) SumWonPrizes(lotterycontract.DrawListFilter) (lotterycontract.DrawTotals, error) {
	return lotterycontract.DrawTotals{WonAmount: money.FromDecimal(decimal.Zero)}, nil
}

// fakeRunner 直接在 fakeRepo 上执行，模拟事务内的读写。
type fakeRunner struct {
	repo *fakeRepo
	// stockRace 为 true 时模拟"加锁后库存仍被抢空"。
	stockRace bool
	credits   []lotterycontract.WalletCreditInput
}

func (f *fakeRunner) WithinDrawTransaction(fn func(tx lotterycontract.DrawTransaction) error) error {
	return fn(f)
}

func (f *fakeRunner) ListPrizesForUpdate() ([]lotterydomain.Prize, error) {
	return f.repo.ListPrizes(false)
}

func (f *fakeRunner) ConsumePrizeStock(prizeID uint) (bool, error) {
	if f.stockRace {
		return false, nil
	}
	for index := range f.repo.prizes {
		if f.repo.prizes[index].ID != prizeID {
			continue
		}
		if f.repo.prizes[index].Stock > 0 {
			f.repo.prizes[index].Stock--
		}
		f.repo.prizes[index].AwardedCount++
		return true, nil
	}
	return false, nil
}

func (f *fakeRunner) CountDrawsByOrder(orderID uint) (int, error) {
	return f.repo.CountDrawsByOrder(orderID)
}

func (f *fakeRunner) CountDrawsByUserSince(userID uint, since time.Time) (int, error) {
	return f.repo.CountDrawsByUserSince(userID, since)
}

func (f *fakeRunner) CreateDraw(draw *lotterydomain.Draw) error {
	f.repo.nextID++
	draw.ID = f.repo.nextID
	draw.CreatedAt = time.Now()
	f.repo.draws = append(f.repo.draws, *draw)
	return nil
}

func (f *fakeRunner) CreditWallet(input lotterycontract.WalletCreditInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	f.credits = append(f.credits, input)
	txn := &walletdomain.Transaction{ID: 99}
	return &walletdomain.Account{UserID: input.UserID}, txn, nil
}

type fakeOrders struct {
	order *lotterycontract.OrderSnapshot
}

func (f *fakeOrders) GetUserOrderByOrderNo(orderNo string, userID uint) (*lotterycontract.OrderSnapshot, error) {
	if f.order == nil || f.order.OrderNo != orderNo || f.order.UserID != userID {
		return nil, nil
	}
	snapshot := *f.order
	return &snapshot, nil
}

func (f *fakeOrders) ListRecentUserOrders(userID uint, limit int) ([]lotterycontract.OrderSnapshot, error) {
	snapshots := make([]lotterycontract.OrderSnapshot, 0, 1)
	if f.order == nil || f.order.UserID != userID || limit <= 0 {
		return snapshots, nil
	}
	snapshots = append(snapshots, *f.order)
	return snapshots, nil
}

type fakeCurrency struct{}

func (fakeCurrency) SiteCurrency() string { return "CNY" }

func paidOrder() *lotterycontract.OrderSnapshot {
	paidAt := time.Now().Add(-time.Hour)
	return &lotterycontract.OrderSnapshot{
		ID:          7,
		OrderNo:     "ORD-7",
		UserID:      42,
		Status:      constants.OrderStatusPaid,
		TotalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		Currency:    "CNY",
		PaidAt:      &paidAt,
	}
}

func newTestService(repo *fakeRepo, runner *fakeRunner, orders *fakeOrders, roll int) *Service {
	return NewService(Options{
		Repository: repo,
		Runner:     runner,
		Orders:     orders,
		Currency:   fakeCurrency{},
		Roll:       func() int { return roll },
	})
}

func baseRepo() *fakeRepo {
	config := lotterydomain.DefaultConfig()
	config.Enabled = true
	config.DrawsPerOrder = 1
	config.ValidHours = 72
	return &fakeRepo{
		config: config,
		prizes: []lotterydomain.Prize{
			{
				ID:          1,
				Name:        "10 元余额",
				Type:        lotterydomain.PrizeTypeBalance,
				Probability: 3000,
				Stock:       5,
				Amount:      money.FromDecimal(decimal.NewFromInt(10)),
				SortOrder:   1,
				Enabled:     true,
			},
			{
				ID:          2,
				Name:        "实物周边",
				Type:        lotterydomain.PrizeTypeCustom,
				Probability: 1000,
				Stock:       lotterydomain.StockUnlimited,
				SortOrder:   2,
				Enabled:     true,
			},
		},
		nextID: 10,
	}
}

func TestDrawCreditsBalancePrizeAndConsumesChance(t *testing.T) {
	repo := baseRepo()
	runner := &fakeRunner{repo: repo}
	service := newTestService(repo, runner, &fakeOrders{order: paidOrder()}, 0)

	result, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7", ClientIP: "1.2.3.4"})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if !result.Draw.IsWin || result.Draw.PrizeID != 1 {
		t.Fatalf("expected prize 1 to win, got %+v", result.Draw)
	}
	if result.SliceIndex != 0 {
		t.Fatalf("slice index = %d, want 0", result.SliceIndex)
	}
	if result.Remaining != 0 {
		t.Fatalf("remaining = %d, want 0", result.Remaining)
	}
	if len(runner.credits) != 1 {
		t.Fatalf("expected one wallet credit, got %d", len(runner.credits))
	}
	if runner.credits[0].TxnType != constants.WalletTxnTypeLotteryPrize {
		t.Fatalf("wallet txn type = %s", runner.credits[0].TxnType)
	}
	if !runner.credits[0].Amount.Decimal.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("credited amount = %s, want 10", runner.credits[0].Amount.Decimal)
	}
	if repo.prizes[0].Stock != 4 || repo.prizes[0].AwardedCount != 1 {
		t.Fatalf("stock not consumed: %+v", repo.prizes[0])
	}
	if result.Draw.DeliveryStatus != lotterydomain.DeliveryStatusNone {
		t.Fatalf("balance prize must not need manual delivery, got %s", result.Draw.DeliveryStatus)
	}

	// 第二次抽奖应被次数限制拦住。
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrChanceExhausted) {
		t.Fatalf("second draw error = %v, want ErrChanceExhausted", err)
	}
}

func TestDrawMarksManualPrizePending(t *testing.T) {
	repo := baseRepo()
	runner := &fakeRunner{repo: repo}
	// roll 落在第二个奖品的区间 [3000, 4000)
	service := newTestService(repo, runner, &fakeOrders{order: paidOrder()}, 3500)

	result, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if result.Draw.PrizeID != 2 || result.SliceIndex != 1 {
		t.Fatalf("expected prize 2 at slice 1, got %+v / %d", result.Draw, result.SliceIndex)
	}
	if result.Draw.DeliveryStatus != lotterydomain.DeliveryStatusPending {
		t.Fatalf("delivery status = %s, want pending", result.Draw.DeliveryStatus)
	}
	if len(runner.credits) != 0 {
		t.Fatal("non-balance prize must not touch the wallet")
	}
}

func TestDrawFallsBackToMissWhenRollExceedsPrizeRanges(t *testing.T) {
	repo := baseRepo()
	runner := &fakeRunner{repo: repo}
	service := newTestService(repo, runner, &fakeOrders{order: paidOrder()}, 9999)

	result, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if result.Draw.IsWin || result.Draw.PrizeID != 0 {
		t.Fatalf("expected miss, got %+v", result.Draw)
	}
	// 兜底扇区固定排在所有启用奖品之后。
	if result.SliceIndex != 2 {
		t.Fatalf("slice index = %d, want 2 (miss slice)", result.SliceIndex)
	}
	if len(repo.draws) != 1 {
		t.Fatalf("miss must still be recorded, got %d draws", len(repo.draws))
	}
}

func TestDrawDegradesToMissWhenStockRacedOut(t *testing.T) {
	repo := baseRepo()
	runner := &fakeRunner{repo: repo, stockRace: true}
	service := newTestService(repo, runner, &fakeOrders{order: paidOrder()}, 0)

	result, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if result.Draw.IsWin {
		t.Fatal("stock race must degrade to miss instead of awarding the prize")
	}
	if len(runner.credits) != 0 {
		t.Fatal("no wallet credit may happen when the stock was lost")
	}
}

func TestDrawRejectsGuestAndForeignOrders(t *testing.T) {
	repo := baseRepo()
	runner := &fakeRunner{repo: repo}
	service := newTestService(repo, runner, &fakeOrders{order: paidOrder()}, 0)

	if _, err := service.Draw(DrawInput{UserID: 0, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrGuestNotAllowed) {
		t.Fatalf("guest draw error = %v, want ErrGuestNotAllowed", err)
	}
	if _, err := service.Draw(DrawInput{UserID: 43, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrOrderNotFound) {
		t.Fatalf("foreign order error = %v, want ErrOrderNotFound", err)
	}
}

func TestDrawRejectsUnpaidExpiredAndUnderThresholdOrders(t *testing.T) {
	repo := baseRepo()
	runner := &fakeRunner{repo: repo}

	pending := paidOrder()
	pending.Status = constants.OrderStatusPendingPayment
	pending.PaidAt = nil
	service := newTestService(repo, runner, &fakeOrders{order: pending}, 0)
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrOrderNotPaid) {
		t.Fatalf("unpaid order error = %v, want ErrOrderNotPaid", err)
	}

	canceled := paidOrder()
	canceled.Status = constants.OrderStatusCanceled
	service = newTestService(repo, runner, &fakeOrders{order: canceled}, 0)
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrOrderNotPaid) {
		t.Fatalf("canceled order error = %v, want ErrOrderNotPaid", err)
	}

	expiredRepo := baseRepo()
	expiredRepo.config.ValidHours = 1
	expired := paidOrder()
	paidAt := time.Now().Add(-3 * time.Hour)
	expired.PaidAt = &paidAt
	service = newTestService(expiredRepo, &fakeRunner{repo: expiredRepo}, &fakeOrders{order: expired}, 0)
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrChanceExpired) {
		t.Fatalf("expired order error = %v, want ErrChanceExpired", err)
	}

	thresholdRepo := baseRepo()
	thresholdRepo.config.MinOrderAmount = money.FromDecimal(decimal.NewFromInt(500))
	service = newTestService(thresholdRepo, &fakeRunner{repo: thresholdRepo}, &fakeOrders{order: paidOrder()}, 0)
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrOrderNotEligible) {
		t.Fatalf("under threshold error = %v, want ErrOrderNotEligible", err)
	}
}

func TestDrawRespectsDailyLimit(t *testing.T) {
	repo := baseRepo()
	repo.config.DrawsPerOrder = 5
	repo.config.DailyDrawLimit = 1
	runner := &fakeRunner{repo: repo}
	service := newTestService(repo, runner, &fakeOrders{order: paidOrder()}, 9999)

	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); err != nil {
		t.Fatalf("first draw: %v", err)
	}
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrDailyLimitReached) {
		t.Fatalf("second draw error = %v, want ErrDailyLimitReached", err)
	}
}

func TestDrawRejectedWhenActivityDisabledOrNoPrize(t *testing.T) {
	repo := baseRepo()
	repo.config.Enabled = false
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrDisabled) {
		t.Fatalf("disabled activity error = %v, want ErrDisabled", err)
	}

	emptyRepo := baseRepo()
	emptyRepo.prizes = nil
	service = newTestService(emptyRepo, &fakeRunner{repo: emptyRepo}, &fakeOrders{order: paidOrder()}, 0)
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrNoPrizeConfigured) {
		t.Fatalf("no prize error = %v, want ErrNoPrizeConfigured", err)
	}
}

func TestGetChanceReportsReasonInsteadOfError(t *testing.T) {
	repo := baseRepo()
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	view, err := service.GetChance(42, "ORD-7")
	if err != nil {
		t.Fatalf("get chance: %v", err)
	}
	if !view.Eligible || view.Reason != ReasonOK || view.Remaining != 1 {
		t.Fatalf("unexpected chance view: %+v", view)
	}

	missing, err := service.GetChance(42, "ORD-UNKNOWN")
	if err != nil {
		t.Fatalf("get chance for missing order: %v", err)
	}
	if missing.Eligible || missing.Reason != ReasonOrderNotFound {
		t.Fatalf("unexpected chance view for missing order: %+v", missing)
	}
}

// GetChance 必须和 Draw 对每日上限给出一致的结论，否则前台会显示一个
// 点下去必然报错的抽奖按钮。
func TestGetChanceReportsDailyLimitLikeDraw(t *testing.T) {
	repo := baseRepo()
	repo.config.DrawsPerOrder = 5
	repo.config.DailyDrawLimit = 1
	runner := &fakeRunner{repo: repo}
	service := newTestService(repo, runner, &fakeOrders{order: paidOrder()}, 9999)

	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); err != nil {
		t.Fatalf("first draw: %v", err)
	}

	view, err := service.GetChance(42, "ORD-7")
	if err != nil {
		t.Fatalf("get chance: %v", err)
	}
	if view.Eligible || view.Reason != ReasonDailyLimit {
		t.Fatalf("chance view = %+v, want eligible=false reason=%s", view, ReasonDailyLimit)
	}
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrDailyLimitReached) {
		t.Fatalf("draw error = %v, want ErrDailyLimitReached", err)
	}
}

func TestGetWheelAlwaysAppendsMissSlice(t *testing.T) {
	repo := baseRepo()
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	view, err := service.GetWheel()
	if err != nil {
		t.Fatalf("get wheel: %v", err)
	}
	if len(view.Slices) != 3 {
		t.Fatalf("expected 2 prizes + miss slice, got %d", len(view.Slices))
	}
	last := view.Slices[len(view.Slices)-1]
	if last.PrizeID != 0 || last.Name != MissSliceName {
		t.Fatalf("last slice must be the miss slice, got %+v", last)
	}
	for index, slice := range view.Slices {
		if slice.Index != index {
			t.Fatalf("slice %d carries index %d", index, slice.Index)
		}
	}
}

func TestPrizeProbabilityBudgetIsEnforced(t *testing.T) {
	repo := baseRepo()
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	// 已配置 3000 + 1000，再加 7000 就超过 10000。
	_, err := service.CreatePrize(PrizeInput{
		Name:        "超额奖品",
		Type:        lotterydomain.PrizeTypeNone,
		Probability: 7000,
		Stock:       lotterydomain.StockUnlimited,
		Enabled:     true,
	})
	if !errors.Is(err, lotterydomain.ErrProbabilityOverflow) {
		t.Fatalf("create prize error = %v, want ErrProbabilityOverflow", err)
	}

	if _, err := service.CreatePrize(PrizeInput{
		Name:        "余额奖品缺金额",
		Type:        lotterydomain.PrizeTypeBalance,
		Probability: 100,
		Stock:       lotterydomain.StockUnlimited,
		Enabled:     true,
	}); !errors.Is(err, lotterydomain.ErrPrizeInvalid) {
		t.Fatalf("balance prize without amount must be rejected, got %v", err)
	}
}

func TestMarkDeliveredOnlyAppliesToManualPrizes(t *testing.T) {
	repo := baseRepo()
	repo.draws = []lotterydomain.Draw{
		{ID: 1, UserID: 42, OrderID: 7, IsWin: true, PrizeType: lotterydomain.PrizeTypeCustom, DeliveryStatus: lotterydomain.DeliveryStatusPending},
		{ID: 2, UserID: 42, OrderID: 8, IsWin: true, PrizeType: lotterydomain.PrizeTypeBalance, DeliveryStatus: lotterydomain.DeliveryStatusNone},
	}
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	view, err := service.MarkDelivered(DeliverInput{DrawID: 1, AdminID: 3, Remark: "已寄出"})
	if err != nil {
		t.Fatalf("mark delivered: %v", err)
	}
	if view.DeliveryStatus != lotterydomain.DeliveryStatusDelivered || view.DeliveredAt == nil {
		t.Fatalf("unexpected delivered view: %+v", view)
	}

	if _, err := service.MarkDelivered(DeliverInput{DrawID: 2, AdminID: 3}); !errors.Is(err, lotterydomain.ErrDeliveryNotApplicable) {
		t.Fatalf("balance prize delivery error = %v, want ErrDeliveryNotApplicable", err)
	}

	reverted, err := service.MarkDelivered(DeliverInput{DrawID: 1, AdminID: 3, Revert: true})
	if err != nil {
		t.Fatalf("revert delivery: %v", err)
	}
	if reverted.DeliveryStatus != lotterydomain.DeliveryStatusPending || reverted.DeliveredAt != nil {
		t.Fatalf("unexpected reverted view: %+v", reverted)
	}
}

// 独立抽奖页靠这个列表选订单，必须和 Draw 的资格判定保持一致：
// 列出来的订单点下去就该能抽，抽完了就不该再出现。
func TestListDrawableOrders(t *testing.T) {
	repo := baseRepo()
	runner := &fakeRunner{repo: repo}
	service := newTestService(repo, runner, &fakeOrders{order: paidOrder()}, 9999)

	orders, err := service.ListDrawableOrders(42)
	if err != nil {
		t.Fatalf("list drawable orders: %v", err)
	}
	if len(orders) != 1 || orders[0].OrderNo != "ORD-7" || orders[0].Remaining != 1 {
		t.Fatalf("unexpected drawable orders: %+v", orders)
	}

	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); err != nil {
		t.Fatalf("draw: %v", err)
	}
	after, err := service.ListDrawableOrders(42)
	if err != nil {
		t.Fatalf("list after draw: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("order with no chance left must drop out, got %+v", after)
	}
}

func TestListDrawableOrdersEmptyWhenDisabledOrGuest(t *testing.T) {
	repo := baseRepo()
	repo.config.Enabled = false
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)
	orders, err := service.ListDrawableOrders(42)
	if err != nil {
		t.Fatalf("list with disabled activity: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("disabled activity must expose no drawable order, got %+v", orders)
	}

	enabled := baseRepo()
	service = newTestService(enabled, &fakeRunner{repo: enabled}, &fakeOrders{order: paidOrder()}, 0)
	guestOrders, err := service.ListDrawableOrders(0)
	if err != nil {
		t.Fatalf("list for anonymous: %v", err)
	}
	if len(guestOrders) != 0 {
		t.Fatalf("anonymous user must get nothing, got %+v", guestOrders)
	}
}

func TestListDrawableOrdersSkipsIneligibleOrders(t *testing.T) {
	repo := baseRepo()
	repo.config.MinOrderAmount = money.FromDecimal(decimal.NewFromInt(500))
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	orders, err := service.ListDrawableOrders(42)
	if err != nil {
		t.Fatalf("list drawable orders: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("order below the threshold must not be listed, got %+v", orders)
	}
}

// 活动开始时间把历史订单挡在外面：没有这道闸门，活动一上线全库已付款订单
// 会同时获得抽奖资格（有效期填 0 时尤其危险）。
func TestActivityStartTimeExcludesOlderOrders(t *testing.T) {
	repo := baseRepo()
	repo.config.ValidHours = 0 // 长期有效，确保拦截来自开始时间而不是有效期
	startsAt := time.Now().Add(-30 * time.Minute)
	repo.config.StartsAt = &startsAt

	// 订单支付于 1 小时前，早于活动开始时间
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	view, err := service.GetChance(42, "ORD-7")
	if err != nil {
		t.Fatalf("get chance: %v", err)
	}
	if view.Eligible || view.Reason != ReasonNotStarted {
		t.Fatalf("chance view = %+v, want eligible=false reason=%s", view, ReasonNotStarted)
	}
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); !errors.Is(err, lotterydomain.ErrActivityNotStarted) {
		t.Fatalf("draw error = %v, want ErrActivityNotStarted", err)
	}
	orders, err := service.ListDrawableOrders(42)
	if err != nil {
		t.Fatalf("list drawable orders: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("order paid before the activity must not be listed, got %+v", orders)
	}
}

func TestActivityStartTimeAllowsLaterOrders(t *testing.T) {
	repo := baseRepo()
	startsAt := time.Now().Add(-2 * time.Hour) // 早于订单支付时间（1 小时前）
	repo.config.StartsAt = &startsAt
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	view, err := service.GetChance(42, "ORD-7")
	if err != nil {
		t.Fatalf("get chance: %v", err)
	}
	if !view.Eligible || view.Reason != ReasonOK {
		t.Fatalf("chance view = %+v, want eligible", view)
	}
	if _, err := service.Draw(DrawInput{UserID: 42, OrderNo: "ORD-7"}); err != nil {
		t.Fatalf("draw: %v", err)
	}
}

func TestEmptyStartTimeKeepsEveryPaidOrderEligible(t *testing.T) {
	repo := baseRepo()
	repo.config.ValidHours = 0
	repo.config.StartsAt = nil
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	view, err := service.GetChance(42, "ORD-7")
	if err != nil {
		t.Fatalf("get chance: %v", err)
	}
	if !view.Eligible {
		t.Fatalf("empty start time must not block anything, got %+v", view)
	}
}

// 活动从关到开时自动填开始时间，避免"顺手开一下"就把全部历史订单放进来。
func TestEnablingActivityDefaultsStartTimeToNow(t *testing.T) {
	repo := baseRepo()
	repo.config.Enabled = false
	repo.config.StartsAt = nil
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	before := time.Now()
	saved, err := service.SaveConfig(ConfigInput{
		Enabled:       true,
		DrawsPerOrder: 1,
		ValidHours:    72,
	})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	if saved.StartsAt == nil {
		t.Fatal("enabling the activity must default the start time instead of leaving it open")
	}
	if saved.StartsAt.Before(before.Add(-time.Minute)) {
		t.Fatalf("default start time = %v, want roughly now", saved.StartsAt)
	}

	// 活动已经开着时清空开始时间，是管理员的明确选择，不再兜底。
	cleared, err := service.SaveConfig(ConfigInput{
		Enabled:       true,
		DrawsPerOrder: 1,
		ValidHours:    72,
	})
	if err != nil {
		t.Fatalf("save config again: %v", err)
	}
	if cleared.StartsAt != nil {
		t.Fatalf("clearing the start time on an already-enabled activity must be respected, got %v", cleared.StartsAt)
	}
}

// 管理员显式指定开始时间时，不能被兜底逻辑覆盖。
func TestEnablingActivityKeepsExplicitStartTime(t *testing.T) {
	repo := baseRepo()
	repo.config.Enabled = false
	service := newTestService(repo, &fakeRunner{repo: repo}, &fakeOrders{order: paidOrder()}, 0)

	explicit := time.Now().Add(-72 * time.Hour)
	saved, err := service.SaveConfig(ConfigInput{
		Enabled:       true,
		StartsAt:      &explicit,
		DrawsPerOrder: 1,
		ValidHours:    72,
	})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	if saved.StartsAt == nil || !saved.StartsAt.Equal(explicit) {
		t.Fatalf("explicit start time = %v, want %v", saved.StartsAt, explicit)
	}
}
