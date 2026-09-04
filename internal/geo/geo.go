// Package geo wraps Redis Geo commands for the real-time outage map.
package geo

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Key is the single Redis key holding all active outage points (no TTL —
// points are removed explicitly when light is restored).
const Key = "outages:geo"

// Point is one outage location returned by a radius query.
type Point struct {
	UserID    string  `json:"usuario_id"`
	Longitude float64 `json:"longitud"`
	Latitude  float64 `json:"latitud"`
}

type Client struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Client {
	return &Client{rdb: rdb}
}

// Add stores an outage point at (lon, lat) keyed by userID.
func (c *Client) Add(ctx context.Context, userID string, lon, lat float64) error {
	return c.rdb.GeoAdd(ctx, Key, &redis.GeoLocation{
		Name:      userID,
		Longitude: lon,
		Latitude:  lat,
	}).Err()
}

// Remove deletes a user's point and reports whether it existed.
func (c *Client) Remove(ctx context.Context, userID string) (bool, error) {
	n, err := c.rdb.ZRem(ctx, Key, userID).Result()
	return n > 0, err
}

// Nearby returns all outage points within radiusKM of (lon, lat).
func (c *Client) Nearby(ctx context.Context, lon, lat, radiusKM float64) ([]Point, error) {
	res, err := c.rdb.GeoSearchLocation(ctx, Key, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lon,
			Latitude:   lat,
			Radius:     radiusKM,
			RadiusUnit: "km",
		},
		WithCoord: true,
	}).Result()
	if err != nil {
		return nil, err
	}
	points := make([]Point, 0, len(res))
	for _, loc := range res {
		points = append(points, Point{
			UserID:    loc.Name,
			Longitude: loc.Longitude,
			Latitude:  loc.Latitude,
		})
	}
	return points, nil
}
