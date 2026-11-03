package provider

import (
	"net/http"
	"testing"
)

// TestCoveredOperationsIncludeEveryResourceAndDataSource ensures new specs are
// reflected in the operation map used by cmd/check-api-coverage.
func TestCoveredOperationsIncludeEveryResourceAndDataSource(t *testing.T) {
	methods := map[string]map[string]bool{}
	for _, op := range CoveredOperations() {
		for _, owner := range op.Owners {
			if methods[owner] == nil {
				methods[owner] = map[string]bool{}
			}
			methods[owner][op.Method] = true
		}
	}
	for _, s := range dataSpecs() {
		if !methods["data.mergeah_"+s.Name][http.MethodGet] {
			t.Errorf("data source %s has no GET operation", s.Name)
		}
	}
	for _, s := range resourceSpecs() {
		m := methods["mergeah_"+s.Name]
		if !m[http.MethodGet] {
			t.Errorf("resource %s has no read operation", s.Name)
		}
		if len(m) < 2 {
			t.Errorf("resource %s has no write operation: %v", s.Name, m)
		}
	}
}
