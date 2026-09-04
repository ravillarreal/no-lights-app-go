package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	srv, err := New(nil, nil, nil, 0.5, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv.Handler()
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestMetricsHeadersPresent(t *testing.T) {
	h := newTestServer(t)
	rr := do(t, h, http.MethodGet, "/api/stats/by-zone?field=invalid", "")

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
	if rr.Header().Get("X-Query-Count") != "0" {
		t.Fatalf("X-Query-Count = %q, want 0", rr.Header().Get("X-Query-Count"))
	}
	if rr.Header().Get("X-Process-Time-Ms") == "" {
		t.Fatal("X-Process-Time-Ms header missing")
	}
}

func TestReportarRejectsInvalidCoords(t *testing.T) {
	h := newTestServer(t)
	rr := do(t, h, http.MethodPost, "/api/reportar",
		`{"usuario_id":"u","longitud":200,"latitud":100,"tiene_luz":false}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestConsultarRadioRejectsInvalidCoords(t *testing.T) {
	h := newTestServer(t)
	rr := do(t, h, http.MethodGet, "/api/consultar-radio?lon=200&lat=100", "")
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestStatsByZoneRejectsInvalidField(t *testing.T) {
	h := newTestServer(t)
	rr := do(t, h, http.MethodGet, "/api/stats/by-zone?field=invalid", "")
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestMapPageRenders(t *testing.T) {
	h := newTestServer(t)
	rr := do(t, h, http.MethodGet, "/mapa", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "mapbox") {
		t.Fatalf("map page missing mapbox reference: %s", rr.Body.String())
	}
}

func TestReportResponseJSON(t *testing.T) {
	var r reportResponse
	_ = json.Unmarshal([]byte(`{"status":"reported","usuario_id":"u"}`), &r)
	if r.Status != "reported" || r.UserID != "u" {
		t.Fatalf("unexpected unmarshal: %+v", r)
	}
}
