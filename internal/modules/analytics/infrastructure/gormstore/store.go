package gormstore

import (
	analyticscontract "github.com/dujiao-next/internal/modules/analytics/contract"

	"gorm.io/gorm"
)

// Store is the GORM adapter for analytics persistence ports.
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

var _ analyticscontract.Repository = (*Store)(nil)
