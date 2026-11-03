package application

import (
	"context"
	"testing"

	analyticscontract "github.com/dujiao-next/internal/modules/analytics/contract"
	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
)

type productViewsRepoStub struct {
	rows        []analyticscontract.ProductViewRow
	lastIDs     []uint
	lastFrom    string
	lastTo      string
	queryCalled bool
}

func (s *productViewsRepoStub) ApplyDailyDeltas(deltas []analyticscontract.DailyDelta) error {
	return nil
}

func (s *productViewsRepoStub) MarkVisitor(scope string, targetID uint, statDate, visitorHash string) (bool, error) {
	return false, nil
}

func (s *productViewsRepoStub) GetProductViewTotals(productIDs []uint, fromDate, toDate string) ([]analyticscontract.ProductViewRow, error) {
	s.queryCalled = true
	s.lastIDs = productIDs
	s.lastFrom = fromDate
	s.lastTo = toDate
	return s.rows, nil
}

func (s *productViewsRepoStub) PruneVisitors(beforeDate string) (int64, error) {
	return 0, nil
}

func TestGetProductViewsDedupesRequestedIDs(t *testing.T) {
	repo := &productViewsRepoStub{rows: []analyticscontract.ProductViewRow{{ProductID: 3, PV: 12, UV: 7}}}
	service := NewService(repo)

	result, err := service.GetProductViews(context.Background(), ProductViewsQuery{
		Range:      reportingdomain.Query{Range: "7d"},
		ProductIDs: []uint{3, 3, 0, 5},
	})
	if err != nil {
		t.Fatalf("get product views failed: %v", err)
	}
	if len(repo.lastIDs) != 2 || repo.lastIDs[0] != 3 || repo.lastIDs[1] != 5 {
		t.Fatalf("ids must be deduped and zero-filtered, got %v", repo.lastIDs)
	}
	if repo.lastFrom == "" || repo.lastTo == "" || repo.lastFrom > repo.lastTo {
		t.Fatalf("unexpected date range %q..%q", repo.lastFrom, repo.lastTo)
	}
	if len(result.Items) != 1 || result.Items[0].ProductID != 3 || result.Items[0].PV != 12 || result.Items[0].UV != 7 {
		t.Fatalf("unexpected items: %+v", result.Items)
	}
}

func TestGetProductViewsSkipsQueryWithoutProductIDs(t *testing.T) {
	repo := &productViewsRepoStub{}
	service := NewService(repo)

	result, err := service.GetProductViews(context.Background(), ProductViewsQuery{Range: reportingdomain.Query{Range: "7d"}})
	if err != nil {
		t.Fatalf("get product views failed: %v", err)
	}
	if repo.queryCalled {
		t.Fatal("an empty id list must not hit the database")
	}
	if len(result.Items) != 0 {
		t.Fatalf("expected no items, got %+v", result.Items)
	}
}

func TestGetProductViewsRejectsInvalidRange(t *testing.T) {
	service := NewService(&productViewsRepoStub{})
	if _, err := service.GetProductViews(context.Background(), ProductViewsQuery{Range: reportingdomain.Query{Range: "decade"}}); err == nil {
		t.Fatal("invalid range must be rejected")
	}
}
