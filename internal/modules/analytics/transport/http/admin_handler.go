package analyticshttp

import (
	"context"
	"errors"
	"strconv"
	"strings"

	analyticsapp "github.com/dujiao-next/internal/modules/analytics/application"
	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
	reportinghttp "github.com/dujiao-next/internal/modules/reporting/transport/http"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// Reader 是后台访问统计用例面。
type Reader interface {
	GetProductViews(ctx context.Context, input analyticsapp.ProductViewsQuery) (*analyticsapp.ProductViewsResponse, error)
}

// AdminHandler 处理后台访问统计查询。
type AdminHandler struct {
	reader Reader
}

func NewAdminHandler(reader Reader) *AdminHandler {
	if reader == nil {
		panic("analytics admin handler: reader is nil")
	}
	return &AdminHandler{reader: reader}
}

// GetProductViews 查询商品浏览量（供商品列表按页补数）。
func (h *AdminHandler) GetProductViews(c *gin.Context) {
	input, err := reportinghttp.ParseQuery(c)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if h == nil || h.reader == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.visit_stats_fetch_failed", nil)
		return
	}

	data, err := h.reader.GetProductViews(c.Request.Context(), analyticsapp.ProductViewsQuery{
		Range:      input,
		ProductIDs: parseProductIDs(c.Query("product_ids")),
	})
	if err != nil {
		if errors.Is(err, reportingdomain.ErrRangeInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.visit_stats_fetch_failed", err)
		return
	}
	response.Success(c, data)
}

func parseProductIDs(raw string) []uint {
	segments := strings.Split(raw, ",")
	result := make([]uint, 0, len(segments))
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			continue
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil || parsed == 0 {
			continue
		}
		result = append(result, uint(parsed))
	}
	return result
}
