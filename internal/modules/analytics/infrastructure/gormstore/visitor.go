package gormstore

import (
	"strings"

	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"

	"gorm.io/gorm/clause"
)

// MarkVisitor 登记当日访客指纹，首次出现返回 true。
//
// 判重完全交给唯一索引：并发下只有一次插入会成功，UV 不会被重复计数。
func (r *Store) MarkVisitor(scope string, targetID uint, statDate, visitorHash string) (bool, error) {
	if r == nil || r.db == nil {
		return false, nil
	}
	if strings.TrimSpace(scope) == "" || strings.TrimSpace(statDate) == "" || strings.TrimSpace(visitorHash) == "" {
		return false, nil
	}
	record := analyticsdomain.VisitVisitorDaily{
		Scope:       scope,
		TargetID:    targetID,
		StatDate:    statDate,
		VisitorHash: visitorHash,
	}
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// PruneVisitors 删除早于指定统计日期的访客指纹。
func (r *Store) PruneVisitors(beforeDate string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	if strings.TrimSpace(beforeDate) == "" {
		return 0, nil
	}
	result := r.db.Where("stat_date < ?", beforeDate).Delete(&analyticsdomain.VisitVisitorDaily{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
