package domain

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// 奖品类型。
const (
	// PrizeTypeNone 谢谢参与，占转盘扇区但不产生奖励。
	PrizeTypeNone = "none"
	// PrizeTypeBalance 钱包余额，中奖后在同一事务内自动入账。
	PrizeTypeBalance = "balance"
	// PrizeTypeCoupon 优惠券，中奖后把已配置的券码快照进中奖记录。
	PrizeTypeCoupon = "coupon"
	// PrizeTypeProduct 商品奖品，中奖后进入待发放，由后台人工发放。
	PrizeTypeProduct = "product"
	// PrizeTypeCustom 自定义奖品（实物等），同样由后台人工发放。
	PrizeTypeCustom = "custom"
)

// ProbabilityScale 是中奖概率的刻度：概率以万分比存储，1 = 0.01%。
//
// 用整数万分比而不是浮点百分比，既能表达 0.01% 这种长尾概率，
// 又不会在累加比较时出现浮点误差导致总和"看起来超过 100%"。
const ProbabilityScale = 10000

// StockUnlimited 表示奖品不限库存。
const StockUnlimited = -1

// Prize 转盘奖品。一个奖品就是转盘上的一个扇区。
type Prize struct {
	ID           uint         `gorm:"primarykey" json:"id"`                                // 主键
	Name         string       `gorm:"type:varchar(100);not null" json:"name"`              // 奖品名称（转盘扇区文案）
	Type         string       `gorm:"type:varchar(20);not null;default:none" json:"type"`  // 奖品类型
	Probability  int          `gorm:"not null;default:0" json:"probability"`               // 中奖概率（万分比，1=0.01%）
	Stock        int          `gorm:"not null;default:-1" json:"stock"`                    // 剩余库存（-1 表示不限）
	AwardedCount int          `gorm:"not null;default:0" json:"awarded_count"`             // 已中奖数量
	Amount       money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"amount"` // 余额奖品金额
	CouponID     *uint        `gorm:"index" json:"coupon_id,omitempty"`                    // 优惠券奖品关联的优惠券
	ProductID    *uint        `gorm:"index" json:"product_id,omitempty"`                   // 商品奖品关联的商品
	Icon         string       `gorm:"type:varchar(255)" json:"icon"`                       // 奖品图标地址
	Color        string       `gorm:"type:varchar(20)" json:"color"`                       // 转盘扇区底色
	Description  string       `gorm:"type:varchar(255)" json:"description"`                // 奖品说明（展示给中奖用户）
	SortOrder    int          `gorm:"not null;default:0;index" json:"sort_order"`          // 扇区排序
	Enabled      bool         `gorm:"not null;default:true;index" json:"enabled"`          // 是否参与抽奖
	CreatedAt    time.Time    `gorm:"index" json:"created_at"`                             // 创建时间
	UpdatedAt    time.Time    `gorm:"index" json:"updated_at"`                             // 更新时间
	DeletedAt    *time.Time   `gorm:"index" json:"-"`                                      // 软删除时间
}

// TableName 指定表名
func (Prize) TableName() string {
	return "lottery_prizes"
}

// NormalizePrizeType 归一化奖品类型，未知值返回空字符串。
func NormalizePrizeType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case PrizeTypeNone:
		return PrizeTypeNone
	case PrizeTypeBalance:
		return PrizeTypeBalance
	case PrizeTypeCoupon:
		return PrizeTypeCoupon
	case PrizeTypeProduct:
		return PrizeTypeProduct
	case PrizeTypeCustom:
		return PrizeTypeCustom
	default:
		return ""
	}
}

// IsWinning 返回该奖品类型是否算中奖。谢谢参与不算。
func IsWinning(prizeType string) bool {
	return NormalizePrizeType(prizeType) != PrizeTypeNone && NormalizePrizeType(prizeType) != ""
}

// NeedsManualDelivery 返回该奖品类型是否需要后台人工发放。
func NeedsManualDelivery(prizeType string) bool {
	switch NormalizePrizeType(prizeType) {
	case PrizeTypeProduct, PrizeTypeCustom:
		return true
	default:
		return false
	}
}

// HasStock 返回奖品当前是否还能发放。
func (p *Prize) HasStock() bool {
	if p == nil {
		return false
	}
	if p.Stock == StockUnlimited {
		return true
	}
	return p.Stock > 0
}

// Drawable 返回奖品是否可以进入本次抽奖的候选池。
func (p *Prize) Drawable() bool {
	if p == nil || !p.Enabled || p.Probability <= 0 {
		return false
	}
	return p.HasStock()
}
