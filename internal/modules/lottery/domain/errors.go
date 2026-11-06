package domain

import "errors"

var (
	// ErrDisabled 抽奖活动未开启。
	ErrDisabled = errors.New("lottery: activity is disabled")
	// ErrPrizeInvalid 奖品配置不合法。
	ErrPrizeInvalid = errors.New("lottery: prize payload is invalid")
	// ErrPrizeNotFound 奖品不存在。
	ErrPrizeNotFound = errors.New("lottery: prize not found")
	// ErrProbabilityOverflow 启用奖品的概率之和超过上限。
	ErrProbabilityOverflow = errors.New("lottery: total probability exceeds the scale")
	// ErrNoPrizeConfigured 没有任何可抽奖品。
	ErrNoPrizeConfigured = errors.New("lottery: no drawable prize configured")
	// ErrOrderNotFound 订单不存在或不属于当前用户。
	ErrOrderNotFound = errors.New("lottery: order not found")
	// ErrOrderNotPaid 订单未支付，不能抽奖。
	ErrOrderNotPaid = errors.New("lottery: order is not paid")
	// ErrOrderNotEligible 订单不满足参与门槛（例如实付金额不足）。
	ErrOrderNotEligible = errors.New("lottery: order is not eligible")
	// ErrChanceExhausted 该订单的抽奖次数已用完。
	ErrChanceExhausted = errors.New("lottery: order draw chance exhausted")
	// ErrActivityNotStarted 订单支付时间早于活动开始时间。
	ErrActivityNotStarted = errors.New("lottery: order was paid before the activity started")
	// ErrChanceExpired 该订单已超过抽奖有效期。
	ErrChanceExpired = errors.New("lottery: order draw chance expired")
	// ErrDailyLimitReached 当日抽奖次数已达上限。
	ErrDailyLimitReached = errors.New("lottery: daily draw limit reached")
	// ErrGuestNotAllowed 游客订单不参与抽奖。
	ErrGuestNotAllowed = errors.New("lottery: guest order is not allowed")
	// ErrDrawNotFound 抽奖记录不存在。
	ErrDrawNotFound = errors.New("lottery: draw record not found")
	// ErrDeliveryNotApplicable 该记录不需要人工发放。
	ErrDeliveryNotApplicable = errors.New("lottery: draw record does not need manual delivery")
	// ErrDrawFailed 抽奖落库失败。
	ErrDrawFailed = errors.New("lottery: draw failed")
	// ErrFetchFailed 读取抽奖数据失败。
	ErrFetchFailed = errors.New("lottery: fetch failed")
	// ErrSaveFailed 写入抽奖数据失败。
	ErrSaveFailed = errors.New("lottery: save failed")
)
