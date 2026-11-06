package lotteryhttp

import (
	"errors"
	"strings"

	lotteryapp "github.com/dujiao-next/internal/modules/lottery/application"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// UserService 是前台抽奖用例端口。
type UserService interface {
	GetWheel() (*lotteryapp.WheelView, error)
	GetChance(userID uint, orderNo string) (*lotteryapp.ChanceView, error)
	Draw(input lotteryapp.DrawInput) (*lotteryapp.DrawResult, error)
	ListDrawableOrders(userID uint) ([]lotteryapp.DrawableOrderView, error)
	ListUserDraws(userID uint, page, pageSize int, onlyWin bool) (*lotteryapp.DrawListResult, error)
}

// UserHandler 处理用户侧抽奖请求。
//
// 所有路由都挂在用户鉴权组下，游客拿不到 user_id，也就进不来；
// 加上订单查询本身按 user_id 过滤，游客订单在数据层也查不到。
type UserHandler struct {
	lottery UserService
}

func NewUserHandler(lottery UserService) *UserHandler {
	if lottery == nil {
		panic("lottery user handler: lottery is nil")
	}
	return &UserHandler{lottery: lottery}
}

// GetWheel 返回活动信息与转盘扇区。
func (h *UserHandler) GetWheel(c *gin.Context) {
	if _, ok := ginutil.GetUserID(c); !ok {
		return
	}
	view, err := h.lottery.GetWheel()
	if err != nil {
		respondUserLotteryError(c, err)
		return
	}
	response.Success(c, view)
}

// GetChance 返回某笔订单的抽奖资格。
func (h *UserHandler) GetChance(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	view, err := h.lottery.GetChance(uid, orderNo)
	if err != nil {
		respondUserLotteryError(c, err)
		return
	}
	response.Success(c, view)
}

// Draw 执行一次抽奖。
func (h *UserHandler) Draw(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	result, err := h.lottery.Draw(lotteryapp.DrawInput{
		UserID:   uid,
		OrderNo:  orderNo,
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		respondUserLotteryError(c, err)
		return
	}
	response.Success(c, result)
}

// ListDrawableOrders 返回当前还能抽奖的订单，供独立抽奖页选择。
func (h *UserHandler) ListDrawableOrders(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	orders, err := h.lottery.ListDrawableOrders(uid)
	if err != nil {
		respondUserLotteryError(c, err)
		return
	}
	response.Success(c, orders)
}

// ListDraws 返回我的抽奖记录。
func (h *UserHandler) ListDraws(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	onlyWin, err := ginutil.ParseQueryBool(c, "only_win")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	result, err := h.lottery.ListUserDraws(uid, page, pageSize, onlyWin)
	if err != nil {
		respondUserLotteryError(c, err)
		return
	}
	response.SuccessWithPage(c, result.Items, response.BuildPagination(page, pageSize, result.Total))
}

func respondUserLotteryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, lotterydomain.ErrDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_disabled", nil)
	case errors.Is(err, lotterydomain.ErrGuestNotAllowed):
		ginutil.RespondError(c, response.CodeForbidden, "error.lottery_guest_not_allowed", nil)
	case errors.Is(err, lotterydomain.ErrOrderNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.lottery_order_not_found", nil)
	case errors.Is(err, lotterydomain.ErrOrderNotPaid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_order_not_paid", nil)
	case errors.Is(err, lotterydomain.ErrActivityNotStarted):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_activity_not_started", nil)
	case errors.Is(err, lotterydomain.ErrOrderNotEligible):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_order_not_eligible", nil)
	case errors.Is(err, lotterydomain.ErrChanceExhausted):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_chance_exhausted", nil)
	case errors.Is(err, lotterydomain.ErrChanceExpired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_chance_expired", nil)
	case errors.Is(err, lotterydomain.ErrDailyLimitReached):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_daily_limit_reached", nil)
	case errors.Is(err, lotterydomain.ErrNoPrizeConfigured):
		ginutil.RespondError(c, response.CodeBadRequest, "error.lottery_no_prize", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.lottery_draw_failed", err)
	}
}
