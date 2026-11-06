package application

import (
	"time"

	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// 抽奖资格判定结果。前台按这些码展示对应文案，不依赖后端返回的自然语言。
const (
	ReasonOK            = "ok"
	ReasonDisabled      = "disabled"
	ReasonOrderNotFound = "order_not_found"
	ReasonNotPaid       = "not_paid"
	ReasonNotStarted    = "not_started"
	ReasonAmountTooLow  = "amount_too_low"
	ReasonExpired       = "expired"
	ReasonExhausted     = "exhausted"
	ReasonDailyLimit    = "daily_limit"
	ReasonNoPrize       = "no_prize"
)

// MissSliceName 是兜底扇区的标识，前台按这个值取本地化文案。
const MissSliceName = "__miss__"

// WheelSlice 是转盘上的一个扇区。不返回概率：概率是后台配置，前台不需要也不该知道。
type WheelSlice struct {
	Index       int          `json:"index"`
	PrizeID     uint         `json:"prize_id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Icon        string       `json:"icon"`
	Color       string       `json:"color"`
	Description string       `json:"description"`
	Amount      money.Amount `json:"amount"`
	SoldOut     bool         `json:"sold_out"`
}

// WheelView 是前台抽奖页需要的活动信息。
type WheelView struct {
	Enabled        bool         `json:"enabled"`
	Title          string       `json:"title"`
	Notice         string       `json:"notice"`
	Currency       string       `json:"currency"`
	MinOrderAmount money.Amount `json:"min_order_amount"`
	DrawsPerOrder  int          `json:"draws_per_order"`
	DailyDrawLimit int          `json:"daily_draw_limit"`
	ValidHours     int          `json:"valid_hours"`
	Slices         []WheelSlice `json:"slices"`
}

// DrawView 是一条抽奖结果的对外视图。
type DrawView struct {
	ID             uint         `json:"id"`
	UserID         uint         `json:"user_id"`
	OrderNo        string       `json:"order_no"`
	Seq            int          `json:"seq"`
	PrizeID        uint         `json:"prize_id"`
	PrizeName      string       `json:"prize_name"`
	PrizeType      string       `json:"prize_type"`
	IsWin          bool         `json:"is_win"`
	Amount         money.Amount `json:"amount"`
	Currency       string       `json:"currency"`
	CouponCode     string       `json:"coupon_code,omitempty"`
	ProductID      *uint        `json:"product_id,omitempty"`
	ProductName    string       `json:"product_name,omitempty"`
	DeliveryStatus string       `json:"delivery_status"`
	DeliveryRemark string       `json:"delivery_remark,omitempty"`
	DeliveredAt    *time.Time   `json:"delivered_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
}

// ChanceView 是某笔订单的抽奖资格。
type ChanceView struct {
	OrderNo   string     `json:"order_no"`
	Eligible  bool       `json:"eligible"`
	Reason    string     `json:"reason"`
	Total     int        `json:"total"`
	Used      int        `json:"used"`
	Remaining int        `json:"remaining"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Draws     []DrawView `json:"draws"`
}

// DrawableOrderView 是独立抽奖页里"还能抽的订单"条目。
type DrawableOrderView struct {
	OrderNo     string       `json:"order_no"`
	Status      string       `json:"status"`
	TotalAmount money.Amount `json:"total_amount"`
	Currency    string       `json:"currency"`
	PaidAt      *time.Time   `json:"paid_at,omitempty"`
	Remaining   int          `json:"remaining"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
}

// DrawResult 是一次抽奖的结果。SliceIndex 指向 WheelView.Slices 的下标，
// 由服务端决定命中扇区，前端只负责把指针转到那里。
type DrawResult struct {
	Draw       DrawView `json:"draw"`
	SliceIndex int      `json:"slice_index"`
	Remaining  int      `json:"remaining"`
}

// PrizeInput 是后台新增/修改奖品的入参。
type PrizeInput struct {
	Name        string
	Type        string
	Probability int
	Stock       int
	Amount      money.Amount
	CouponID    *uint
	ProductID   *uint
	Icon        string
	Color       string
	Description string
	SortOrder   int
	Enabled     bool
}

// ConfigInput 是后台保存活动规则的入参。
type ConfigInput struct {
	Enabled        bool
	Title          string
	Notice         string
	MinOrderAmount money.Amount
	StartsAt       *time.Time
	DrawsPerOrder  int
	DailyDrawLimit int
	ValidHours     int
}

// PrizeOverview 是后台奖品列表附带的概率概览。
type PrizeOverview struct {
	Prizes               []lotterydomain.Prize `json:"prizes"`
	ProbabilityScale     int                   `json:"probability_scale"`
	ConfiguredTotal      int                   `json:"configured_total"`
	RemainderProbability int                   `json:"remainder_probability"`
}

// DrawListResult 是后台中奖记录列表。
type DrawListResult struct {
	Items  []DrawView                 `json:"items"`
	Total  int64                      `json:"total"`
	Totals lotterycontract.DrawTotals `json:"totals"`
}

// DeliverInput 是后台标记发放的入参。
type DeliverInput struct {
	DrawID  uint
	AdminID uint
	Remark  string
	Revert  bool
}
