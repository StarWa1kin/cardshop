package application

import (
	"strings"

	lotterydomain "github.com/dujiao-next/internal/modules/lottery/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

const maxPrizeTextLength = 255

// ListPrizesForAdmin 返回后台奖品列表与概率概览。
func (s *Service) ListPrizesForAdmin() (*PrizeOverview, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrFetchFailed
	}
	prizes, err := s.repo.ListPrizes(true)
	if err != nil {
		return nil, err
	}
	configured := lotterydomain.ConfiguredProbability(prizes)
	remainder := lotterydomain.ProbabilityScale - configured
	if remainder < 0 {
		remainder = 0
	}
	return &PrizeOverview{
		Prizes:               lotterydomain.SortPrizes(prizes),
		ProbabilityScale:     lotterydomain.ProbabilityScale,
		ConfiguredTotal:      configured,
		RemainderProbability: remainder,
	}, nil
}

// CreatePrize 新增奖品。
func (s *Service) CreatePrize(input PrizeInput) (*lotterydomain.Prize, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrSaveFailed
	}
	prize, err := s.buildPrize(&lotterydomain.Prize{}, input)
	if err != nil {
		return nil, err
	}
	if err := s.assertProbabilityBudget(0, *prize); err != nil {
		return nil, err
	}
	if err := s.repo.CreatePrize(prize); err != nil {
		return nil, err
	}
	return prize, nil
}

// UpdatePrize 修改奖品。已中奖数量与中奖记录不受影响。
func (s *Service) UpdatePrize(id uint, input PrizeInput) (*lotterydomain.Prize, error) {
	if s == nil || s.repo == nil {
		return nil, lotterydomain.ErrSaveFailed
	}
	existing, err := s.repo.GetPrizeByID(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, lotterydomain.ErrPrizeNotFound
	}
	prize, err := s.buildPrize(existing, input)
	if err != nil {
		return nil, err
	}
	if err := s.assertProbabilityBudget(id, *prize); err != nil {
		return nil, err
	}
	if err := s.repo.UpdatePrize(prize); err != nil {
		return nil, err
	}
	return prize, nil
}

// DeletePrize 删除奖品。
func (s *Service) DeletePrize(id uint) error {
	if s == nil || s.repo == nil {
		return lotterydomain.ErrSaveFailed
	}
	existing, err := s.repo.GetPrizeByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return lotterydomain.ErrPrizeNotFound
	}
	return s.repo.DeletePrize(id)
}

// buildPrize 校验并归一化奖品入参。
func (s *Service) buildPrize(base *lotterydomain.Prize, input PrizeInput) (*lotterydomain.Prize, error) {
	prize := *base
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, lotterydomain.ErrPrizeInvalid
	}
	prizeType := lotterydomain.NormalizePrizeType(input.Type)
	if prizeType == "" {
		return nil, lotterydomain.ErrPrizeInvalid
	}
	if input.Probability < 0 || input.Probability > lotterydomain.ProbabilityScale {
		return nil, lotterydomain.ErrPrizeInvalid
	}
	if input.Stock < lotterydomain.StockUnlimited {
		return nil, lotterydomain.ErrPrizeInvalid
	}

	prize.Name = truncate(name, 100)
	prize.Type = prizeType
	prize.Probability = input.Probability
	prize.Stock = input.Stock
	prize.Icon = truncate(strings.TrimSpace(input.Icon), maxPrizeTextLength)
	prize.Color = truncate(strings.TrimSpace(input.Color), 20)
	prize.Description = truncate(strings.TrimSpace(input.Description), maxPrizeTextLength)
	prize.SortOrder = input.SortOrder
	prize.Enabled = input.Enabled
	prize.Amount = money.FromDecimal(decimal.Zero)
	prize.CouponID = nil
	prize.ProductID = nil

	switch prizeType {
	case lotterydomain.PrizeTypeBalance:
		amount := input.Amount.Decimal.Round(2)
		if amount.LessThanOrEqual(decimal.Zero) {
			return nil, lotterydomain.ErrPrizeInvalid
		}
		prize.Amount = money.FromDecimal(amount)
	case lotterydomain.PrizeTypeCoupon:
		if input.CouponID == nil || *input.CouponID == 0 {
			return nil, lotterydomain.ErrPrizeInvalid
		}
		// 券必须当下就能解析出来，否则后台会配出一个中了也拿不到东西的奖品。
		if s.coupons != nil {
			if _, ok := s.coupons.ResolveCoupon(*input.CouponID); !ok {
				return nil, lotterydomain.ErrPrizeInvalid
			}
		}
		prize.CouponID = input.CouponID
	case lotterydomain.PrizeTypeProduct:
		if input.ProductID == nil || *input.ProductID == 0 {
			return nil, lotterydomain.ErrPrizeInvalid
		}
		if s.products != nil {
			if _, ok := s.products.ResolveProductName(*input.ProductID); !ok {
				return nil, lotterydomain.ErrPrizeInvalid
			}
		}
		prize.ProductID = input.ProductID
	}
	return &prize, nil
}

// assertProbabilityBudget 校验启用奖品的概率之和不超过 100%。
//
// excludeID 是正在被修改的奖品，避免把它的旧概率重复计入。
func (s *Service) assertProbabilityBudget(excludeID uint, candidate lotterydomain.Prize) error {
	if !candidate.Enabled || candidate.Probability <= 0 {
		return nil
	}
	prizes, err := s.repo.ListPrizes(true)
	if err != nil {
		return err
	}
	total := candidate.Probability
	for index := range prizes {
		if prizes[index].ID == excludeID {
			continue
		}
		if !prizes[index].Enabled || prizes[index].Probability <= 0 {
			continue
		}
		total += prizes[index].Probability
	}
	if total > lotterydomain.ProbabilityScale {
		return lotterydomain.ErrProbabilityOverflow
	}
	return nil
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
