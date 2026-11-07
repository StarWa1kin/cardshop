package gormstore

import (
	"fmt"

	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"
	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"
)

// trafficSumExpr 按作用域拆分 PV/UV 汇总。
func trafficSumExpr() string {
	return fmt.Sprintf(`
		COALESCE(SUM(CASE WHEN scope = '%[1]s' THEN pv ELSE 0 END), 0) as home_pv,
		COALESCE(SUM(CASE WHEN scope = '%[1]s' THEN uv ELSE 0 END), 0) as home_uv,
		COALESCE(SUM(CASE WHEN scope = '%[2]s' THEN pv ELSE 0 END), 0) as product_pv,
		COALESCE(SUM(CASE WHEN scope = '%[2]s' THEN uv ELSE 0 END), 0) as product_uv
	`, analyticsdomain.ScopeHome, analyticsdomain.ScopeProduct)
}

// GetTrafficOverview 获取区间内的访问汇总。
//
// 访问统计按天聚合存储，所以这里用日期字符串闭区间过滤，不走时间戳。
func (r *Store) GetTrafficOverview(fromDate, toDate string) (dashboard.TrafficOverviewRow, error) {
	row := dashboard.TrafficOverviewRow{}
	if r == nil || r.db == nil {
		return row, nil
	}
	if err := r.db.Model(&analyticsdomain.VisitDaily{}).
		Select(trafficSumExpr()).
		Where("stat_date >= ? AND stat_date <= ?", fromDate, toDate).
		Scan(&row).Error; err != nil {
		return dashboard.TrafficOverviewRow{}, err
	}
	return row, nil
}

// GetTrafficTrends 获取按天的访问趋势。
func (r *Store) GetTrafficTrends(fromDate, toDate string) ([]dashboard.TrafficTrendRow, error) {
	rows := make([]dashboard.TrafficTrendRow, 0)
	if r == nil || r.db == nil {
		return rows, nil
	}
	if err := r.db.Model(&analyticsdomain.VisitDaily{}).
		Select("stat_date as day, "+trafficSumExpr()).
		Where("stat_date >= ? AND stat_date <= ?", fromDate, toDate).
		Group("stat_date").
		Order("day ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetTopProductViews 获取商品浏览排行榜。
func (r *Store) GetTopProductViews(fromDate, toDate string, limit int) ([]dashboard.ProductViewRankingRow, error) {
	if limit <= 0 {
		limit = 5
	}
	rows := make([]dashboard.ProductViewRankingRow, 0)
	if r == nil || r.db == nil {
		return rows, nil
	}
	titleExpr := localizedJSONCoalesceExpr(r.db, "products.title_json")
	if err := r.db.Model(&analyticsdomain.VisitDaily{}).
		Select(fmt.Sprintf(`
			visit_daily_stats.target_id as product_id,
			%s as title,
			COALESCE(SUM(visit_daily_stats.pv), 0) as pv,
			COALESCE(SUM(visit_daily_stats.uv), 0) as uv
		`, titleExpr)).
		Joins("JOIN products ON products.id = visit_daily_stats.target_id AND products.deleted_at IS NULL").
		Where("visit_daily_stats.scope = ? AND visit_daily_stats.stat_date >= ? AND visit_daily_stats.stat_date <= ?",
			analyticsdomain.ScopeProduct, fromDate, toDate).
		// 与商品销量榜同理：products.title_json 是 json 列，Postgres 下不能直接进 GROUP BY，
		// 改为按主键分组并对标题输出名分组。
		Group("visit_daily_stats.target_id, products.id, title").
		Order("pv DESC, uv DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
