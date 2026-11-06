package application

import (
	"strings"
	"time"

	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
)

// defaultCurrency 是站点币种读取失败时的兜底。
const defaultCurrency = "CNY"

// Options 是抽奖服务的依赖。
type Options struct {
	Repository lotterycontract.Repository
	Runner     lotterycontract.DrawTransactionRunner
	Orders     lotterycontract.OrderReader
	Coupons    lotterycontract.CouponReader
	Products   lotterycontract.ProductReader
	Currency   lotterycontract.CurrencyProvider
	// Roll 可注入随机源，便于测试固定命中区间；留空时使用 crypto/rand。
	Roll func() int
}

// Service 提供抽奖的前台与后台用例。
type Service struct {
	repo     lotterycontract.Repository
	runner   lotterycontract.DrawTransactionRunner
	orders   lotterycontract.OrderReader
	coupons  lotterycontract.CouponReader
	products lotterycontract.ProductReader
	currency lotterycontract.CurrencyProvider
	roll     func() int
}

// NewService 创建抽奖服务。
func NewService(opts Options) *Service {
	roll := opts.Roll
	if roll == nil {
		roll = lotterydomain.Roll
	}
	return &Service{
		repo:     opts.Repository,
		runner:   opts.Runner,
		orders:   opts.Orders,
		coupons:  opts.Coupons,
		products: opts.Products,
		currency: opts.Currency,
		roll:     roll,
	}
}

// GetConfig 读取活动规则（后台用）。
func (s *Service) GetConfig() (*lotterydomain.Config, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrFetchFailed
	}
	return s.repo.GetConfig()
}

// SaveConfig 保存活动规则（后台用）。
func (s *Service) SaveConfig(input ConfigInput) (*lotterydomain.Config, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrSaveFailed
	}
	current, err := s.repo.GetConfig()
	if err != nil {
		return nil, err
	}
	config := lotterydomain.DefaultConfig()
	if current != nil {
		config = *current
	}
	wasEnabled := config.Enabled
	config.Enabled = input.Enabled
	config.Title = strings.TrimSpace(input.Title)
	config.Notice = strings.TrimSpace(input.Notice)
	config.MinOrderAmount = input.MinOrderAmount
	config.StartsAt = input.StartsAt
	config.DrawsPerOrder = input.DrawsPerOrder
	config.DailyDrawLimit = input.DailyDrawLimit
	config.ValidHours = input.ValidHours

	// 活动从关到开、且没有指定开始时间时，默认只对此刻之后付款的订单生效。
	// 否则"顺手开一下活动"会让全库历史订单同时获得抽奖资格。
	// 活动已经开着时不再兜底：这时清空开始时间是管理员的明确选择。
	if config.Enabled && !wasEnabled && config.StartsAt == nil {
		now := time.Now()
		config.StartsAt = &now
	}

	normalized := lotterydomain.NormalizeConfig(config)
	if err := s.repo.SaveConfig(&normalized); err != nil {
		return nil, err
	}
	return &normalized, nil
}

// siteCurrency 返回站点币种。
func (s *Service) siteCurrency() string {
	if s == nil || s.currency == nil {
		return defaultCurrency
	}
	if currency := strings.TrimSpace(s.currency.SiteCurrency()); currency != "" {
		return strings.ToUpper(currency)
	}
	return defaultCurrency
}
