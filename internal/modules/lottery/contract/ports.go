package contract

import (
	"time"

	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// DrawListFilter 是抽奖记录查询条件。
type DrawListFilter struct {
	UserID         uint
	OrderNo        string
	PrizeID        uint
	PrizeType      string
	IsWin          *bool
	DeliveryStatus string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	Page           int
	PageSize       int
}

// Repository 是抽奖模块的持久化端口。
type Repository interface {
	GetConfig() (*lotterydomain.Config, error)
	SaveConfig(config *lotterydomain.Config) error

	ListPrizes(includeDisabled bool) ([]lotterydomain.Prize, error)
	GetPrizeByID(id uint) (*lotterydomain.Prize, error)
	CreatePrize(prize *lotterydomain.Prize) error
	UpdatePrize(prize *lotterydomain.Prize) error
	DeletePrize(id uint) error

	ListDraws(filter DrawListFilter) ([]lotterydomain.Draw, int64, error)
	GetDrawByID(id uint) (*lotterydomain.Draw, error)
	UpdateDraw(draw *lotterydomain.Draw) error
	CountDrawsByOrder(orderID uint) (int, error)
	CountDrawsByUserSince(userID uint, since time.Time) (int, error)
	SumWonPrizes(filter DrawListFilter) (DrawTotals, error)
}

// DrawTotals 是抽奖记录的汇总数字，供后台列表显示概览。
type DrawTotals struct {
	Total     int64        `json:"total"`
	WinCount  int64        `json:"win_count"`
	Pending   int64        `json:"pending"`
	WonAmount money.Amount `json:"won_amount"`
}

// WalletCreditInput 描述余额奖品入账需要的事实。
type WalletCreditInput struct {
	UserID    uint
	Amount    money.Amount
	Currency  string
	TxnType   string
	Reference string
	Remark    string
	OrderID   *uint
}

// DrawTransaction 是一次抽奖在同一事务内需要的最小能力集合。
type DrawTransaction interface {
	ListPrizesForUpdate() ([]lotterydomain.Prize, error)
	ConsumePrizeStock(prizeID uint) (bool, error)
	CountDrawsByOrder(orderID uint) (int, error)
	CountDrawsByUserSince(userID uint, since time.Time) (int, error)
	CreateDraw(draw *lotterydomain.Draw) error
	CreditWallet(input WalletCreditInput) (*walletdomain.Account, *walletdomain.Transaction, error)
}

// DrawTransactionRunner 保证"扣库存 + 写记录 + 余额入账"原子提交或回滚。
type DrawTransactionRunner interface {
	WithinDrawTransaction(fn func(tx DrawTransaction) error) error
}

// OrderSnapshot 是抽奖资格判断需要的订单事实。
type OrderSnapshot struct {
	ID          uint
	OrderNo     string
	UserID      uint
	Status      string
	TotalAmount money.Amount
	Currency    string
	PaidAt      *time.Time
}

// OrderReader 读取订单抽奖资格。游客订单取不到（UserID 为 0），所以游客天然抽不了。
type OrderReader interface {
	GetUserOrderByOrderNo(orderNo string, userID uint) (*OrderSnapshot, error)
	// ListRecentUserOrders 返回该用户最近的订单，供抽奖页列出"还能抽的订单"。
	// 只取最近若干条：能抽的订单必然在抽奖有效期内，不可能翻到很旧的分页。
	ListRecentUserOrders(userID uint, limit int) ([]OrderSnapshot, error)
}

// CouponReader 解析优惠券奖品的券码。
type CouponReader interface {
	ResolveCoupon(couponID uint) (code string, found bool)
}

// ProductReader 解析商品奖品的名称。
type ProductReader interface {
	ResolveProductName(productID uint) (name string, found bool)
}

// CurrencyProvider 是站点币种读取端口。
type CurrencyProvider interface {
	SiteCurrency() string
}
