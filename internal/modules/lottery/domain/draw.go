package domain

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// 奖品发放状态。
const (
	// DeliveryStatusNone 无需发放（未中奖，或余额/优惠券已自动到账）。
	DeliveryStatusNone = "none"
	// DeliveryStatusPending 待后台发放。
	DeliveryStatusPending = "pending"
	// DeliveryStatusDelivered 后台已发放。
	DeliveryStatusDelivered = "delivered"
)

// MaxDeliveryRemarkLength 限制后台发放备注长度。
const MaxDeliveryRemarkLength = 500

// Draw 一次抽奖记录。未中奖同样落库，用于次数核销与后台核查。
//
// (OrderID, Seq) 建唯一索引：抽奖次数的并发控制交给数据库，
// 同一订单被并发点两次抽奖时只有一条能插入成功。
type Draw struct {
	ID             uint         `gorm:"primarykey" json:"id"`                                                        // 主键
	UserID         uint         `gorm:"not null;index" json:"user_id"`                                               // 抽奖用户（游客不可抽奖，必然非 0）
	OrderID        uint         `gorm:"not null;uniqueIndex:uniq_lottery_draw_order_seq,priority:1" json:"order_id"` // 来源订单
	OrderNo        string       `gorm:"type:varchar(64);not null;index" json:"order_no"`                             // 订单号快照
	Seq            int          `gorm:"not null;uniqueIndex:uniq_lottery_draw_order_seq,priority:2" json:"seq"`      // 该订单的第几次抽奖
	PrizeID        uint         `gorm:"not null;default:0;index" json:"prize_id"`                                    // 命中奖品（0 表示未中奖）
	PrizeName      string       `gorm:"type:varchar(100)" json:"prize_name"`                                         // 奖品名称快照
	PrizeType      string       `gorm:"type:varchar(20);not null;default:none" json:"prize_type"`                    // 奖品类型快照
	IsWin          bool         `gorm:"not null;default:false;index" json:"is_win"`                                  // 是否中奖
	Amount         money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"amount"`                         // 余额奖品金额快照
	Currency       string       `gorm:"type:varchar(10)" json:"currency"`                                            // 币种快照
	CouponID       *uint        `gorm:"index" json:"coupon_id,omitempty"`                                            // 优惠券奖品ID快照
	CouponCode     string       `gorm:"type:varchar(64)" json:"coupon_code,omitempty"`                               // 优惠券码快照
	ProductID      *uint        `gorm:"index" json:"product_id,omitempty"`                                           // 商品奖品ID快照
	ProductName    string       `gorm:"type:varchar(200)" json:"product_name,omitempty"`                             // 商品名称快照
	WalletTxnID    *uint        `gorm:"index" json:"wallet_txn_id,omitempty"`                                        // 余额入账流水
	DeliveryStatus string       `gorm:"type:varchar(20);not null;default:none;index" json:"delivery_status"`         // 发放状态
	DeliveryRemark string       `gorm:"type:varchar(500)" json:"delivery_remark,omitempty"`                          // 发放备注
	DeliveredAt    *time.Time   `gorm:"index" json:"delivered_at,omitempty"`                                         // 发放时间
	DeliveredBy    *uint        `gorm:"index" json:"delivered_by,omitempty"`                                         // 发放操作管理员
	ClientIP       string       `gorm:"type:varchar(64)" json:"client_ip,omitempty"`                                 // 抽奖客户端IP
	CreatedAt      time.Time    `gorm:"index" json:"created_at"`                                                     // 抽奖时间
	UpdatedAt      time.Time    `json:"updated_at"`                                                                  // 更新时间
}

// TableName 指定表名
func (Draw) TableName() string {
	return "lottery_draws"
}

// NormalizeDeliveryStatus 归一化发放状态，未知值返回空字符串。
func NormalizeDeliveryStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case DeliveryStatusNone:
		return DeliveryStatusNone
	case DeliveryStatusPending:
		return DeliveryStatusPending
	case DeliveryStatusDelivered:
		return DeliveryStatusDelivered
	default:
		return ""
	}
}

// InitialDeliveryStatus 返回中奖记录落库时的发放状态。
func InitialDeliveryStatus(prizeType string) string {
	if NeedsManualDelivery(prizeType) {
		return DeliveryStatusPending
	}
	return DeliveryStatusNone
}
