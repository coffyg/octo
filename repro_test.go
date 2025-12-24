package octo

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// AsyncResponseWriter simulates a writer that buffers data or writes it asynchronously,
// retaining the slice passed to Write() for a short period.
type AsyncResponseWriter struct {
	*httptest.ResponseRecorder
	writtenData [][]byte
	mu          sync.Mutex
}

func (w *AsyncResponseWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// CRITICAL: We retain the slice 'b' directly, NOT a copy.
	// Many high-performance middlewares or net implementations might do this
	// momentarily before flushing to a syscall.
	w.writtenData = append(w.writtenData, b)
	
	// Simulate IO delay where the slice must remain valid
	time.Sleep(1 * time.Millisecond)
	
	return w.ResponseRecorder.Write(b)
}

func TestSendJSON_BufferCorruption(t *testing.T) {
	router := NewRouter[any]()
	router.GET("/", func(ctx *Ctx[any]) {
		// Send a large enough JSON to use the pool
		data := make(map[string]string)
		for i := 0; i < 100; i++ {
			data["key"] = strings.Repeat("x", 100)
		}
		ctx.SendJSON(200, data)
	})

	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	// We need to detect if any written data changes *after* it was written
	// implying the underlying array was reused.
	
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				recorder := httptest.NewRecorder()
				w := &AsyncResponseWriter{ResponseRecorder: recorder}
				
				req := httptest.NewRequest("GET", "/", nil)
				router.ServeHTTP(w, req)
				
				// Verify data integrity
				w.mu.Lock()
				for _, slice := range w.writtenData {
					// Check if the slice still contains valid JSON start
					if len(slice) > 0 {
						if slice[0] != '{' {
							t.Errorf("Memory corruption detected! Expected '{', got %c", slice[0])
						}
					}
				}
				w.mu.Unlock()
			}
		}()
	}
	
	wg.Wait()
}
