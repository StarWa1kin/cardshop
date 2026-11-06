package orderreader

import (
	"strings"

	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
)

// UserOrderReader 是抽奖资格判断所需的最小订单能力。
type UserOrderReader interface {
	GetByOrderNoAndUser(orderNo string, userID uint) (*orderdomain.Order, error)
	ListByUser(filter ordercontract.ListFilter) ([]orderdomain.Order, int64, error)
}

// Reader 把订单读模型收敛成抽奖只需要的事实。
//
// 只走"按订单号 + 用户ID"这一条查询：游客订单的 user_id 是 0，
// 任何已登录用户都查不到别人的或游客的订单，所以游客订单在数据层就被排除了。
type Reader struct {
	orders UserOrderReader
}

func New(orders UserOrderReader) *Reader {
	return &Reader{orders: orders}
}

// GetUserOrderByOrderNo 返回属于该用户的订单快照。
func (r *Reader) GetUserOrderByOrderNo(orderNo string, userID uint) (*lotterycontract.OrderSnapshot, error) {
	if r == nil || r.orders == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(orderNo)
	if trimmed == "" || userID == 0 {
		return nil, nil
	}
	order, err := r.orders.GetByOrderNoAndUser(trimmed, userID)
	if err != nil {
		return nil, err
	}
	if order == nil || order.ID == 0 {
		return nil, nil
	}
	return &lotterycontract.OrderSnapshot{
		ID:          order.ID,
		OrderNo:     order.OrderNo,
		UserID:      order.UserID,
		Status:      order.Status,
		TotalAmount: order.TotalAmount,
		Currency:    order.Currency,
		PaidAt:      order.PaidAt,
	}, nil
}

// ListRecentUserOrders 返回该用户最近的订单快照。
func (r *Reader) ListRecentUserOrders(userID uint, limit int) ([]lotterycontract.OrderSnapshot, error) {
	snapshots := make([]lotterycontract.OrderSnapshot, 0)
	if r == nil || r.orders == nil || userID == 0 || limit <= 0 {
		return snapshots, nil
	}
	orders, _, err := r.orders.ListByUser(ordercontract.ListFilter{
		UserID:   userID,
		Page:     1,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}
	for index := range orders {
		order := orders[index]
		if order.ID == 0 || order.UserID == 0 {
			continue
		}
		snapshots = append(snapshots, lotterycontract.OrderSnapshot{
			ID:          order.ID,
			OrderNo:     order.OrderNo,
			UserID:      order.UserID,
			Status:      order.Status,
			TotalAmount: order.TotalAmount,
			Currency:    order.Currency,
			PaidAt:      order.PaidAt,
		})
	}
	return snapshots, nil
}

var _ lotterycontract.OrderReader = (*Reader)(nil)
