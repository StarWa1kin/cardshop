package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// ConfigSingletonID 是配置表唯一一行的主键。抽奖规则是站点级配置，不做多活动并行。
const ConfigSingletonID uint = 1

// 规则字段上限，防止后台填出让抽奖永远不可用或可以无限刷的值。
const (
	MaxDrawsPerOrder  = 20
	MaxDailyDrawLimit = 1000
	MaxValidHours     = 24 * 365
	MaxTitleLength    = 100
	MaxNoticeLength   = 2000
)

// Config 抽奖活动规则。整表只有 ConfigSingletonID 一行。
type Config struct {
	ID             uint         `gorm:"primarykey" json:"id"`                                          // 主键（固定为 1）
	Enabled        bool         `gorm:"not null;default:false" json:"enabled"`                         // 活动总开关
	Title          string       `gorm:"type:varchar(100)" json:"title"`                                // 活动名称
	Notice         string       `gorm:"type:text" json:"notice"`                                       // 活动规则说明（展示给用户）
	MinOrderAmount money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"min_order_amount"` // 参与门槛：订单实付金额
	StartsAt       *time.Time   `gorm:"index" json:"starts_at"`                                        // 活动开始时间：支付时间早于它的订单不参与；NULL 表示不限
	DrawsPerOrder  int          `gorm:"not null;default:1" json:"draws_per_order"`                     // 每笔订单可抽次数
	DailyDrawLimit int          `gorm:"not null;default:0" json:"daily_draw_limit"`                    // 每个用户每日抽奖次数上限（0 不限）
	ValidHours     int          `gorm:"not null;default:72" json:"valid_hours"`                        // 订单支付后多少小时内可抽（0 不限）
	CreatedAt      time.Time    `json:"created_at"`                                                    // 创建时间
	UpdatedAt      time.Time    `json:"updated_at"`                                                    // 更新时间
}

// TableName 指定表名
func (Config) TableName() string {
	return "lottery_configs"
}

// DefaultConfig 返回新安装的推荐规则。
//
// 总开关默认关闭：奖品概率还没配完就把活动开着，等于白送奖品。
func DefaultConfig() Config {
	return Config{
		ID:             ConfigSingletonID,
		Enabled:        false,
		MinOrderAmount: money.FromDecimal(decimal.Zero),
		DrawsPerOrder:  1,
		DailyDrawLimit: 0,
		ValidHours:     72,
	}
}

// NormalizeConfig 归一化规则，把越界值收回默认值。
func NormalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	cfg.ID = ConfigSingletonID
	if cfg.DrawsPerOrder < 1 || cfg.DrawsPerOrder > MaxDrawsPerOrder {
		cfg.DrawsPerOrder = defaults.DrawsPerOrder
	}
	if cfg.DailyDrawLimit < 0 || cfg.DailyDrawLimit > MaxDailyDrawLimit {
		cfg.DailyDrawLimit = defaults.DailyDrawLimit
	}
	if cfg.ValidHours < 0 || cfg.ValidHours > MaxValidHours {
		cfg.ValidHours = defaults.ValidHours
	}
	if cfg.MinOrderAmount.Decimal.IsNegative() {
		cfg.MinOrderAmount = defaults.MinOrderAmount
	}
	cfg.MinOrderAmount = money.FromDecimal(cfg.MinOrderAmount.Decimal.Round(2))
	cfg.Title = truncateRunes(cfg.Title, MaxTitleLength)
	cfg.Notice = truncateRunes(cfg.Notice, MaxNoticeLength)
	return cfg
}

// StartedFor 返回该支付时间的订单是否已进入活动区间。
//
// 没有这道闸门时，活动一上线，全库历史已付款订单会同时获得抽奖资格
// （有效期填 0 时尤其危险），奖品成本完全不可控。
func (c *Config) StartedFor(paidAt *time.Time) bool {
	if c == nil || c.StartsAt == nil || c.StartsAt.IsZero() {
		return true
	}
	if paidAt == nil || paidAt.IsZero() {
		return false
	}
	return !paidAt.Before(*c.StartsAt)
}

// DrawDeadline 返回订单的抽奖截止时间；未配置有效期时返回 nil。
func (c *Config) DrawDeadline(paidAt *time.Time) *time.Time {
	if c == nil || c.ValidHours <= 0 || paidAt == nil || paidAt.IsZero() {
		return nil
	}
	deadline := paidAt.Add(time.Duration(c.ValidHours) * time.Hour)
	return &deadline
}

// truncateRunes 按字符而不是字节截断，避免把中文截成半个字。
func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
