// Package gormuow 把抽奖的库存扣减、记录写入与钱包入账绑定到同一个 GORM 事务。
package gormuow

import (
	"time"

	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
	"github.com/dujiao-next/internal/modules/lottery/infrastructure/gormstore"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"

	"gorm.io/gorm"
)

// Runner 提供抽奖写路径的事务边界。
//
// 余额奖品必须和中奖记录同生共死：要么用户看到奖金到账且记录在库，
// 要么两边都没发生，不能出现"记录说中了 10 块但钱包没动"。
type Runner struct {
	lottery *gormstore.Store
	wallet  *walletapp.Service
}

func New(lottery *gormstore.Store, wallet *walletapp.Service) *Runner {
	return &Runner{lottery: lottery, wallet: wallet}
}

// WithinDrawTransaction 在一个事务内执行抽奖写操作。
func (r *Runner) WithinDrawTransaction(fn func(tx lotterycontract.DrawTransaction) error) error {
	if r == nil || r.lottery == nil {
		return lotterydomain.ErrDrawFailed
	}
	return r.lottery.Transaction(func(tx *gorm.DB) error {
		return fn(&transaction{
			db:      tx,
			lottery: r.lottery.WithTx(tx),
			wallet:  r.wallet,
		})
	})
}

type transaction struct {
	db      *gorm.DB
	lottery *gormstore.Store
	wallet  *walletapp.Service
}

func (tx *transaction) ListPrizesForUpdate() ([]lotterydomain.Prize, error) {
	return tx.lottery.ListPrizesForUpdate()
}

func (tx *transaction) ConsumePrizeStock(prizeID uint) (bool, error) {
	return tx.lottery.ConsumePrizeStock(prizeID)
}

func (tx *transaction) CountDrawsByOrder(orderID uint) (int, error) {
	return tx.lottery.CountDrawsByOrder(orderID)
}

func (tx *transaction) CountDrawsByUserSince(userID uint, since time.Time) (int, error) {
	return tx.lottery.CountDrawsByUserSince(userID, since)
}

func (tx *transaction) CreateDraw(draw *lotterydomain.Draw) error {
	return tx.lottery.CreateDraw(draw)
}

func (tx *transaction) CreditWallet(input lotterycontract.WalletCreditInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	if tx.wallet == nil {
		return nil, nil, walletcontract.ErrTransactionRequired
	}
	return tx.wallet.CreditInTransaction(walletgormstore.UseTransaction(tx.db), walletcontract.CreditInput{
		UserID:    input.UserID,
		Amount:    input.Amount,
		Currency:  input.Currency,
		Type:      input.TxnType,
		Reference: input.Reference,
		Remark:    input.Remark,
		OrderID:   input.OrderID,
	})
}

var (
	_ lotterycontract.DrawTransactionRunner = (*Runner)(nil)
	_ lotterycontract.DrawTransaction       = (*transaction)(nil)
)
