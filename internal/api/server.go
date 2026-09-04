// Package api exposes the HTTP server: REST endpoints (JSON) plus the
// server-rendered web frontend and static assets.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/ravillarreal/no-lights-app-go/internal/geo"
	"github.com/ravillarreal/no-lights-app-go/internal/geocode"
	"github.com/ravillarreal/no-lights-app-go/internal/metrics"
	"github.com/ravillarreal/no-lights-app-go/internal/notify"
	"github.com/ravillarreal/no-lights-app-go/internal/store"
	"github.com/ravillarreal/no-lights-app-go/web"
)

type Server struct {
	router      *chi.Mux
	store       *store.Store
	geo         *geo.Client
	notify      *notify.Client
	radiusKM    float64
	mapboxToken string
	templates   *template.Template
}

func New(s *store.Store, rdb *redis.Client, notif *notify.Client, radiusKM float64, mapboxToken string) (*Server, error) {
	tmpl, err := template.ParseFS(web.FS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	srv := &Server{
		router:      chi.NewRouter(),
		store:       s,
		geo:         geo.New(rdb),
		notify:      notif,
		radiusKM:    radiusKM,
		mapboxToken: mapboxToken,
		templates:   tmpl,
	}
	srv.routes()
	return srv, nil
}

func (s *Server) Handler() http.Handler {
	return s.metricsMiddleware(s.router)
}

func (s *Server) routes() {
	s.router.Route("/api", func(r chi.Router) {
		r.Post("/reportar", s.reportar)
		r.Get("/consultar-radio", s.consultarRadio)
		r.Get("/stats/summary", s.statsSummary)
		r.Get("/stats/by-day", s.statsByDay)
		r.Get("/stats/by-zone", s.statsByZone)
	})

	s.router.Get("/", s.page("map.html"))
	s.router.Get("/mapa", s.page("map.html"))
	s.router.Get("/dashboard", s.page("dashboard.html"))
	s.router.Get("/login", s.page("login.html"))
	s.router.Handle("/static/*", http.FileServer(http.FS(mustSub(web.FS, "static"))))
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// pageData carries the server-injected values available to every template.
type pageData struct {
	MapboxToken string
	RadiusKM    float64
}

func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := pageData{MapboxToken: s.mapboxToken, RadiusKM: s.radiusKM}
		if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// ─── Middleware: query count + latency (RED method) ──────────────────────────

type metricsWriter struct {
	http.ResponseWriter
	ctx   context.Context
	start time.Time
	done  bool
}

func (w *metricsWriter) WriteHeader(code int) {
	if !w.done {
		w.done = true
		elapsed := time.Since(w.start).Milliseconds()
		w.Header().Set("X-Process-Time-Ms", strconv.FormatInt(elapsed, 10))
		w.Header().Set("X-Query-Count", strconv.FormatInt(metrics.QueryCount(w.ctx), 10))
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *metricsWriter) Write(b []byte) (int, error) {
	if !w.done {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (s *Server) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := metrics.WithCounter(r.Context())
		next.ServeHTTP(&metricsWriter{ResponseWriter: w, ctx: ctx, start: time.Now()}, r.WithContext(ctx))
	})
}

// ─── Handlers ────────────────────────────────────────────────────────────────

type reportPayload struct {
	UserID    string  `json:"usuario_id"`
	Longitude float64 `json:"longitud"`
	Latitude  float64 `json:"latitud"`
	TieneLuz  bool    `json:"tiene_luz"`
}

type reportResponse struct {
	Status string `json:"status"`
	UserID string `json:"usuario_id"`
}

func (s *Server) reportar(w http.ResponseWriter, r *http.Request) {
	var p reportPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	if p.UserID == "" || p.Longitude < -180 || p.Longitude > 180 || p.Latitude < -90 || p.Latitude > 90 {
		writeErr(w, http.StatusUnprocessableEntity, "campos fuera de rango")
		return
	}

	ctx := r.Context()

	if !p.TieneLuz {
		if err := s.geo.Add(ctx, p.UserID, p.Longitude, p.Latitude); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if s.store != nil {
			id, err := s.store.OutageStart(ctx, p.UserID, p.Longitude, p.Latitude)
			if err != nil {
				log.Printf("reportar: OutageStart failed: %v", err)
			} else {
				go s.reverseGeocode(id, p.Longitude, p.Latitude)
			}
		}
		go s.notifyNearby(p.Longitude, p.Latitude, false)
		writeJSON(w, http.StatusOK, reportResponse{Status: "reported", UserID: p.UserID})
		return
	}

	removed, err := s.geo.Remove(ctx, p.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if removed && s.store != nil {
		if _, err := s.store.OutageEnd(ctx, p.UserID); err != nil {
			log.Printf("reportar: OutageEnd failed: %v", err)
		}
	}
	if removed {
		go s.notifyNearby(p.Longitude, p.Latitude, true)
		writeJSON(w, http.StatusOK, reportResponse{Status: "removed", UserID: p.UserID})
		return
	}
	writeJSON(w, http.StatusOK, reportResponse{Status: "not_found", UserID: p.UserID})
}

func (s *Server) reverseGeocode(eventID int, lon, lat float64) {
	addr, raw, err := geocode.Lookup(context.Background(), lon, lat)
	if err != nil {
		log.Printf("reverse geocoding failed for event %d: %v", eventID, err)
		return
	}
	if err := s.store.SetGeo(context.Background(), eventID, addr.Neighborhood, addr.Municipality, addr.State, addr.Country, raw); err != nil {
		log.Printf("SetGeo failed for event %d: %v", eventID, err)
	}
}

func (s *Server) notifyNearby(lon, lat float64, tieneLuz bool) {
	if s.notify != nil {
		s.notify.NotifyNearby(context.Background(), lon, lat, tieneLuz)
	}
}

type outagePoint struct {
	UserID    string  `json:"usuario_id"`
	Longitude float64 `json:"longitud"`
	Latitude  float64 `json:"latitud"`
}

type radiusResponse struct {
	Count    int           `json:"count"`
	RadiusKM float64       `json:"radio_km"`
	Points   []outagePoint `json:"points"`
}

func (s *Server) consultarRadio(w http.ResponseWriter, r *http.Request) {
	lon, err1 := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	lat, err2 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	if err1 != nil || err2 != nil || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
		writeErr(w, http.StatusUnprocessableEntity, "coordenadas fuera de rango")
		return
	}

	points, err := s.geo.Nearby(r.Context(), lon, lat, s.radiusKM)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := radiusResponse{Count: len(points), RadiusKM: s.radiusKM, Points: make([]outagePoint, 0, len(points))}
	for _, p := range points {
		out.Points = append(out.Points, outagePoint{UserID: p.UserID, Longitude: p.Longitude, Latitude: p.Latitude})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) statsSummary(w http.ResponseWriter, r *http.Request) {
	days := intQuery(r, "days", 30)
	if s.store == nil {
		writeJSON(w, http.StatusOK, store.Summary{})
		return
	}
	sum, err := s.store.StatsSummary(r.Context(), days)
	if err != nil {
		log.Printf("stats_summary: %v", err)
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (s *Server) statsByDay(w http.ResponseWriter, r *http.Request) {
	days := intQuery(r, "days", 30)
	if s.store == nil {
		writeJSON(w, http.StatusOK, []store.DayStat{})
		return
	}
	out, err := s.store.StatsByDay(r.Context(), days)
	if err != nil {
		log.Printf("stats_by_day: %v", err)
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) statsByZone(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	if field == "" {
		field = "neighborhood"
	}
	if field != "neighborhood" && field != "municipality" && field != "state" {
		writeErr(w, http.StatusUnprocessableEntity, "field debe ser neighborhood, municipality o state")
		return
	}
	days := intQuery(r, "days", 30)
	limit := intQuery(r, "limit", 10)
	if s.store == nil {
		writeJSON(w, http.StatusOK, []store.ZoneStat{})
		return
	}
	out, err := s.store.StatsByZone(r.Context(), field, days, limit)
	if err != nil {
		log.Printf("stats_by_zone: %v", err)
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func intQuery(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"detail": msg})
}
