package application

import (
	"errors"
	"fmt"
	"time"

	"github.com/dujiao-next/internal/constants"
	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// drawEligibleOrderStatuses 是可以参与抽奖的订单状态。
//
// 判定依据是"钱真的收到了且没有整单退回"：待支付、已取消、已整单退款都不给抽，
// 部分退款保留资格，因为用户确实为这笔订单付过钱。
var drawEligibleOrderStatuses = map[string]struct{}{
	constants.OrderStatusPaid:               {},
	constants.OrderStatusFulfilling:         {},
	constants.OrderStatusPartiallyDelivered: {},
	constants.OrderStatusPartiallyRefunded:  {},
	constants.OrderStatusDelivered:          {},
	constants.OrderStatusCompleted:          {},
}

// DrawInput 是一次抽奖请求。
type DrawInput struct {
	UserID   uint
	OrderNo  string
	ClientIP string
}

// GetWheel 返回前台转盘视图。
func (s *Service) GetWheel() (*WheelView, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrFetchFailed
	}
	config, err := s.repo.GetConfig()
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, lotterydomain.ErrFetchFailed
	}
	prizes, err := s.repo.ListPrizes(false)
	if err != nil {
		return nil, err
	}
	return &WheelView{
		Enabled:        config.Enabled,
		Title:          config.Title,
		Notice:         config.Notice,
		Currency:       s.siteCurrency(),
		MinOrderAmount: config.MinOrderAmount,
		DrawsPerOrder:  config.DrawsPerOrder,
		DailyDrawLimit: config.DailyDrawLimit,
		ValidHours:     config.ValidHours,
		Slices:         buildSlices(prizes),
	}, nil
}

// GetChance 返回某笔订单的抽奖资格与已有结果。
//
// 这里不返回错误码而是返回 Reason：前台需要在订单详情上解释"为什么不能抽"，
// 把它当成读接口比让它抛一串业务错误更好用。
func (s *Service) GetChance(userID uint, orderNo string) (*ChanceView, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrFetchFailed
	}
	view := &ChanceView{OrderNo: orderNo, Reason: ReasonDisabled, Draws: []DrawView{}}
	if userID == 0 {
		view.Reason = ReasonOrderNotFound
		return view, nil
	}

	config, err := s.repo.GetConfig()
	if err != nil {
		return nil, err
	}
	if config == nil || !config.Enabled {
		return view, nil
	}
	view.Total = config.DrawsPerOrder

	order, err := s.loadOrder(orderNo, userID)
	if err != nil {
		if errors.Is(err, lotterydomain.ErrOrderNotFound) || errors.Is(err, lotterydomain.ErrGuestNotAllowed) {
			view.Reason = ReasonOrderNotFound
			return view, nil
		}
		return nil, err
	}

	draws, _, err := s.repo.ListDraws(lotterycontract.DrawListFilter{
		UserID:   userID,
		OrderNo:  order.OrderNo,
		Page:     1,
		PageSize: lotterydomain.MaxDrawsPerOrder,
	})
	if err != nil {
		return nil, err
	}
	for index := range draws {
		if draws[index].OrderID != order.ID {
			continue
		}
		view.Draws = append(view.Draws, newDrawView(draws[index]))
	}
	view.Used = len(view.Draws)
	view.Remaining = config.DrawsPerOrder - view.Used
	if view.Remaining < 0 {
		view.Remaining = 0
	}
	if deadline := config.DrawDeadline(order.PaidAt); deadline != nil {
		view.ExpiresAt = deadline
	}

	now := time.Now()
	view.Reason = s.evaluateEligibility(config, order, view.Used, now)
	if view.Reason == ReasonOK && config.DailyDrawLimit > 0 {
		// 每日上限在抽奖事务里才是权威判定，但这里也要算一遍：
		// 否则前台会显示一个点下去必然报错的"立即抽奖"按钮。
		dailyUsed, dailyErr := s.repo.CountDrawsByUserSince(userID, startOfDay(now))
		if dailyErr != nil {
			return nil, dailyErr
		}
		if dailyUsed >= config.DailyDrawLimit {
			view.Reason = ReasonDailyLimit
		}
	}
	if view.Reason == ReasonOK {
		prizes, prizeErr := s.repo.ListPrizes(false)
		if prizeErr != nil {
			return nil, prizeErr
		}
		if !lotterydomain.HasDrawablePrize(prizes) {
			view.Reason = ReasonNoPrize
		}
	}
	view.Eligible = view.Reason == ReasonOK
	return view, nil
}

// Draw 执行一次抽奖。
func (s *Service) Draw(input DrawInput) (*DrawResult, error) {
	if s == nil || s.repo == nil || s.runner == nil {
		return nil, lotterydomain.ErrDrawFailed
	}
	if input.UserID == 0 {
		// 未登录请求本来就到不了这里（路由在用户鉴权组内），兜底拒绝。
		return nil, lotterydomain.ErrGuestNotAllowed
	}
	config, err := s.repo.GetConfig()
	if err != nil {
		return nil, err
	}
	if config == nil || !config.Enabled {
		return nil, lotterydomain.ErrDisabled
	}
	order, err := s.loadOrder(input.OrderNo, input.UserID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	switch s.evaluateEligibility(config, order, 0, now) {
	case ReasonNotPaid:
		return nil, lotterydomain.ErrOrderNotPaid
	case ReasonNotStarted:
		return nil, lotterydomain.ErrActivityNotStarted
	case ReasonAmountTooLow:
		return nil, lotterydomain.ErrOrderNotEligible
	case ReasonExpired:
		return nil, lotterydomain.ErrChanceExpired
	}

	currency := s.siteCurrency()
	var (
		record     lotterydomain.Draw
		sliceIndex int
		remaining  int
	)
	err = s.runner.WithinDrawTransaction(func(tx lotterycontract.DrawTransaction) error {
		used, countErr := tx.CountDrawsByOrder(order.ID)
		if countErr != nil {
			return countErr
		}
		if used >= config.DrawsPerOrder {
			return lotterydomain.ErrChanceExhausted
		}
		if config.DailyDrawLimit > 0 {
			dailyUsed, dailyErr := tx.CountDrawsByUserSince(input.UserID, startOfDay(now))
			if dailyErr != nil {
				return dailyErr
			}
			if dailyUsed >= config.DailyDrawLimit {
				return lotterydomain.ErrDailyLimitReached
			}
		}

		prizes, prizeErr := tx.ListPrizesForUpdate()
		if prizeErr != nil {
			return prizeErr
		}
		prizes = lotterydomain.SortPrizes(prizes)
		if !lotterydomain.HasDrawablePrize(prizes) {
			return lotterydomain.ErrNoPrizeConfigured
		}

		hit := lotterydomain.Pick(prizes, s.roll())
		if hit != nil {
			consumed, stockErr := tx.ConsumePrizeStock(hit.ID)
			if stockErr != nil {
				return stockErr
			}
			if !consumed {
				// 库存在加锁之后仍被抢空（例如并发或库存被后台改小），
				// 降级为未中奖而不是报错：这次机会已经消耗，必须留下记录。
				hit = nil
			}
		}

		draft := s.buildDraw(order, input, hit, used+1, currency)
		if hit != nil && hit.Type == lotterydomain.PrizeTypeBalance {
			_, txn, creditErr := tx.CreditWallet(lotterycontract.WalletCreditInput{
				UserID:    input.UserID,
				Amount:    hit.Amount,
				Currency:  currency,
				TxnType:   constants.WalletTxnTypeLotteryPrize,
				Reference: fmt.Sprintf("lottery:%d:%d", order.ID, used+1),
				Remark:    fmt.Sprintf("抽奖奖励：%s", hit.Name),
				OrderID:   &order.ID,
			})
			if creditErr != nil {
				return creditErr
			}
			if txn != nil && txn.ID > 0 {
				draft.WalletTxnID = &txn.ID
			}
		}
		if createErr := tx.CreateDraw(&draft); createErr != nil {
			// (order_id, seq) 唯一索引命中，说明同一订单被并发抽了两次。
			return lotterydomain.ErrChanceExhausted
		}

		record = draft
		sliceIndex = sliceIndexOf(prizes, draft.PrizeID)
		remaining = config.DrawsPerOrder - (used + 1)
		if remaining < 0 {
			remaining = 0
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &DrawResult{
		Draw:       newDrawView(record),
		SliceIndex: sliceIndex,
		Remaining:  remaining,
	}, nil
}

// buildDraw 组装抽奖记录。未中奖同样写一条，次数核销靠它。
func (s *Service) buildDraw(
	order *lotterycontract.OrderSnapshot,
	input DrawInput,
	hit *lotterydomain.Prize,
	seq int,
	currency string,
) lotterydomain.Draw {
	draw := lotterydomain.Draw{
		UserID:         input.UserID,
		OrderID:        order.ID,
		OrderNo:        order.OrderNo,
		Seq:            seq,
		PrizeType:      lotterydomain.PrizeTypeNone,
		Amount:         money.FromDecimal(decimal.Zero),
		Currency:       currency,
		DeliveryStatus: lotterydomain.DeliveryStatusNone,
		ClientIP:       input.ClientIP,
	}
	if hit == nil {
		return draw
	}
	draw.PrizeID = hit.ID
	draw.PrizeName = hit.Name
	draw.PrizeType = hit.Type
	draw.IsWin = lotterydomain.IsWinning(hit.Type)
	draw.DeliveryStatus = lotterydomain.InitialDeliveryStatus(hit.Type)

	switch hit.Type {
	case lotterydomain.PrizeTypeBalance:
		draw.Amount = hit.Amount
	case lotterydomain.PrizeTypeCoupon:
		draw.CouponID = hit.CouponID
		if hit.CouponID != nil && s.coupons != nil {
			if code, ok := s.coupons.ResolveCoupon(*hit.CouponID); ok {
				draw.CouponCode = code
			} else if code != "" {
				// 券已停用：码仍然快照下来，并转人工处理，别让用户空手而归。
				draw.CouponCode = code
				draw.DeliveryStatus = lotterydomain.DeliveryStatusPending
			} else {
				draw.DeliveryStatus = lotterydomain.DeliveryStatusPending
			}
		}
	case lotterydomain.PrizeTypeProduct:
		draw.ProductID = hit.ProductID
		if hit.ProductID != nil && s.products != nil {
			if name, ok := s.products.ResolveProductName(*hit.ProductID); ok {
				draw.ProductName = name
			}
		}
	}
	return draw
}

// loadOrder 读取订单；取不到就是不存在、不属于该用户，或者是游客订单。
func (s *Service) loadOrder(orderNo string, userID uint) (*lotterycontract.OrderSnapshot, error) {
	if s.orders == nil {
		return nil, lotterydomain.ErrFetchFailed
	}
	order, err := s.orders.GetUserOrderByOrderNo(orderNo, userID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, lotterydomain.ErrOrderNotFound
	}
	if order.UserID == 0 {
		return nil, lotterydomain.ErrGuestNotAllowed
	}
	return order, nil
}

// evaluateEligibility 判定订单资格。used 传 0 表示不检查已用次数。
func (s *Service) evaluateEligibility(
	config *lotterydomain.Config,
	order *lotterycontract.OrderSnapshot,
	used int,
	now time.Time,
) string {
	if _, ok := drawEligibleOrderStatuses[order.Status]; !ok || order.PaidAt == nil {
		return ReasonNotPaid
	}
	if !config.StartedFor(order.PaidAt) {
		return ReasonNotStarted
	}
	if order.TotalAmount.Decimal.Round(2).LessThan(config.MinOrderAmount.Decimal.Round(2)) {
		return ReasonAmountTooLow
	}
	if deadline := config.DrawDeadline(order.PaidAt); deadline != nil && now.After(*deadline) {
		return ReasonExpired
	}
	if used > 0 && used >= config.DrawsPerOrder {
		return ReasonExhausted
	}
	return ReasonOK
}

// buildSlices 组装转盘扇区：启用奖品按配置顺序排列，最后固定追加一个兜底扇区。
//
// 兜底扇区始终存在，因为奖品售罄后它的概率会落到"未中奖"，
// 指针总得有地方可停；总概率配满 100% 且无售罄时，它只是永远不会被命中。
func buildSlices(prizes []lotterydomain.Prize) []WheelSlice {
	sorted := lotterydomain.SortPrizes(prizes)
	slices := make([]WheelSlice, 0, len(sorted)+1)
	for index := range sorted {
		prize := sorted[index]
		if !prize.Enabled {
			continue
		}
		slices = append(slices, WheelSlice{
			Index:       len(slices),
			PrizeID:     prize.ID,
			Name:        prize.Name,
			Type:        prize.Type,
			Icon:        prize.Icon,
			Color:       prize.Color,
			Description: prize.Description,
			Amount:      prize.Amount,
			SoldOut:     !prize.HasStock(),
		})
	}
	slices = append(slices, WheelSlice{
		Index:   len(slices),
		PrizeID: 0,
		Name:    MissSliceName,
		Type:    lotterydomain.PrizeTypeNone,
		Amount:  money.FromDecimal(decimal.Zero),
		SoldOut: false,
	})
	return slices
}

// sliceIndexOf 返回奖品在转盘上的扇区下标，未中奖时指向最后的兜底扇区。
func sliceIndexOf(prizes []lotterydomain.Prize, prizeID uint) int {
	cursor := 0
	for index := range prizes {
		if !prizes[index].Enabled {
			continue
		}
		if prizes[index].ID == prizeID && prizeID != 0 {
			return cursor
		}
		cursor++
	}
	return cursor
}

func startOfDay(now time.Time) time.Time {
	local := now.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

// maxDrawableOrderScan 限制"可抽订单"扫描的订单条数。
// 能抽的订单必然还在抽奖有效期内，不可能翻到很旧的分页，取最近这些足够。
const maxDrawableOrderScan = 50

// ListDrawableOrders 返回该用户当前还能抽奖的订单，供独立抽奖页选择。
func (s *Service) ListDrawableOrders(userID uint) ([]DrawableOrderView, error) {
	views := make([]DrawableOrderView, 0)
	if s == nil || s.repo == nil || userID == 0 {
		return views, nil
	}
	config, err := s.repo.GetConfig()
	if err != nil {
		return nil, err
	}
	if config == nil || !config.Enabled || s.orders == nil {
		return views, nil
	}

	orders, err := s.orders.ListRecentUserOrders(userID, maxDrawableOrderScan)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return views, nil
	}

	// 一次取回该用户的抽奖记录按订单归并，避免每个订单打一次库。
	draws, _, err := s.repo.ListDraws(lotterycontract.DrawListFilter{
		UserID:   userID,
		Page:     1,
		PageSize: maxDrawableOrderScan * lotterydomain.MaxDrawsPerOrder,
	})
	if err != nil {
		return nil, err
	}
	usedByOrder := make(map[uint]int, len(orders))
	for index := range draws {
		usedByOrder[draws[index].OrderID]++
	}

	now := time.Now()
	for index := range orders {
		order := orders[index]
		used := usedByOrder[order.ID]
		if s.evaluateEligibility(config, &order, used, now) != ReasonOK {
			continue
		}
		remaining := config.DrawsPerOrder - used
		if remaining <= 0 {
			continue
		}
		views = append(views, DrawableOrderView{
			OrderNo:     order.OrderNo,
			Status:      order.Status,
			TotalAmount: order.TotalAmount,
			Currency:    order.Currency,
			PaidAt:      order.PaidAt,
			Remaining:   remaining,
			ExpiresAt:   config.DrawDeadline(order.PaidAt),
		})
	}
	return views, nil
}
