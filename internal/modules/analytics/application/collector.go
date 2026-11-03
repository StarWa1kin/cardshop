package application

import (
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/logger"
	analyticscontract "github.com/dujiao-next/internal/modules/analytics/contract"
	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"
)

const (
	defaultFlushInterval  = 10 * time.Second
	defaultMaxPendingKeys = 2000
	prunePeriod           = 6 * time.Hour
)

// TrackInput 是一次页面访问上报。
type TrackInput struct {
	Scope       string
	ProductSlug string
	VisitorKey  string
	ClientIP    string
	UserAgent   string
}

// CollectorOptions 是采集器依赖。
type CollectorOptions struct {
	Repository     analyticscontract.Repository
	Products       analyticscontract.ProductResolver
	Salt           string
	FlushInterval  time.Duration
	MaxPendingKeys int
	Clock          func() time.Time
}

type dailyKey struct {
	scope    string
	targetID uint
	statDate string
}

// Collector 收集页面访问。
//
// UV 需要判重，所以指纹写入是同步的（命中唯一索引，单次写很轻）；PV 只是计数，
// 先在内存里按 (scope, target, date) 累加，再定期批量 UPSERT，避免每次浏览都打一次库。
type Collector struct {
	repo           analyticscontract.Repository
	products       analyticscontract.ProductResolver
	salt           string
	flushInterval  time.Duration
	maxPendingKeys int
	clock          func() time.Time

	mu           sync.Mutex
	pending      map[dailyKey]*analyticscontract.DailyDelta
	lastPrunedAt time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewCollector 创建访问采集器。
func NewCollector(opts CollectorOptions) *Collector {
	interval := opts.FlushInterval
	if interval <= 0 {
		interval = defaultFlushInterval
	}
	maxPendingKeys := opts.MaxPendingKeys
	if maxPendingKeys <= 0 {
		maxPendingKeys = defaultMaxPendingKeys
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Collector{
		repo:           opts.Repository,
		products:       opts.Products,
		salt:           opts.Salt,
		flushInterval:  interval,
		maxPendingKeys: maxPendingKeys,
		clock:          clock,
		pending:        make(map[dailyKey]*analyticscontract.DailyDelta),
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

// Start 启动后台刷盘循环，重复调用无副作用。
func (c *Collector) Start() {
	if c == nil || c.repo == nil {
		return
	}
	c.startOnce.Do(func() {
		go c.loop()
	})
}

// Close 停止刷盘循环并把内存中剩余的增量落库。
func (c *Collector) Close() {
	if c == nil || c.repo == nil {
		return
	}
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	select {
	case <-c.doneCh:
	case <-time.After(5 * time.Second):
		logger.Warnw("analytics_collector_close_timeout")
	}
}

// Track 记录一次页面访问。
func (c *Collector) Track(input TrackInput) error {
	if c == nil || c.repo == nil {
		return nil
	}
	scope := analyticsdomain.NormalizeScope(input.Scope)
	if scope == "" {
		return analyticsdomain.ErrScopeInvalid
	}

	targetID := uint(0)
	if scope == analyticsdomain.ScopeProduct {
		slug := strings.TrimSpace(input.ProductSlug)
		if slug == "" {
			return analyticsdomain.ErrTargetInvalid
		}
		if c.products == nil {
			return analyticsdomain.ErrTargetNotFound
		}
		resolved, ok := c.products.ResolveProductID(slug)
		if !ok || resolved == 0 {
			return analyticsdomain.ErrTargetNotFound
		}
		targetID = resolved
	}

	now := c.now()
	statDate := analyticsdomain.FormatStatDate(now)
	visitorHash := analyticsdomain.BuildVisitorHash(c.salt, statDate, input.VisitorKey, input.ClientIP, input.UserAgent)

	uvDelta := int64(0)
	fresh, err := c.repo.MarkVisitor(scope, targetID, statDate, visitorHash)
	if err != nil {
		// 指纹写失败只会少算 UV，不能连 PV 一起丢。
		logger.Warnw("analytics_mark_visitor_failed", "scope", scope, "target_id", targetID, "error", err)
	} else if fresh {
		uvDelta = 1
	}

	c.enqueue(dailyKey{scope: scope, targetID: targetID, statDate: statDate}, 1, uvDelta)
	return nil
}

// Flush 把内存中的增量批量累加到按天聚合表。
func (c *Collector) Flush() error {
	if c == nil || c.repo == nil {
		return nil
	}
	deltas := c.drain()
	if len(deltas) == 0 {
		c.maybePrune()
		return nil
	}
	if err := c.repo.ApplyDailyDeltas(deltas); err != nil {
		c.restore(deltas)
		return err
	}
	c.maybePrune()
	return nil
}

func (c *Collector) loop() {
	defer close(c.doneCh)
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.Flush(); err != nil {
				logger.Warnw("analytics_flush_failed", "error", err)
			}
		case <-c.stopCh:
			if err := c.Flush(); err != nil {
				logger.Warnw("analytics_final_flush_failed", "error", err)
			}
			return
		}
	}
}

func (c *Collector) enqueue(key dailyKey, pv, uv int64) {
	c.mu.Lock()
	entry, exists := c.pending[key]
	if exists {
		entry.PV += pv
		entry.UV += uv
		c.mu.Unlock()
		return
	}
	overflow := len(c.pending) >= c.maxPendingKeys
	c.pending[key] = &analyticscontract.DailyDelta{
		Scope:    key.scope,
		TargetID: key.targetID,
		StatDate: key.statDate,
		PV:       pv,
		UV:       uv,
	}
	c.mu.Unlock()

	if overflow {
		// 统计单元数量超出预算（例如被大量商品页同时打爆），立即落盘以限制内存。
		if err := c.Flush(); err != nil {
			logger.Warnw("analytics_overflow_flush_failed", "error", err)
		}
	}
}

func (c *Collector) drain() []analyticscontract.DailyDelta {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pending) == 0 {
		return nil
	}
	deltas := make([]analyticscontract.DailyDelta, 0, len(c.pending))
	for _, entry := range c.pending {
		deltas = append(deltas, *entry)
	}
	c.pending = make(map[dailyKey]*analyticscontract.DailyDelta)
	return deltas
}

// restore 把落库失败的增量放回内存，等下一个窗口重试；超出预算的部分丢弃，
// 否则数据库长时间不可用会把增量无限堆在内存里。
func (c *Collector) restore(deltas []analyticscontract.DailyDelta) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for index := range deltas {
		delta := deltas[index]
		key := dailyKey{scope: delta.Scope, targetID: delta.TargetID, statDate: delta.StatDate}
		if entry, exists := c.pending[key]; exists {
			entry.PV += delta.PV
			entry.UV += delta.UV
			continue
		}
		if len(c.pending) >= c.maxPendingKeys {
			logger.Warnw("analytics_restore_dropped", "scope", delta.Scope, "target_id", delta.TargetID, "pv", delta.PV, "uv", delta.UV)
			continue
		}
		copied := delta
		c.pending[key] = &copied
	}
}

// maybePrune 清理过期的 UV 指纹。
//
// 这里不挂异步队列：队列依赖 Redis，而采集器在任何部署形态下都在跑，
// 顺手清理比多一个可选依赖更可靠。
func (c *Collector) maybePrune() {
	now := c.now()
	c.mu.Lock()
	if !c.lastPrunedAt.IsZero() && now.Sub(c.lastPrunedAt) < prunePeriod {
		c.mu.Unlock()
		return
	}
	c.lastPrunedAt = now
	c.mu.Unlock()

	beforeDate := analyticsdomain.ShiftStatDate(now, -analyticsdomain.VisitorRetentionDays)
	affected, err := c.repo.PruneVisitors(beforeDate)
	if err != nil {
		logger.Warnw("analytics_prune_visitors_failed", "before_date", beforeDate, "error", err)
		return
	}
	if affected > 0 {
		logger.Debugw("analytics_prune_visitors_ok", "before_date", beforeDate, "affected", affected)
	}
}

func (c *Collector) now() time.Time {
	if c.clock == nil {
		return time.Now()
	}
	return c.clock()
}
