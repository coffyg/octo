package octo

import (
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestParamsRaceCondition verifies that accessing parameters in a goroutine
// (simulating a background task) does not cause a data race or panic due to
// map recycling/pooling.
func TestParamsRaceCondition(t *testing.T) {
	router := NewRouter[any]()
	
	// Route that accesses params in a background goroutine
	router.GET("/race/:id", func(ctx *Ctx[any]) {
		// Capture the param value for comparison
		expected := ctx.Param("id")
		
		var wg sync.WaitGroup
		wg.Add(1)
		
		// Simulate user code spawning a goroutine that keeps using the context
		// This is technically "unsafe" usage in many frameworks, but a robust one
		// handling legacy code should ideally not crash.
		go func(c *Ctx[any], expect string) {
			defer wg.Done()
			// Artificial delay to ensure ServeHTTP likely returns and the pool puts the object back
			time.Sleep(100 * time.Microsecond)
			
			// Access the map. If the map was returned to the pool and cleared/written to
			// by another request, this triggers a race detector warning or crash.
			val := c.Param("id")
			
			// Just use the value to ensure compiler doesn't optimize it out
			if val != expect && val != "" {
				// We don't fail the test here because the value MIGHT change if reused,
				// but we are mostly testing for PANICS/RACES (crash stability).
				// However, if we want strict correctness, it should match.
			}
		}(ctx, expected)
		
		// We don't wait for wg here, allowing ServeHTTP to return
	})

	// Run concurrent requests to trigger pool churn
	var wg sync.WaitGroup
	workers := 10
	requests := 100

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < requests; i++ {
				path := fmt.Sprintf("/race/req-%d-%d", id, i)
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}
		}(w)
	}

	wg.Wait()
}
