package domain

import (
	"strings"
	"time"
)

// 访问统计作用域。当前只统计首页与商品详情页，后续可扩展文章、分类等页面。
const (
	ScopeHome    = "home"
	ScopeProduct = "product"
)

// StatDateLayout 是 stat_date 的存储格式。
//
// 注意：写入时按服务器本地时区分桶（见 FormatStatDate），后台查询时按管理员
// 选择的时区换算日期区间（见 StatDateRange）。两者不一致时边界那天会有少量偏差，
// 这是刻意用「整数日计数」换取的简单与低成本。
const StatDateLayout = "2006-01-02"

// VisitorRetentionDays 是 UV 去重指纹的保留天数，超期指纹由采集器定期清理。
const VisitorRetentionDays = 7

// VisitDaily 按天聚合的访问计数。
// 一个 (scope, target_id, stat_date) 只有一行，PV/UV 以增量方式累加。
type VisitDaily struct {
	ID        uint      `gorm:"primarykey" json:"id"`                                                                     // 主键
	Scope     string    `gorm:"type:varchar(16);not null;uniqueIndex:uniq_visit_daily,priority:1" json:"scope"`           // 作用域（home/product）
	TargetID  uint      `gorm:"not null;uniqueIndex:uniq_visit_daily,priority:2" json:"target_id"`                        // 目标ID（首页固定 0）
	StatDate  string    `gorm:"type:varchar(10);not null;uniqueIndex:uniq_visit_daily,priority:3;index" json:"stat_date"` // 统计日期
	PV        int64     `gorm:"not null;default:0" json:"pv"`                                                             // 当日浏览量
	UV        int64     `gorm:"not null;default:0" json:"uv"`                                                             // 当日独立访客数
	CreatedAt time.Time `json:"created_at"`                                                                               // 创建时间
	UpdatedAt time.Time `json:"updated_at"`                                                                               // 更新时间
}

// TableName 指定表名
func (VisitDaily) TableName() string {
	return "visit_daily_stats"
}

// VisitVisitorDaily 当日访客指纹，用于 UV 去重。
// 只存不可逆哈希，不落访客标识原值、IP 与 UA。
type VisitVisitorDaily struct {
	ID          uint      `gorm:"primarykey" json:"id"`                                                                       // 主键
	Scope       string    `gorm:"type:varchar(16);not null;uniqueIndex:uniq_visit_visitor,priority:1" json:"scope"`           // 作用域
	TargetID    uint      `gorm:"not null;uniqueIndex:uniq_visit_visitor,priority:2" json:"target_id"`                        // 目标ID
	StatDate    string    `gorm:"type:varchar(10);not null;uniqueIndex:uniq_visit_visitor,priority:3;index" json:"stat_date"` // 统计日期
	VisitorHash string    `gorm:"type:varchar(32);not null;uniqueIndex:uniq_visit_visitor,priority:4" json:"visitor_hash"`    // 访客指纹
	CreatedAt   time.Time `json:"created_at"`                                                                                 // 创建时间
}

// TableName 指定表名
func (VisitVisitorDaily) TableName() string {
	return "visit_daily_visitors"
}

// NormalizeScope 归一化作用域，未知值返回空字符串。
func NormalizeScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ScopeHome:
		return ScopeHome
	case ScopeProduct:
		return ScopeProduct
	default:
		return ""
	}
}

// FormatStatDate 按服务器本地时区把时间转换为统计日期。
func FormatStatDate(at time.Time) string {
	return at.In(time.Local).Format(StatDateLayout)
}

// StatDateRange 把左闭右开的时间窗口转换为闭区间日期字符串 [from, to]。
func StatDateRange(startAt, endAt time.Time) (string, string) {
	from := startAt.Format(StatDateLayout)
	last := endAt.Add(-time.Nanosecond)
	if last.Before(startAt) {
		last = startAt
	}
	return from, last.Format(StatDateLayout)
}

// ShiftStatDate 返回相对给定时间偏移若干天的统计日期。
func ShiftStatDate(at time.Time, days int) string {
	return FormatStatDate(at.AddDate(0, 0, days))
}
