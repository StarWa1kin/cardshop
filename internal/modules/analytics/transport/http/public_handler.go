package analyticshttp

import (
	"errors"

	analyticsapp "github.com/dujiao-next/internal/modules/analytics/application"
	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// Collector 是前台埋点上报端口。
type Collector interface {
	Track(input analyticsapp.TrackInput) error
}

// PublicHandler 处理前台访问上报。
type PublicHandler struct {
	collector Collector
}

func NewPublicHandler(collector Collector) *PublicHandler {
	if collector == nil {
		panic("analytics public handler: collector is nil")
	}
	return &PublicHandler{collector: collector}
}

type trackVisitRequest struct {
	Scope      string `json:"scope" binding:"required"`
	Target     string `json:"target"`
	VisitorKey string `json:"visitor_key"`
}

// TrackVisit 记录一次页面访问（首页或商品详情页）。
func (h *PublicHandler) TrackVisit(c *gin.Context) {
	var req trackVisitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if h == nil || h.collector == nil {
		response.Success(c, gin.H{"ok": true})
		return
	}

	err := h.collector.Track(analyticsapp.TrackInput{
		Scope:       req.Scope,
		ProductSlug: req.Target,
		VisitorKey:  req.VisitorKey,
		ClientIP:    c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	})
	switch {
	case err == nil:
	case errors.Is(err, analyticsdomain.ErrScopeInvalid),
		errors.Is(err, analyticsdomain.ErrTargetInvalid),
		errors.Is(err, analyticsdomain.ErrTargetNotFound):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	default:
		// 埋点不是业务流程，落库异常只记日志，不让前台看到失败。
		ginutil.RequestLog(c).Warnw("analytics_track_visit_failed", "scope", req.Scope, "error", err)
	}
	response.Success(c, gin.H{"ok": true})
}
