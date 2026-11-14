package panelrpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrentRetryExecutesOnce(t *testing.T) {
	var writes atomic.Int32
	b := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writes.Add(1); w.Write([]byte(`{"saved":true}`)) }))
	req := Request{ID: "unique-request-0001", Epoch: b.epoch, Operation: "settings", Method: "PUT", Body: json.RawMessage(`{}`)}
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r := b.execute(req); r.Status != 200 {
				t.Errorf("reply %v", r)
			}
		}()
	}
	wg.Wait()
	if writes.Load() != 1 {
		t.Fatalf("writes %d", writes.Load())
	}
	req.Body = json.RawMessage(`{"other":1}`)
	if b.execute(req).Status != 409 {
		t.Fatal("ID reuse accepted")
	}
}

func TestBoundedWireAndCache(t *testing.T) {
	b := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }))
	for i := 0; i < Capacity; i++ {
		id := fmt.Sprintf("bounded-request-%04d", i)
		if got := b.execute(Request{ID: id, Operation: "status", Method: "GET"}); got.Status != 200 {
			t.Fatal(got)
		}
	}
	if got := b.execute(Request{ID: "bounded-request-extra", Operation: "status", Method: "GET"}); got.Status != 429 {
		t.Fatal(got)
	}
	w := httptest.NewRecorder()
	b.ServeHTTP(w, httptest.NewRequest("POST", "/rpc", strings.NewReader(strings.Repeat(" ", MaxWire+1))))
	if w.Code != 413 {
		t.Fatalf("oversize %d", w.Code)
	}
	w = httptest.NewRecorder()
	b.ServeHTTP(w, httptest.NewRequest("POST", "/rpc", strings.NewReader(`{"id":"bounded-test-123456","method":"GET","operation":"status"} {}`)))
	if w.Code != 400 {
		t.Fatalf("trailing JSON %d", w.Code)
	}
}
func TestEpochAndAllowlist(t *testing.T) {
	b := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }))
	req := Request{ID: "unique-request-0002", Operation: "settings", Method: "PUT", Body: json.RawMessage(`{}`)}
	if b.execute(req).Status != 409 {
		t.Fatal("mutation without epoch accepted")
	}
	req.Method = "GET"
	if b.execute(req).Status != 200 {
		t.Fatal("discovery read failed")
	}
	for _, operation := range []string{"rpc", "../../etc/passwd", "status?other", "shell"} {
		req.Operation = operation
		if Valid(req) {
			t.Fatal("bad operation allowed", operation)
		}
	}
	req.Operation = "status"
	req.Method = "PUT"
	if Valid(req) {
		t.Fatal("bad method allowed")
	}
}
