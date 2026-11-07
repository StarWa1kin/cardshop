package catalogreader

import (
	"strings"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
)

// ProductSlugReader 是解析商品所需的最小 Catalog 能力。
type ProductSlugReader interface {
	GetBySlug(slug string, onlyActive bool) (*productdomain.Product, error)
}

// Reader 把前台上报的商品 slug 解析为商品 ID。
// 解析失败即视为非法目标，统计表不会写入不存在的商品。
type Reader struct {
	products ProductSlugReader
}

func New(products ProductSlugReader) *Reader {
	return &Reader{products: products}
}

// ResolveProductID 返回已上架商品的 ID。
func (r *Reader) ResolveProductID(slug string) (uint, bool) {
	if r == nil || r.products == nil {
		return 0, false
	}
	trimmed := strings.TrimSpace(slug)
	if trimmed == "" {
		return 0, false
	}
	product, err := r.products.GetBySlug(trimmed, true)
	if err != nil || product == nil || product.ID == 0 {
		return 0, false
	}
	return product.ID, true
}
