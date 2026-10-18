package ops

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	catalogMu   sync.RWMutex
	catalogOps  []Operation
	catalogByID map[string]Operation
	engine      *gin.Engine
)

func Initialize(e *gin.Engine) {
	engine = e
	BuildCatalog(e.Routes())
}

func ResetForTest() {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	engine = nil
	catalogOps = nil
	catalogByID = nil
}

func Engine() *gin.Engine {
	return engine
}

// BuildCatalog classifies every dashboard API route. Routes that are not under
// /api are ignored; /api routes must either become an Operation or carry an
// explicit exclusion reason. The catalog test fails on unclassified routes.
func BuildCatalog(routes []gin.RouteInfo) {
	ops := make([]Operation, 0, len(routes))
	byID := make(map[string]Operation, len(routes))
	for _, route := range routes {
		if !strings.HasPrefix(route.Path, "/api/") && route.Path != "/api" {
			continue
		}
		op := classifyRoute(route.Method, route.Path)
		if op.ExcludedReason != "" {
			continue
		}
		if op.ID == "" {
			op.ID = route.Method + " " + route.Path
		}
		ops = append(ops, op)
		byID[op.ID] = op
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
	catalogMu.Lock()
	catalogOps = ops
	catalogByID = byID
	catalogMu.Unlock()
}

func Find(id string) (Operation, bool) {
	catalogMu.RLock()
	defer catalogMu.RUnlock()
	op, ok := catalogByID[id]
	return op, ok
}

func All() []Operation {
	catalogMu.RLock()
	defer catalogMu.RUnlock()
	return append([]Operation(nil), catalogOps...)
}

func ExcludedCount(routes []gin.RouteInfo) int {
	count := 0
	for _, route := range routes {
		if !strings.HasPrefix(route.Path, "/api/") && route.Path != "/api" {
			continue
		}
		if classifyRoute(route.Method, route.Path).ExcludedReason != "" {
			count++
		}
	}
	return count
}

// Visible reports whether one operation may be discovered by a profile/role.
// The real route middleware remains authoritative when executing.
func Visible(op Operation, profile string, role int) bool {
	if op.ExcludedReason != "" {
		return false
	}
	if profile == "user" {
		return op.Scope == ScopeUser && role >= op.MinRole
	}
	if profile == "admin" {
		return role >= op.MinRole
	}
	return false
}

func List(profile string, role int, domain, query string, limit int) []Operation {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query = strings.ToLower(strings.TrimSpace(query))
	domain = strings.ToLower(strings.TrimSpace(domain))
	result := make([]Operation, 0, limit)
	for _, op := range All() {
		if !Visible(op, profile, role) {
			continue
		}
		if domain != "" && strings.ToLower(op.Domain) != domain {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(op.ID + " " + op.Summary + " " + op.Description)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		result = append(result, op)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func operationError(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
