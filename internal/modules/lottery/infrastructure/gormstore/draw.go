package gormstore

import (
	"errors"
	"strings"
	"time"

	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
	"github.com/dujiao-next/internal/persistence/gormutil"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ListDraws 分页查询抽奖记录。
func (r *Store) ListDraws(filter lotterycontract.DrawListFilter) ([]lotterydomain.Draw, int64, error) {
	draws := make([]lotterydomain.Draw, 0)
	if r == nil || r.db == nil {
		return draws, 0, nil
	}
	query := r.applyDrawFilter(r.db.Model(&lotterydomain.Draw{}), filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := gormutil.ApplyPagination(query, filter.Page, filter.PageSize).
		Order("id DESC").
		Find(&draws).Error; err != nil {
		return nil, 0, err
	}
	return draws, total, nil
}

// GetDrawByID 查询单条抽奖记录。
func (r *Store) GetDrawByID(id uint) (*lotterydomain.Draw, error) {
	if r == nil || r.db == nil || id == 0 {
		return nil, nil
	}
	var draw lotterydomain.Draw
	if err := r.db.First(&draw, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &draw, nil
}

// CreateDraw 写入一条抽奖记录。
func (r *Store) CreateDraw(draw *lotterydomain.Draw) error {
	if r == nil || r.db == nil || draw == nil {
		return lotterydomain.ErrSaveFailed
	}
	now := time.Now()
	draw.CreatedAt = now
	draw.UpdatedAt = now
	return r.db.Create(draw).Error
}

// UpdateDraw 更新发放状态等后台可维护字段。
func (r *Store) UpdateDraw(draw *lotterydomain.Draw) error {
	if r == nil || r.db == nil || draw == nil || draw.ID == 0 {
		return lotterydomain.ErrSaveFailed
	}
	draw.UpdatedAt = time.Now()
	return r.db.Model(&lotterydomain.Draw{}).
		Where("id = ?", draw.ID).
		Select("delivery_status", "delivery_remark", "delivered_at", "delivered_by", "updated_at").
		Updates(draw).Error
}

// CountDrawsByOrder 统计某订单已抽次数。
func (r *Store) CountDrawsByOrder(orderID uint) (int, error) {
	if r == nil || r.db == nil || orderID == 0 {
		return 0, nil
	}
	var count int64
	if err := r.db.Model(&lotterydomain.Draw{}).Where("order_id = ?", orderID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// CountDrawsByUserSince 统计用户在给定时间之后的抽奖次数，用于每日上限。
func (r *Store) CountDrawsByUserSince(userID uint, since time.Time) (int, error) {
	if r == nil || r.db == nil || userID == 0 {
		return 0, nil
	}
	var count int64
	if err := r.db.Model(&lotterydomain.Draw{}).
		Where("user_id = ? AND created_at >= ?", userID, since).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// SumWonPrizes 汇总当前筛选条件下的抽奖概览。
func (r *Store) SumWonPrizes(filter lotterycontract.DrawListFilter) (lotterycontract.DrawTotals, error) {
	totals := lotterycontract.DrawTotals{WonAmount: money.FromDecimal(decimal.Zero)}
	if r == nil || r.db == nil {
		return totals, nil
	}
	var row struct {
		Total     int64           `gorm:"column:total"`
		WinCount  int64           `gorm:"column:win_count"`
		Pending   int64           `gorm:"column:pending"`
		WonAmount decimal.Decimal `gorm:"column:won_amount"`
	}
	query := r.applyDrawFilter(r.db.Model(&lotterydomain.Draw{}), filter)
	if err := query.Select(
		"COUNT(*) as total, " +
			"COALESCE(SUM(CASE WHEN is_win THEN 1 ELSE 0 END), 0) as win_count, " +
			"COALESCE(SUM(CASE WHEN delivery_status = 'pending' THEN 1 ELSE 0 END), 0) as pending, " +
			"COALESCE(SUM(CASE WHEN is_win AND prize_type = 'balance' THEN amount ELSE 0 END), 0) as won_amount",
	).Scan(&row).Error; err != nil {
		return totals, err
	}
	totals.Total = row.Total
	totals.WinCount = row.WinCount
	totals.Pending = row.Pending
	totals.WonAmount = money.FromDecimal(row.WonAmount)
	return totals, nil
}

func (r *Store) applyDrawFilter(query *gorm.DB, filter lotterycontract.DrawListFilter) *gorm.DB {
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if orderNo := strings.TrimSpace(filter.OrderNo); orderNo != "" {
		query = query.Where("order_no LIKE ?", "%"+orderNo+"%")
	}
	if filter.PrizeID > 0 {
		query = query.Where("prize_id = ?", filter.PrizeID)
	}
	if prizeType := lotterydomain.NormalizePrizeType(filter.PrizeType); prizeType != "" {
		query = query.Where("prize_type = ?", prizeType)
	}
	if filter.IsWin != nil {
		query = query.Where("is_win = ?", *filter.IsWin)
	}
	if status := lotterydomain.NormalizeDeliveryStatus(filter.DeliveryStatus); status != "" {
		query = query.Where("delivery_status = ?", status)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	return query
}
