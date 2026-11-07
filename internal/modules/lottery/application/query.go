package application

import (
	"strings"
	"time"

	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
)

// maxDrawPageSize 限制单页记录数，和其他后台列表保持一致。
const maxDrawPageSize = 200

// ListUserDraws 返回用户自己的抽奖记录。
func (s *Service) ListUserDraws(userID uint, page, pageSize int, onlyWin bool) (*DrawListResult, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrFetchFailed
	}
	if userID == 0 {
		return nil, lotterydomain.ErrGuestNotAllowed
	}
	filter := lotterycontract.DrawListFilter{
		UserID:   userID,
		Page:     page,
		PageSize: normalizePageSize(pageSize),
	}
	if onlyWin {
		win := true
		filter.IsWin = &win
	}
	draws, total, err := s.repo.ListDraws(filter)
	if err != nil {
		return nil, err
	}
	return &DrawListResult{Items: newDrawViews(draws), Total: total}, nil
}

// ListDrawsForAdmin 返回后台中奖记录与概览。
func (s *Service) ListDrawsForAdmin(filter lotterycontract.DrawListFilter) (*DrawListResult, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrFetchFailed
	}
	filter.PageSize = normalizePageSize(filter.PageSize)
	draws, total, err := s.repo.ListDraws(filter)
	if err != nil {
		return nil, err
	}
	totals, err := s.repo.SumWonPrizes(filter)
	if err != nil {
		return nil, err
	}
	return &DrawListResult{Items: newDrawViews(draws), Total: total, Totals: totals}, nil
}

// MarkDelivered 标记中奖记录已发放，或把它退回待发放。
//
// 只对需要人工发放的奖品生效：余额与优惠券在抽中时就已经到账，
// 让后台能把它们改成"已发放"只会制造一份假的发放记录。
func (s *Service) MarkDelivered(input DeliverInput) (*DrawView, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrSaveFailed
	}
	draw, err := s.repo.GetDrawByID(input.DrawID)
	if err != nil {
		return nil, err
	}
	if draw == nil {
		return nil, lotterydomain.ErrDrawNotFound
	}
	if !draw.IsWin || !lotterydomain.NeedsManualDelivery(draw.PrizeType) {
		return nil, lotterydomain.ErrDeliveryNotApplicable
	}

	remark := strings.TrimSpace(input.Remark)
	if len(remark) > lotterydomain.MaxDeliveryRemarkLength {
		remark = remark[:lotterydomain.MaxDeliveryRemarkLength]
	}
	draw.DeliveryRemark = remark
	if input.Revert {
		draw.DeliveryStatus = lotterydomain.DeliveryStatusPending
		draw.DeliveredAt = nil
		draw.DeliveredBy = nil
	} else {
		now := time.Now()
		draw.DeliveryStatus = lotterydomain.DeliveryStatusDelivered
		draw.DeliveredAt = &now
		if input.AdminID > 0 {
			draw.DeliveredBy = &input.AdminID
		}
	}
	if err := s.repo.UpdateDraw(draw); err != nil {
		return nil, err
	}
	view := newDrawView(*draw)
	return &view, nil
}

func newDrawViews(draws []lotterydomain.Draw) []DrawView {
	views := make([]DrawView, 0, len(draws))
	for index := range draws {
		views = append(views, newDrawView(draws[index]))
	}
	return views
}

func newDrawView(draw lotterydomain.Draw) DrawView {
	return DrawView{
		ID:             draw.ID,
		UserID:         draw.UserID,
		OrderNo:        draw.OrderNo,
		Seq:            draw.Seq,
		PrizeID:        draw.PrizeID,
		PrizeName:      draw.PrizeName,
		PrizeType:      draw.PrizeType,
		IsWin:          draw.IsWin,
		Amount:         draw.Amount,
		Currency:       draw.Currency,
		CouponCode:     draw.CouponCode,
		ProductID:      draw.ProductID,
		ProductName:    draw.ProductName,
		DeliveryStatus: draw.DeliveryStatus,
		DeliveryRemark: draw.DeliveryRemark,
		DeliveredAt:    draw.DeliveredAt,
		CreatedAt:      draw.CreatedAt,
	}
}

func normalizePageSize(pageSize int) int {
	if pageSize <= 0 {
		return 20
	}
	if pageSize > maxDrawPageSize {
		return maxDrawPageSize
	}
	return pageSize
}
