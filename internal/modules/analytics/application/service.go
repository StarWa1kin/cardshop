package application

import (
	"context"
	"time"

	analyticscontract "github.com/dujiao-next/internal/modules/analytics/contract"
	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"
	reportingapp "github.com/dujiao-next/internal/modules/reporting/application"
)

// maxProductViewIDs 限制单次查询的商品数量，对齐后台列表分页上限。
const maxProductViewIDs = 200

// Service 提供后台读取访问统计的用例。
type Service struct {
	repo analyticscontract.Repository
}

// NewService 创建访问统计服务。
func NewService(repo analyticscontract.Repository) *Service {
	return &Service{repo: repo}
}

// GetProductViews 查询指定商品在区间内的 PV/UV。
func (s *Service) GetProductViews(_ context.Context, input ProductViewsQuery) (*ProductViewsResponse, error) {
	window, err := reportingapp.Resolve(input.Range, time.Now())
	if err != nil {
		return nil, err
	}

	response := &ProductViewsResponse{
		Range:    window.Range,
		From:     window.StartAt.Format(time.RFC3339),
		To:       window.EndAt.Add(-time.Second).Format(time.RFC3339),
		Timezone: window.Timezone,
		Items:    make([]ProductViewItem, 0),
	}
	if s == nil || s.repo == nil {
		return response, nil
	}

	productIDs := dedupeProductIDs(input.ProductIDs)
	if len(productIDs) == 0 {
		return response, nil
	}

	fromDate, toDate := analyticsdomain.StatDateRange(window.StartAt, window.EndAt)
	rows, err := s.repo.GetProductViewTotals(productIDs, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		response.Items = append(response.Items, ProductViewItem{
			ProductID: row.ProductID,
			PV:        row.PV,
			UV:        row.UV,
		})
	}
	return response, nil
}

func dedupeProductIDs(raw []uint) []uint {
	seen := make(map[uint]struct{}, len(raw))
	result := make([]uint, 0, len(raw))
	for _, id := range raw {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
		if len(result) >= maxProductViewIDs {
			break
		}
	}
	return result
}
