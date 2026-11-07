package contract

// Repository 是访问统计的持久化端口。
type Repository interface {
	ApplyDailyDeltas(deltas []DailyDelta) error
	MarkVisitor(scope string, targetID uint, statDate, visitorHash string) (bool, error)
	GetProductViewTotals(productIDs []uint, fromDate, toDate string) ([]ProductViewRow, error)
	PruneVisitors(beforeDate string) (int64, error)
}

// ProductResolver 把商品 slug 解析为商品 ID，避免写入不存在的统计目标。
type ProductResolver interface {
	ResolveProductID(slug string) (uint, bool)
}

// DailyDelta 是一个统计单元在一次刷盘窗口内累积的增量。
type DailyDelta struct {
	Scope    string
	TargetID uint
	StatDate string
	PV       int64
	UV       int64
}

// ProductViewRow 是单个商品在查询区间内的浏览汇总。
type ProductViewRow struct {
	ProductID uint  `gorm:"column:product_id"`
	PV        int64 `gorm:"column:pv"`
	UV        int64 `gorm:"column:uv"`
}
