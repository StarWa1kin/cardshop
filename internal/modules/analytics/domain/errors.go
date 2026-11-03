package domain

import "errors"

var (
	// ErrScopeInvalid 作用域非法。
	ErrScopeInvalid = errors.New("analytics: visit scope is invalid")
	// ErrTargetInvalid 目标标识缺失或格式非法。
	ErrTargetInvalid = errors.New("analytics: visit target is invalid")
	// ErrTargetNotFound 目标不存在（例如商品已下架或 slug 不存在）。
	ErrTargetNotFound = errors.New("analytics: visit target not found")
)
