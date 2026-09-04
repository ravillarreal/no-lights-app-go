// Command api is the No Lights HTTP server: REST API + server-rendered frontend.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ravillarreal/no-lights-app-go/internal/api"
	"github.com/ravillarreal/no-lights-app-go/internal/notify"
	"github.com/ravillarreal/no-lights-app-go/internal/store"
)

func main() {
	redisHost := getenv("REDIS_HOST", "localhost")
	redisPort := getenv("REDIS_PORT", "6379")
	radiusKM := getenvFloat("SEARCH_RADIUS_KM", 0.5)
	dbURL := os.Getenv("DATABASE_URL")
	token := os.Getenv("TELEGRAM_TOKEN")
	mapboxToken := os.Getenv("MAPBOX_TOKEN")
	addr := getenv("ADDR", ":8000")

	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{Addr: redisHost + ":" + redisPort})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("cannot connect to Redis at %s:%s: %v", redisHost, redisPort, err)
	}
	log.Println("connected to Redis")

	var st *store.Store
	if dbURL != "" {
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			log.Fatalf("cannot create pool: %v", err)
		}
		defer pool.Close()
		if err := pool.Ping(ctx); err != nil {
			log.Fatalf("cannot connect to PostgreSQL: %v", err)
		}
		st = store.New(pool)
		log.Println("connected to PostgreSQL")
	}

	var notif *notify.Client
	if st != nil && token != "" {
		notif = notify.New(token, st, radiusKM)
	}

	srv, err := api.New(st, rdb, notif, radiusKM, mapboxToken)
	if err != nil {
		log.Fatalf("server init: %v", err)
	}

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
