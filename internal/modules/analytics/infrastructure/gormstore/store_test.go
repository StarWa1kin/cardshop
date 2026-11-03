package gormstore

import (
	"fmt"
	"strings"
	"testing"

	analyticscontract "github.com/dujiao-next/internal/modules/analytics/contract"
	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAnalyticsStoreTest(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&analyticsdomain.VisitDaily{}, &analyticsdomain.VisitVisitorDaily{}); err != nil {
		t.Fatalf("migrate analytics models failed: %v", err)
	}
	return New(db), db
}

func TestApplyDailyDeltasAccumulatesExistingCounters(t *testing.T) {
	store, db := setupAnalyticsStoreTest(t)

	first := []analyticscontract.DailyDelta{
		{Scope: analyticsdomain.ScopeHome, TargetID: 0, StatDate: "2026-11-03", PV: 3, UV: 2},
		{Scope: analyticsdomain.ScopeProduct, TargetID: 7, StatDate: "2026-11-03", PV: 1, UV: 1},
	}
	if err := store.ApplyDailyDeltas(first); err != nil {
		t.Fatalf("apply first deltas failed: %v", err)
	}
	second := []analyticscontract.DailyDelta{
		{Scope: analyticsdomain.ScopeHome, TargetID: 0, StatDate: "2026-11-03", PV: 5, UV: 1},
	}
	if err := store.ApplyDailyDeltas(second); err != nil {
		t.Fatalf("apply second deltas failed: %v", err)
	}

	var home analyticsdomain.VisitDaily
	if err := db.Where("scope = ? AND target_id = ? AND stat_date = ?", analyticsdomain.ScopeHome, 0, "2026-11-03").
		First(&home).Error; err != nil {
		t.Fatalf("load home row failed: %v", err)
	}
	if home.PV != 8 || home.UV != 3 {
		t.Fatalf("home counters must accumulate, got pv=%d uv=%d", home.PV, home.UV)
	}

	var rows int64
	if err := db.Model(&analyticsdomain.VisitDaily{}).Count(&rows).Error; err != nil {
		t.Fatalf("count rows failed: %v", err)
	}
	if rows != 2 {
		t.Fatalf("upsert must keep one row per (scope,target,date), got %d", rows)
	}
}

func TestApplyDailyDeltasSkipsEmptyAndZeroDeltas(t *testing.T) {
	store, db := setupAnalyticsStoreTest(t)

	err := store.ApplyDailyDeltas([]analyticscontract.DailyDelta{
		{Scope: "", StatDate: "2026-11-03", PV: 1},
		{Scope: analyticsdomain.ScopeHome, StatDate: "", PV: 1},
		{Scope: analyticsdomain.ScopeHome, StatDate: "2026-11-03", PV: 0, UV: 0},
	})
	if err != nil {
		t.Fatalf("apply deltas failed: %v", err)
	}

	var rows int64
	if err := db.Model(&analyticsdomain.VisitDaily{}).Count(&rows).Error; err != nil {
		t.Fatalf("count rows failed: %v", err)
	}
	if rows != 0 {
		t.Fatalf("invalid deltas must not be written, got %d rows", rows)
	}
}

func TestMarkVisitorReportsOnlyFirstVisitPerDay(t *testing.T) {
	store, _ := setupAnalyticsStoreTest(t)

	fresh, err := store.MarkVisitor(analyticsdomain.ScopeHome, 0, "2026-11-03", "hash-a")
	if err != nil {
		t.Fatalf("mark visitor failed: %v", err)
	}
	if !fresh {
		t.Fatal("first visit of the day must be reported as new")
	}

	repeat, err := store.MarkVisitor(analyticsdomain.ScopeHome, 0, "2026-11-03", "hash-a")
	if err != nil {
		t.Fatalf("mark visitor again failed: %v", err)
	}
	if repeat {
		t.Fatal("repeat visit within the same day must not count as new")
	}

	nextDay, err := store.MarkVisitor(analyticsdomain.ScopeHome, 0, "2026-11-04", "hash-a")
	if err != nil {
		t.Fatalf("mark visitor next day failed: %v", err)
	}
	if !nextDay {
		t.Fatal("UV is a per-day metric, so the next day must count again")
	}
}

func TestGetProductViewTotalsFiltersScopeAndDateRange(t *testing.T) {
	store, _ := setupAnalyticsStoreTest(t)

	deltas := []analyticscontract.DailyDelta{
		{Scope: analyticsdomain.ScopeProduct, TargetID: 1, StatDate: "2026-11-01", PV: 2, UV: 2},
		{Scope: analyticsdomain.ScopeProduct, TargetID: 1, StatDate: "2026-11-03", PV: 4, UV: 3},
		{Scope: analyticsdomain.ScopeProduct, TargetID: 2, StatDate: "2026-11-03", PV: 9, UV: 5},
		{Scope: analyticsdomain.ScopeProduct, TargetID: 1, StatDate: "2026-11-09", PV: 100, UV: 90},
		{Scope: analyticsdomain.ScopeHome, TargetID: 0, StatDate: "2026-11-03", PV: 50, UV: 40},
	}
	if err := store.ApplyDailyDeltas(deltas); err != nil {
		t.Fatalf("apply deltas failed: %v", err)
	}

	rows, err := store.GetProductViewTotals([]uint{1}, "2026-11-01", "2026-11-03")
	if err != nil {
		t.Fatalf("get product view totals failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one product row, got %d", len(rows))
	}
	if rows[0].ProductID != 1 || rows[0].PV != 6 || rows[0].UV != 5 {
		t.Fatalf("unexpected totals: %+v", rows[0])
	}
}

func TestPruneVisitorsDropsFingerprintsBeforeDate(t *testing.T) {
	store, db := setupAnalyticsStoreTest(t)

	for _, statDate := range []string{"2026-10-20", "2026-10-27", "2026-11-03"} {
		if _, err := store.MarkVisitor(analyticsdomain.ScopeHome, 0, statDate, "hash-"+statDate); err != nil {
			t.Fatalf("mark visitor failed: %v", err)
		}
	}

	affected, err := store.PruneVisitors("2026-10-28")
	if err != nil {
		t.Fatalf("prune visitors failed: %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected two expired fingerprints removed, got %d", affected)
	}

	var remaining int64
	if err := db.Model(&analyticsdomain.VisitVisitorDaily{}).Count(&remaining).Error; err != nil {
		t.Fatalf("count fingerprints failed: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected one fingerprint left, got %d", remaining)
	}
}
