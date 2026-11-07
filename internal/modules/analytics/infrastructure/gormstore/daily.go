package gormstore

import (
	"time"

	analyticscontract "github.com/dujiao-next/internal/modules/analytics/contract"
	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ApplyDailyDeltas 批量把增量累加到按天聚合表。
//
// 用 UPSERT + excluded 累加，所以多实例并发刷盘也不会互相覆盖。
func (r *Store) ApplyDailyDeltas(deltas []analyticscontract.DailyDelta) error {
	if r == nil || r.db == nil || len(deltas) == 0 {
		return nil
	}

	rows := make([]analyticsdomain.VisitDaily, 0, len(deltas))
	for _, delta := range deltas {
		if delta.Scope == "" || delta.StatDate == "" {
			continue
		}
		if delta.PV <= 0 && delta.UV <= 0 {
			continue
		}
		rows = append(rows, analyticsdomain.VisitDaily{
			Scope:    delta.Scope,
			TargetID: delta.TargetID,
			StatDate: delta.StatDate,
			PV:       delta.PV,
			UV:       delta.UV,
		})
	}
	if len(rows) == 0 {
		return nil
	}

	table := analyticsdomain.VisitDaily{}.TableName()
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "scope"}, {Name: "target_id"}, {Name: "stat_date"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"pv":         gorm.Expr(table + ".pv + excluded.pv"),
			"uv":         gorm.Expr(table + ".uv + excluded.uv"),
			"updated_at": time.Now(),
		}),
	}).Create(&rows).Error
}

// GetProductViewTotals 汇总指定商品在日期区间内的 PV/UV。
func (r *Store) GetProductViewTotals(productIDs []uint, fromDate, toDate string) ([]analyticscontract.ProductViewRow, error) {
	rows := make([]analyticscontract.ProductViewRow, 0)
	if r == nil || r.db == nil || len(productIDs) == 0 {
		return rows, nil
	}
	if err := r.db.Model(&analyticsdomain.VisitDaily{}).
		Select("target_id as product_id, COALESCE(SUM(pv), 0) as pv, COALESCE(SUM(uv), 0) as uv").
		Where("scope = ? AND target_id IN ? AND stat_date >= ? AND stat_date <= ?",
			analyticsdomain.ScopeProduct, productIDs, fromDate, toDate).
		Group("target_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
