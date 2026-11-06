// Package awardreader 解析奖品关联的优惠券与商品，只为写入中奖快照服务。
package awardreader

import (
	"strconv"
	"strings"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
)

// CouponByIDReader 是读取优惠券的最小能力。
type CouponByIDReader interface {
	GetByID(id uint) (*coupondomain.Coupon, error)
}

// ProductByIDReader 是读取商品的最小能力。
type ProductByIDReader interface {
	GetByID(id string) (*productdomain.Product, error)
}

// Reader 把奖品关联对象解析成中奖记录里的文本快照。
type Reader struct {
	coupons  CouponByIDReader
	products ProductByIDReader
}

func New(coupons CouponByIDReader, products ProductByIDReader) *Reader {
	return &Reader{coupons: coupons, products: products}
}

// ResolveCoupon 返回优惠券码。券被删除或停用时返回 found=false，
// 调用方据此判断奖品配置已失效。
func (r *Reader) ResolveCoupon(couponID uint) (string, bool) {
	if r == nil || r.coupons == nil || couponID == 0 {
		return "", false
	}
	coupon, err := r.coupons.GetByID(couponID)
	if err != nil || coupon == nil || coupon.ID == 0 {
		return "", false
	}
	if !coupon.IsActive {
		return coupon.Code, false
	}
	return coupon.Code, true
}

// ResolveProductName 返回商品名称快照。
//
// 商品标题是多语言 JSON，这里只取一个可读值写进记录，用于后台核对发放；
// 展示给用户的文案仍然用奖品自己的名称，不受这里的语言选择影响。
func (r *Reader) ResolveProductName(productID uint) (string, bool) {
	if r == nil || r.products == nil || productID == 0 {
		return "", false
	}
	product, err := r.products.GetByID(strconv.FormatUint(uint64(productID), 10))
	if err != nil || product == nil || product.ID == 0 {
		return "", false
	}
	return pickLocalizedTitle(product), true
}

func pickLocalizedTitle(product *productdomain.Product) string {
	if product == nil {
		return ""
	}
	for _, locale := range []string{"zh-CN", "en-US", "zh-TW"} {
		if text, ok := product.TitleJSON[locale].(string); ok {
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				return trimmed
			}
		}
	}
	for _, raw := range product.TitleJSON {
		if text, ok := raw.(string); ok {
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				return trimmed
			}
		}
	}
	return product.Slug
}

var (
	_ lotterycontract.CouponReader  = (*Reader)(nil)
	_ lotterycontract.ProductReader = (*Reader)(nil)
)
