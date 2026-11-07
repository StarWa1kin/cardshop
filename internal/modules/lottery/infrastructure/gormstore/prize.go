package gormstore

import (
	"errors"
	"time"

	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListPrizes 按扇区顺序返回奖品。
func (r *Store) ListPrizes(includeDisabled bool) ([]lotterydomain.Prize, error) {
	prizes := make([]lotterydomain.Prize, 0)
	if r == nil || r.db == nil {
		return prizes, nil
	}
	query := r.db.Model(&lotterydomain.Prize{}).Where("deleted_at IS NULL")
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	if err := query.Order("sort_order ASC, id ASC").Find(&prizes).Error; err != nil {
		return nil, err
	}
	return prizes, nil
}

// ListPrizesForUpdate 在事务内加锁读取可抽奖品，保证库存判断与扣减之间不被插队。
func (r *Store) ListPrizesForUpdate() ([]lotterydomain.Prize, error) {
	prizes := make([]lotterydomain.Prize, 0)
	if r == nil || r.db == nil {
		return prizes, nil
	}
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("deleted_at IS NULL AND enabled = ?", true).
		Order("sort_order ASC, id ASC").
		Find(&prizes).Error; err != nil {
		return nil, err
	}
	return prizes, nil
}

// GetPrizeByID 查询单个奖品。
func (r *Store) GetPrizeByID(id uint) (*lotterydomain.Prize, error) {
	if r == nil || r.db == nil || id == 0 {
		return nil, nil
	}
	var prize lotterydomain.Prize
	if err := r.db.Where("deleted_at IS NULL").First(&prize, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &prize, nil
}

// CreatePrize 新增奖品。
func (r *Store) CreatePrize(prize *lotterydomain.Prize) error {
	if r == nil || r.db == nil || prize == nil {
		return lotterydomain.ErrSaveFailed
	}
	now := time.Now()
	prize.CreatedAt = now
	prize.UpdatedAt = now
	return r.db.Create(prize).Error
}

// UpdatePrize 更新奖品。库存与已中奖数量由抽奖路径维护，这里用 Select 明确可改字段。
func (r *Store) UpdatePrize(prize *lotterydomain.Prize) error {
	if r == nil || r.db == nil || prize == nil || prize.ID == 0 {
		return lotterydomain.ErrSaveFailed
	}
	prize.UpdatedAt = time.Now()
	return r.db.Model(&lotterydomain.Prize{}).
		Where("id = ? AND deleted_at IS NULL", prize.ID).
		Select(
			"name", "type", "probability", "stock", "amount", "coupon_id", "product_id",
			"icon", "color", "description", "sort_order", "enabled", "updated_at",
		).
		Updates(prize).Error
}

// DeletePrize 软删除奖品。已产生的中奖记录保留快照，不受影响。
func (r *Store) DeletePrize(id uint) error {
	if r == nil || r.db == nil || id == 0 {
		return lotterydomain.ErrSaveFailed
	}
	now := time.Now()
	return r.db.Model(&lotterydomain.Prize{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{"deleted_at": now, "updated_at": now, "enabled": false}).Error
}

// ConsumePrizeStock 原子扣减库存并累加已中奖数量，返回是否扣减成功。
//
// 条件更新而不是"读出来再写回"：即使外层锁因为方言差异失效，
// stock > 0 的 WHERE 条件也能保证不会把库存扣成负数。
func (r *Store) ConsumePrizeStock(prizeID uint) (bool, error) {
	if r == nil || r.db == nil || prizeID == 0 {
		return false, nil
	}
	result := r.db.Model(&lotterydomain.Prize{}).
		Where("id = ? AND deleted_at IS NULL AND enabled = ?", prizeID, true).
		Where("stock = ? OR stock > 0", lotterydomain.StockUnlimited).
		Updates(map[string]interface{}{
			"stock":         gorm.Expr("CASE WHEN stock = ? THEN stock ELSE stock - 1 END", lotterydomain.StockUnlimited),
			"awarded_count": gorm.Expr("awarded_count + 1"),
			"updated_at":    time.Now(),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
