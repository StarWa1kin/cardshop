package application

import reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"

// ProductViewsQuery 是后台按商品查询浏览量的输入。
type ProductViewsQuery struct {
	Range      reportingdomain.Query
	ProductIDs []uint
}

// ProductViewsResponse 是商品浏览量查询结果。
type ProductViewsResponse struct {
	Range    string            `json:"range"`
	From     string            `json:"from"`
	To       string            `json:"to"`
	Timezone string            `json:"timezone"`
	Items    []ProductViewItem `json:"items"`
}

// ProductViewItem 是单个商品在区间内的浏览汇总。
//
// UV 为区间内各日独立访客数之和（与主流统计口径一致），不是区间去重值。
type ProductViewItem struct {
	ProductID uint  `json:"product_id"`
	PV        int64 `json:"pv"`
	UV        int64 `json:"uv"`
}
