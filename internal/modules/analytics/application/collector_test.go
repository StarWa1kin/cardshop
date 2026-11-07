package application

import (
	"errors"
	"sync"
	"testing"
	"time"

	analyticscontract "github.com/dujiao-next/internal/modules/analytics/contract"
	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"
)

type collectorRepoStub struct {
	mu           sync.Mutex
	applied      []analyticscontract.DailyDelta
	visitors     map[string]struct{}
	applyErr     error
	markErr      error
	prunedBefore []string
	applyCalls   int
}

func newCollectorRepoStub() *collectorRepoStub {
	return &collectorRepoStub{visitors: make(map[string]struct{})}
}

func (s *collectorRepoStub) ApplyDailyDeltas(deltas []analyticscontract.DailyDelta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyCalls++
	if s.applyErr != nil {
		return s.applyErr
	}
	s.applied = append(s.applied, deltas...)
	return nil
}

func (s *collectorRepoStub) MarkVisitor(scope string, targetID uint, statDate, visitorHash string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markErr != nil {
		return false, s.markErr
	}
	key := scope + "|" + statDate + "|" + visitorHash
	if _, exists := s.visitors[key]; exists {
		return false, nil
	}
	s.visitors[key] = struct{}{}
	return true, nil
}

func (s *collectorRepoStub) GetProductViewTotals(productIDs []uint, fromDate, toDate string) ([]analyticscontract.ProductViewRow, error) {
	return nil, nil
}

func (s *collectorRepoStub) PruneVisitors(beforeDate string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prunedBefore = append(s.prunedBefore, beforeDate)
	return 0, nil
}

func (s *collectorRepoStub) snapshot() ([]analyticscontract.DailyDelta, int, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	applied := make([]analyticscontract.DailyDelta, len(s.applied))
	copy(applied, s.applied)
	pruned := make([]string, len(s.prunedBefore))
	copy(pruned, s.prunedBefore)
	return applied, s.applyCalls, pruned
}

func (s *collectorRepoStub) clearApplyError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyErr = nil
}

type productResolverStub struct {
	ids map[string]uint
}

func (s productResolverStub) ResolveProductID(slug string) (uint, bool) {
	id, ok := s.ids[slug]
	return id, ok
}

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func TestCollectorCountsEveryPVButUVOncePerVisitorPerDay(t *testing.T) {
	repo := newCollectorRepoStub()
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	collector := NewCollector(CollectorOptions{Repository: repo, Salt: "salt", Clock: fixedClock(now)})

	for index := 0; index < 3; index++ {
		if err := collector.Track(TrackInput{Scope: analyticsdomain.ScopeHome, VisitorKey: "visitor-a"}); err != nil {
			t.Fatalf("track home failed: %v", err)
		}
	}
	if err := collector.Track(TrackInput{Scope: analyticsdomain.ScopeHome, VisitorKey: "visitor-b"}); err != nil {
		t.Fatalf("track second visitor failed: %v", err)
	}
	if err := collector.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	applied, _, _ := repo.snapshot()
	if len(applied) != 1 {
		t.Fatalf("one statistical unit expected, got %d: %+v", len(applied), applied)
	}
	if applied[0].PV != 4 {
		t.Fatalf("every view must count as PV, got %d", applied[0].PV)
	}
	if applied[0].UV != 2 {
		t.Fatalf("UV must count distinct visitors only, got %d", applied[0].UV)
	}
	if applied[0].StatDate != analyticsdomain.FormatStatDate(now) {
		t.Fatalf("unexpected stat date %q", applied[0].StatDate)
	}
}

func TestCollectorResolvesProductTargetAndRejectsUnknownOnes(t *testing.T) {
	repo := newCollectorRepoStub()
	collector := NewCollector(CollectorOptions{
		Repository: repo,
		Products:   productResolverStub{ids: map[string]uint{"known-product": 42}},
		Clock:      fixedClock(time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)),
	})

	if err := collector.Track(TrackInput{Scope: analyticsdomain.ScopeProduct, ProductSlug: "known-product", VisitorKey: "v"}); err != nil {
		t.Fatalf("track known product failed: %v", err)
	}
	if err := collector.Track(TrackInput{Scope: analyticsdomain.ScopeProduct, ProductSlug: "missing", VisitorKey: "v"}); !errors.Is(err, analyticsdomain.ErrTargetNotFound) {
		t.Fatalf("unknown product must be rejected, got %v", err)
	}
	if err := collector.Track(TrackInput{Scope: analyticsdomain.ScopeProduct, VisitorKey: "v"}); !errors.Is(err, analyticsdomain.ErrTargetInvalid) {
		t.Fatalf("missing target must be rejected, got %v", err)
	}
	if err := collector.Track(TrackInput{Scope: "blog", VisitorKey: "v"}); !errors.Is(err, analyticsdomain.ErrScopeInvalid) {
		t.Fatalf("unknown scope must be rejected, got %v", err)
	}
	if err := collector.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	applied, _, _ := repo.snapshot()
	if len(applied) != 1 || applied[0].TargetID != 42 || applied[0].PV != 1 {
		t.Fatalf("only the resolved product may be recorded, got %+v", applied)
	}
}

func TestCollectorKeepsPVWhenVisitorFingerprintFails(t *testing.T) {
	repo := newCollectorRepoStub()
	repo.markErr = errors.New("fingerprint table unavailable")
	collector := NewCollector(CollectorOptions{Repository: repo, Clock: fixedClock(time.Now())})

	if err := collector.Track(TrackInput{Scope: analyticsdomain.ScopeHome, VisitorKey: "visitor-a"}); err != nil {
		t.Fatalf("track must not fail when dedupe fails: %v", err)
	}
	if err := collector.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	applied, _, _ := repo.snapshot()
	if len(applied) != 1 || applied[0].PV != 1 || applied[0].UV != 0 {
		t.Fatalf("PV must survive a dedupe failure, got %+v", applied)
	}
}

func TestCollectorRetriesDeltasAfterFlushFailure(t *testing.T) {
	repo := newCollectorRepoStub()
	repo.applyErr = errors.New("database unavailable")
	collector := NewCollector(CollectorOptions{Repository: repo, Clock: fixedClock(time.Now())})

	if err := collector.Track(TrackInput{Scope: analyticsdomain.ScopeHome, VisitorKey: "visitor-a"}); err != nil {
		t.Fatalf("track failed: %v", err)
	}
	if err := collector.Flush(); err == nil {
		t.Fatal("flush must surface the persistence error")
	}

	repo.clearApplyError()
	if err := collector.Track(TrackInput{Scope: analyticsdomain.ScopeHome, VisitorKey: "visitor-b"}); err != nil {
		t.Fatalf("track failed: %v", err)
	}
	if err := collector.Flush(); err != nil {
		t.Fatalf("second flush failed: %v", err)
	}

	applied, _, _ := repo.snapshot()
	if len(applied) != 1 {
		t.Fatalf("expected the retried unit only once, got %+v", applied)
	}
	if applied[0].PV != 2 {
		t.Fatalf("failed deltas must be retried, got pv=%d", applied[0].PV)
	}
}

func TestCollectorPrunesExpiredFingerprintsOnFirstFlush(t *testing.T) {
	repo := newCollectorRepoStub()
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	collector := NewCollector(CollectorOptions{Repository: repo, Clock: fixedClock(now)})

	if err := collector.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if err := collector.Flush(); err != nil {
		t.Fatalf("second flush failed: %v", err)
	}

	_, _, pruned := repo.snapshot()
	if len(pruned) != 1 {
		t.Fatalf("prune must be throttled, got %v", pruned)
	}
	want := analyticsdomain.ShiftStatDate(now, -analyticsdomain.VisitorRetentionDays)
	if pruned[0] != want {
		t.Fatalf("prune cutoff must honour retention, got %q want %q", pruned[0], want)
	}
}

func TestCollectorFlushesWhenPendingUnitsExceedBudget(t *testing.T) {
	repo := newCollectorRepoStub()
	collector := NewCollector(CollectorOptions{
		Repository:     repo,
		Products:       productResolverStub{ids: map[string]uint{"a": 1, "b": 2, "c": 3}},
		MaxPendingKeys: 2,
		Clock:          fixedClock(time.Now()),
	})

	for _, slug := range []string{"a", "b", "c"} {
		if err := collector.Track(TrackInput{Scope: analyticsdomain.ScopeProduct, ProductSlug: slug, VisitorKey: "v"}); err != nil {
			t.Fatalf("track %s failed: %v", slug, err)
		}
	}

	_, applyCalls, _ := repo.snapshot()
	if applyCalls == 0 {
		t.Fatal("exceeding the pending budget must trigger an immediate flush")
	}
}
