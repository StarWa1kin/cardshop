// Package settingscurrency 把站点币种设置适配成抽奖模块的币种端口。
package settingscurrency

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
)

// Provider 读取站点币种，用于余额奖品入账与金额展示。
type Provider struct {
	settings *settingsapp.Service
}

func New(settings *settingsapp.Service) Provider {
	return Provider{settings: settings}
}

// SiteCurrency 返回站点币种，读取失败时回退默认币种。
func (provider Provider) SiteCurrency() string {
	if provider.settings == nil {
		return constants.SiteCurrencyDefault
	}
	currency, err := provider.settings.GetSiteCurrency(constants.SiteCurrencyDefault)
	if err != nil || strings.TrimSpace(currency) == "" {
		return constants.SiteCurrencyDefault
	}
	return currency
}

var _ lotterycontract.CurrencyProvider = (*Provider)(nil)
