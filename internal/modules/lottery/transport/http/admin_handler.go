package lotteryhttp

import (
	"errors"
	"strings"

	lotteryapp "github.com/dujiao-next/internal/modules/lottery/application"
	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// AdminService 是后台抽奖管理端口。
type AdminService interface {
	GetConfig() (*lotterydomain.Config, error)
	SaveConfig(input lotteryapp.ConfigInput) (*lotterydomain.Config, error)
	ListPrizesForAdmin() (*lotteryapp.PrizeOverview, error)
	CreatePrize(input lotteryapp.PrizeInput) (*lotterydomain.Prize, error)
	UpdatePrize(id uint, input lotteryapp.PrizeInput) (*lotterydomain.Prize, error)
	DeletePrize(id uint) error
	ListDrawsForAdmin(filter lotterycontract.DrawListFilter) (*lotteryapp.DrawListResult, error)
	MarkDelivered(input lotteryapp.DeliverInput) (*lotteryapp.DrawView, error)
}

// AdminHandler 处理后台抽奖配置与中奖记录请求。
type AdminHandler struct {
	lottery AdminService
}

func NewAdminHandler(lottery AdminService) *AdminHandler {
	if lottery == nil {
		panic("lottery admin handler: lottery is nil")
	}
	return &AdminHandler{lottery: lottery}
}

type configRequest struct {
	Enabled        bool   `json:"enabled"`
	Title          string `json:"title"`
	Notice         string `json:"notice"`
	MinOrderAmount string `json:"min_order_amount"`
	StartsAt       string `json:"starts_at"`
	DrawsPerOrder  int    `json:"draws_per_order"`
	DailyDrawLimit int    `json:"daily_draw_limit"`
	ValidHours     int    `json:"valid_hours"`
}

type prizeRequest struct {
	Name        string `json:"name" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Probability int    `json:"probability"`
	Stock       int    `json:"stock"`
	Amount      string `json:"amount"`
	CouponID    *uint  `json:"coupon_id"`
	ProductID   *uint  `json:"product_id"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	Enabled     *bool  `json:"enabled"`
}

type deliverRequest struct {
	Remark string `json:"remark"`
	Revert bool   `json:"revert"`
}

// GetConfig 读取活动规则。
func (h *AdminHandler) GetConfig(c *gin.Context) {
	config, err := h.lottery.GetConfig()
	if err != nil {
		respondAdminLotteryError(c, err)
		return
	}
	response.Success(c, config)
}

// UpdateConfig 保存活动规则。
func (h *AdminHandler) UpdateConfig(c *gin.Context) {
	var req configRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := parseAmount(req.MinOrderAmount)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	startsAt, err := ginutil.ParseTimeNullable(strings.TrimSpace(req.StartsAt))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	config, err := h.lottery.SaveConfig(lotteryapp.ConfigInput{
		Enabled:        req.Enabled,
		Title:          req.Title,
		Notice:         req.Notice,
		MinOrderAmount: amount,
		StartsAt:       startsAt,
		DrawsPerOrder:  req.DrawsPerOrder,
		DailyDrawLimit: req.DailyDrawLimit,
		ValidHours:     req.ValidHours,
	})
	if err != nil {
		respondAdminLotteryError(c, err)
		return
	}
	response.Success(c, config)
}

// ListPrizes 返回奖品列表与概率概览。
func (h *AdminHandler) ListPrizes(c *gin.Context) {
	overview, err := h.lottery.ListPrizesForAdmin()
	if err != nil {
		respondAdminLotteryError(c, err)
		return
	}
	response.Success(c, overview)
}

// CreatePrize 新增奖品。
func (h *AdminHandler) CreatePrize(c *gin.Context) {
	input, ok := bindPrizeInput(c)
	if !ok {
		return
	}
	prize, err := h.lottery.CreatePrize(input)
	if err != nil {
		respondAdminLotteryError(c, err)
		return
	}
	response.Success(c, prize)
}

// UpdatePrize 修改奖品。
func (h *AdminHandler) UpdatePrize(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	input, ok := bindPrizeInput(c)
	if !ok {
		return
	}
	prize, err := h.lottery.UpdatePrize(id, input)
	if err != nil {
		respondAdminLotteryError(c, err)
		return
	}
	response.Success(c, prize)
}

// DeletePrize 删除奖品。
func (h *AdminHandler) DeletePrize(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.lottery.DeletePrize(id); err != nil {
		respondAdminLotteryError(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// ListDraws 返回中奖结果列表。
func (h *AdminHandler) ListDraws(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	isWin, err := ginutil.ParseQueryBoolPtr(c, "is_win")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	userID, err := ginutil.ParseQueryUint(c.Query("user_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	prizeID, err := ginutil.ParseQueryUint(c.Query("prize_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	result, err := h.lottery.ListDrawsForAdmin(lotterycontract.DrawListFilter{
		UserID:         userID,
		OrderNo:        strings.TrimSpace(c.Query("order_no")),
		PrizeID:        prizeID,
		PrizeType:      strings.TrimSpace(c.Query("prize_type")),
		IsWin:          isWin,
		DeliveryStatus: strings.TrimSpace(c.Query("delivery_status")),
		CreatedFrom:    createdFrom,
		CreatedTo:      createdTo,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		respondAdminLotteryError(c, err)
		return
	}
	response.SuccessWithPage(c, gin.H{
		"items":  result.Items,
		"totals": result.Totals,
	}, response.BuildPagination(page, pageSize, result.Total))
}

// MarkDrawDelivered 标记中奖记录发放状态。
func (h *AdminHandler) MarkDrawDelivered(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var req deliverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	view, err := h.lottery.MarkDelivered(lotteryapp.DeliverInput{
		DrawID:  id,
		AdminID: adminID,
		Remark:  req.Remark,
		Revert:  req.Revert,
	})
	if err != nil {
		respondAdminLotteryError(c, err)
		return
	}
	response.Success(c, view)
}

func bindPrizeInput(c *gin.Context) (lotteryapp.PrizeInput, bool) {
	var req prizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return lotteryapp.PrizeInput{}, false
	}
	amount, err := parseAmount(req.Amount)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return lotteryapp.PrizeInput{}, false
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return lotteryapp.PrizeInput{
		Name:        req.Name,
		Type:        req.Type,
		Probability: req.Probability,
		Stock:       req.Stock,
		Amount:      amount,
		CouponID:    req.CouponID,
		ProductID:   req.ProductID,
		Icon:        req.Icon,
		Color:       req.Color,
		Description: req.Description,
		SortOrder:   req.SortOrder,
		Enabled:     enabled,
	}, true
}

// parseAmount 解析金额字符串；空值视为 0，避免前端不填就报错。
func parseAmount(raw string) (money.Amount, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return money.FromDecimal(decimal.Zero), nil
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return money.Amount{}, err
	}
	return money.FromDecimal(parsed), nil
}

func respondAdminLotteryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, lotterydomain.ErrPrizeInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_prize_invalid", nil)
	case errors.Is(err, lotterydomain.ErrPrizeNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.lottery_prize_not_found", nil)
	case errors.Is(err, lotterydomain.ErrProbabilityOverflow):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_probability_overflow", nil)
	case errors.Is(err, lotterydomain.ErrDrawNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.lottery_draw_not_found", nil)
	case errors.Is(err, lotterydomain.ErrDeliveryNotApplicable):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_delivery_not_applicable", nil)
	case errors.Is(err, lotterydomain.ErrSaveFailed):
		ginutil.RespondError(c, response.CodeInternal, "error.lottery_save_failed", err)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.lottery_fetch_failed", err)
	}
}
