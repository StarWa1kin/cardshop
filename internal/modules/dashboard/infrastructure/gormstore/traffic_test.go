package gormstore

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func seedVisitDaily(t *testing.T, db *gorm.DB, rows []analyticsdomain.VisitDaily) {
	t.Helper()
	if err := db.AutoMigrate(&analyticsdomain.VisitDaily{}); err != nil {
		t.Fatalf("migrate visit stats failed: %v", err)
	}
	for index := range rows {
		row := rows[index]
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create visit row failed: %v", err)
		}
	}
}

func TestGetTrafficOverviewSplitsHomeAndProductCounters(t *testing.T) {
	store, db := setupDashboardRepositoryTest(t)
	seedVisitDaily(t, db, []analyticsdomain.VisitDaily{
		{Scope: analyticsdomain.ScopeHome, TargetID: 0, StatDate: "2026-11-02", PV: 10, UV: 6},
		{Scope: analyticsdomain.ScopeHome, TargetID: 0, StatDate: "2026-11-03", PV: 4, UV: 3},
		{Scope: analyticsdomain.ScopeProduct, TargetID: 1, StatDate: "2026-11-03", PV: 7, UV: 5},
		{Scope: analyticsdomain.ScopeHome, TargetID: 0, StatDate: "2026-11-09", PV: 99, UV: 99},
	})

	overview, err := store.GetTrafficOverview("2026-11-02", "2026-11-03")
	if err != nil {
		t.Fatalf("get traffic overview failed: %v", err)
	}
	if overview.HomePV != 14 || overview.HomeUV != 9 {
		t.Fatalf("unexpected home counters: %+v", overview)
	}
	if overview.ProductPV != 7 || overview.ProductUV != 5 {
		t.Fatalf("unexpected product counters: %+v", overview)
	}
}

func TestGetTrafficTrendsReturnsOneRowPerDay(t *testing.T) {
	store, db := setupDashboardRepositoryTest(t)
	seedVisitDaily(t, db, []analyticsdomain.VisitDaily{
		{Scope: analyticsdomain.ScopeHome, TargetID: 0, StatDate: "2026-11-02", PV: 10, UV: 6},
		{Scope: analyticsdomain.ScopeProduct, TargetID: 1, StatDate: "2026-11-02", PV: 3, UV: 2},
		{Scope: analyticsdomain.ScopeHome, TargetID: 0, StatDate: "2026-11-03", PV: 4, UV: 3},
	})

	rows, err := store.GetTrafficTrends("2026-11-02", "2026-11-03")
	if err != nil {
		t.Fatalf("get traffic trends failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two daily rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].Day != "2026-11-02" || rows[0].HomePV != 10 || rows[0].ProductPV != 3 {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
	if rows[1].Day != "2026-11-03" || rows[1].HomePV != 4 || rows[1].ProductPV != 0 {
		t.Fatalf("unexpected second row: %+v", rows[1])
	}
}

func TestGetTopProductViewsRanksByPageViewsWithTitle(t *testing.T) {
	store, db := setupDashboardRepositoryTest(t)
	category := createDashboardCategory(t, db, "traffic-category")

	products := map[string]string{"popular": "热门商品", "quiet": "冷门商品"}
	ids := make(map[string]uint, len(products))
	for slug, title := range products {
		product := &productdomain.Product{
			CategoryID:      category.ID,
			Slug:            slug,
			TitleJSON:       jsonmap.JSON{"zh-CN": title},
			PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
			PurchaseType:    constants.ProductPurchaseMember,
			FulfillmentType: constants.FulfillmentTypeManual,
			IsActive:        true,
		}
		if err := db.Create(product).Error; err != nil {
			t.Fatalf("create product failed: %v", err)
		}
		ids[slug] = product.ID
	}

	seedVisitDaily(t, db, []analyticsdomain.VisitDaily{
		{Scope: analyticsdomain.ScopeProduct, TargetID: ids["popular"], StatDate: "2026-11-03", PV: 30, UV: 20},
		{Scope: analyticsdomain.ScopeProduct, TargetID: ids["quiet"], StatDate: "2026-11-03", PV: 2, UV: 2},
		{Scope: analyticsdomain.ScopeProduct, TargetID: 9999, StatDate: "2026-11-03", PV: 500, UV: 400},
		{Scope: analyticsdomain.ScopeHome, TargetID: 0, StatDate: "2026-11-03", PV: 999, UV: 999},
	})

	rows, err := store.GetTopProductViews("2026-11-03", "2026-11-03", 5)
	if err != nil {
		t.Fatalf("get top product views failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows must be limited to existing products, got %d: %+v", len(rows), rows)
	}
	if rows[0].ProductID != ids["popular"] || rows[0].PV != 30 || rows[0].UV != 20 {
		t.Fatalf("unexpected top row: %+v", rows[0])
	}
	if rows[0].Title != "热门商品" {
		t.Fatalf("ranking must carry the localized title, got %q", rows[0].Title)
	}
}
