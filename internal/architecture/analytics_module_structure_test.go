package architecture

import (
	"path/filepath"
	"testing"
)

func TestAnalyticsImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "analytics")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	catalogReaderRoot := filepath.Join(moduleRoot, "infrastructure", "catalogreader")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "visit.go"), []string{"VisitDaily", "VisitVisitorDaily"})
	assertFileDeclaresFunctions(t, filepath.Join(domainRoot, "visit.go"), []string{
		"NormalizeScope", "FormatStatDate", "StatDateRange", "ShiftStatDate",
	})
	assertFileDeclaresFunctions(t, filepath.Join(domainRoot, "visitor.go"), []string{"BuildVisitorHash"})

	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"Repository", "ProductResolver", "DailyDelta", "ProductViewRow",
	})

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "collector.go"), []string{
		"Collector", "CollectorOptions", "TrackInput",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "collector.go"), []string{
		"NewCollector", "Start", "Close", "Track", "Flush",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService", "GetProductViews"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "types.go"), []string{
		"ProductViewsQuery", "ProductViewsResponse", "ProductViewItem",
	})

	storeFunctions := map[string][]string{
		"store.go":   {"New"},
		"daily.go":   {"ApplyDailyDeltas", "GetProductViewTotals"},
		"visitor.go": {"MarkVisitor", "PruneVisitors"},
	}
	for file, functions := range storeFunctions {
		assertFileDeclaresFunctions(t, filepath.Join(storeRoot, file), functions)
	}
	assertFileDeclaresFunctions(t, filepath.Join(catalogReaderRoot, "reader.go"), []string{"New", "ResolveProductID"})

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterPublicRoutes", "RegisterAdminRoutes"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "public_handler.go"), []string{"NewPublicHandler", "TrackVisit"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_handler.go"), []string{"NewAdminHandler", "GetProductViews"})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("analytics module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, domainRoot, 4)
	assertDirectoryGoFileBudget(t, contractRoot, 2)
	assertDirectoryGoFileBudget(t, applicationRoot, 6)
	assertDirectoryGoFileBudget(t, storeRoot, 6)
	assertDirectoryGoFileBudget(t, catalogReaderRoot, 2)
	assertDirectoryGoFileBudget(t, transportRoot, 6)
	assertProductionImportsAbsent(t, domainRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, transportRoot, moduleImportPath+"/internal/models")
}

// TestAnalyticsWriteModelStaysIndependentOfTransportAndGorm 守住采集链路的方向：
// 领域与契约不能反向依赖 HTTP 传输层或 ORM，否则埋点很容易被塞进 handler 里。
func TestAnalyticsWriteModelStaysIndependentOfTransportAndGorm(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "analytics")
	for _, directory := range []string{
		filepath.Join(moduleRoot, "domain"),
		filepath.Join(moduleRoot, "contract"),
		filepath.Join(moduleRoot, "application"),
	} {
		assertProductionImportsAbsent(t, directory, "gorm.io/gorm")
		assertProductionImportsAbsent(t, directory, "github.com/gin-gonic/gin")
	}
}
