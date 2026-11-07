package gormstore

import (
	"errors"
	"time"

	lotterycontract "github.com/dujiao-next/internal/modules/lottery/contract"
	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"

	"gorm.io/gorm"
)

// Store 是抽奖持久化端口的 GORM 实现。
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// WithTx 返回绑定到给定事务的仓储副本。
func (r *Store) WithTx(tx *gorm.DB) *Store {
	if tx == nil {
		return r
	}
	return &Store{db: tx}
}

// Transaction 为抽奖写路径提供可与钱包入账共享的事务。
func (r *Store) Transaction(fn func(tx *gorm.DB) error) error {
	if r == nil || r.db == nil {
		return lotterydomain.ErrFetchFailed
	}
	return r.db.Transaction(fn)
}

// GetConfig 读取活动规则；首次读取时写入默认规则，避免后台打开页面是空的。
func (r *Store) GetConfig() (*lotterydomain.Config, error) {
	if r == nil || r.db == nil {
		return nil, lotterydomain.ErrFetchFailed
	}
	var config lotterydomain.Config
	err := r.db.First(&config, lotterydomain.ConfigSingletonID).Error
	if err == nil {
		normalized := lotterydomain.NormalizeConfig(config)
		return &normalized, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	defaults := lotterydomain.DefaultConfig()
	now := time.Now()
	defaults.CreatedAt = now
	defaults.UpdatedAt = now
	if createErr := r.db.Create(&defaults).Error; createErr != nil {
		// 并发首次访问时另一个请求可能已经插入，这里不把它当失败。
		var existing lotterydomain.Config
		if queryErr := r.db.First(&existing, lotterydomain.ConfigSingletonID).Error; queryErr == nil {
			normalized := lotterydomain.NormalizeConfig(existing)
			return &normalized, nil
		}
		return nil, createErr
	}
	return &defaults, nil
}

// SaveConfig 覆盖写入活动规则。
func (r *Store) SaveConfig(config *lotterydomain.Config) error {
	if r == nil || r.db == nil {
		return lotterydomain.ErrSaveFailed
	}
	if config == nil {
		return lotterydomain.ErrSaveFailed
	}
	config.ID = lotterydomain.ConfigSingletonID
	config.UpdatedAt = time.Now()
	if config.CreatedAt.IsZero() {
		config.CreatedAt = config.UpdatedAt
	}
	return r.db.Save(config).Error
}

var _ lotterycontract.Repository = (*Store)(nil)
