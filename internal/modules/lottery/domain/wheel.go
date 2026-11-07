package domain

import (
	"crypto/rand"
	"math/big"
	"sort"
)

// SortPrizes 按后台配置的扇区顺序排序；顺序相同时用 ID 兜底，保证转盘扇区稳定。
func SortPrizes(prizes []Prize) []Prize {
	sorted := make([]Prize, len(prizes))
	copy(sorted, prizes)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SortOrder != sorted[j].SortOrder {
			return sorted[i].SortOrder < sorted[j].SortOrder
		}
		return sorted[i].ID < sorted[j].ID
	})
	return sorted
}

// ConfiguredProbability 返回所有启用奖品的概率之和（不看库存）。
// 后台保存奖品时用它校验总概率不超过 100%。
func ConfiguredProbability(prizes []Prize) int {
	total := 0
	for index := range prizes {
		if !prizes[index].Enabled || prizes[index].Probability <= 0 {
			continue
		}
		total += prizes[index].Probability
	}
	return total
}

// DrawableProbability 返回本次抽奖实际参与的概率之和。
// 售罄奖品的概率不会分给别人，而是落到"未中奖"，否则库存一空剩下的奖品就会被抽爆。
func DrawableProbability(prizes []Prize) int {
	total := 0
	for index := range prizes {
		if !prizes[index].Drawable() {
			continue
		}
		total += prizes[index].Probability
	}
	return total
}

// HasDrawablePrize 返回是否还有奖品可抽。
func HasDrawablePrize(prizes []Prize) bool {
	for index := range prizes {
		if prizes[index].Drawable() {
			return true
		}
	}
	return false
}

// Pick 根据 [0, ProbabilityScale) 区间内的随机数命中奖品，返回 nil 表示未中奖。
//
// 调用方需传入已排序的奖品，命中区间与转盘扇区顺序一致，这样前端可以直接按
// 返回的奖品定位扇区，而不用自己再算一次概率（也就无法伪造结果）。
func Pick(prizes []Prize, roll int) *Prize {
	if roll < 0 || roll >= ProbabilityScale {
		return nil
	}
	cursor := 0
	for index := range prizes {
		if !prizes[index].Drawable() {
			continue
		}
		cursor += prizes[index].Probability
		if roll < cursor {
			return &prizes[index]
		}
	}
	return nil
}

// Roll 返回 [0, ProbabilityScale) 的密码学随机数。
//
// 用 crypto/rand 而不是 math/rand：抽奖结果直接对应真金白银的奖品，
// 可预测的随机源等于把中奖概率送给能猜出种子的人。
func Roll() int {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(ProbabilityScale)))
	if err != nil {
		// 熵源不可用时退化为"未中奖"：宁可不发奖，也不要用可预测的随机数发奖。
		return ProbabilityScale - 1
	}
	return int(value.Int64())
}
