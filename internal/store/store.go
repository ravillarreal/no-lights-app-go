// Package store persists outage events and user locations in
// PostgreSQL/PostGIS via pgx. Every query bumps the metrics query counter.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravillarreal/no-lights-app-go/internal/metrics"
)

type Store struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool}
}

// OutageStart records a new outage event and returns its id.
func (s *Store) OutageStart(ctx context.Context, userID string, lon, lat float64) (int, error) {
	metrics.BumpQuery(ctx)
	var id int
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO outage_events (usuario_id, geom)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography)
		RETURNING id`,
		userID, lon, lat,
	).Scan(&id)
	return id, err
}

// OutageEnd closes the open outage for a user, returning its id (nil if none).
func (s *Store) OutageEnd(ctx context.Context, userID string) (*int, error) {
	metrics.BumpQuery(ctx)
	var id int
	err := s.Pool.QueryRow(ctx, `
		UPDATE outage_events
		SET ended_at = NOW()
		WHERE usuario_id = $1 AND ended_at IS NULL
		RETURNING id`,
		userID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// SetGeo fills the reverse-geocoded fields for an event.
func (s *Store) SetGeo(ctx context.Context, id int, neighborhood, municipality, state, country, raw string) error {
	metrics.BumpQuery(ctx)
	_, err := s.Pool.Exec(ctx, `
		UPDATE outage_events
		SET neighborhood = $1, municipality = $2, state = $3, country = $4, raw_address = $5
		WHERE id = $6`,
		neighborhood, municipality, state, country, raw, id,
	)
	return err
}

// Summary is the aggregate dashboard snapshot.
type Summary struct {
	TotalCompleted int64    `json:"total_completed"`
	ActiveOutages  int64    `json:"active_outages"`
	AvgDurationMin *float64 `json:"avg_duration_min"`
	AffectedZones  int64    `json:"affected_zones"`
}

// StatsSummary returns aggregated stats for the last `days`.
func (s *Store) StatsSummary(ctx context.Context, days int) (Summary, error) {
	metrics.BumpQuery(ctx)
	var out Summary
	err := s.Pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE ended_at IS NOT NULL) AS total_completed,
		  COUNT(*) FILTER (WHERE ended_at IS NULL)     AS active_outages,
		  ROUND(AVG(duration_min) FILTER (WHERE ended_at IS NOT NULL)::numeric, 1) AS avg_duration_min,
		  COUNT(DISTINCT COALESCE(neighborhood, municipality)) FILTER (WHERE ended_at IS NOT NULL) AS affected_zones
		FROM outage_events
		WHERE started_at >= NOW() - ($1 * INTERVAL '1 day')`,
		days,
	).Scan(&out.TotalCompleted, &out.ActiveOutages, &out.AvgDurationMin, &out.AffectedZones)
	return out, err
}

// DayStat is one daily bucket of completed outages.
type DayStat struct {
	Date           string  `json:"date"`
	Outages        int64   `json:"outages"`
	AvgDurationMin float64 `json:"avg_duration_min"`
}

// StatsByDay returns daily completed-outage counts for the last `days`.
func (s *Store) StatsByDay(ctx context.Context, days int) ([]DayStat, error) {
	metrics.BumpQuery(ctx)
	rows, err := s.Pool.Query(ctx, `
		SELECT
		  DATE_TRUNC('day', started_at)::date AS date,
		  COUNT(*) AS outages,
		  ROUND(COALESCE(AVG(duration_min), 0)::numeric, 1) AS avg_duration_min
		FROM outage_events
		WHERE started_at >= NOW() - ($1 * INTERVAL '1 day')
		  AND ended_at IS NOT NULL
		GROUP BY DATE_TRUNC('day', started_at)::date
		ORDER BY date`,
		days,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DayStat
	for rows.Next() {
		var d DayStat
		if err := rows.Scan(&d.Date, &d.Outages, &d.AvgDurationMin); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ZoneStat is one zone bucket of completed outages.
type ZoneStat struct {
	Zone           string  `json:"zone"`
	Outages        int64   `json:"outages"`
	AvgDurationMin float64 `json:"avg_duration_min"`
}

// validZoneFields mirrors the whitelist in the API handler.
var validZoneFields = map[string]string{
	"neighborhood": "COALESCE(neighborhood, municipality, state, country)",
	"municipality": "COALESCE(municipality, state, country)",
	"state":        "COALESCE(state, country)",
}

// StatsByZone returns top zones for the last `days`. field must be one of
// neighborhood, municipality or state (validated by the caller).
func (s *Store) StatsByZone(ctx context.Context, field string, days, limit int) ([]ZoneStat, error) {
	expr, ok := validZoneFields[field]
	if !ok {
		return nil, errors.New("invalid zone field")
	}
	metrics.BumpQuery(ctx)
	// expr is a whitelisted constant from validZoneFields, never user input.
	rows, err := s.Pool.Query(ctx, `
		SELECT
		  COALESCE(`+expr+`, '(sin datos)') AS zone,
		  COUNT(*) AS outages,
		  ROUND(COALESCE(AVG(duration_min), 0)::numeric, 1) AS avg_duration_min
		FROM outage_events
		WHERE started_at >= NOW() - ($1 * INTERVAL '1 day')
		  AND ended_at IS NOT NULL
		GROUP BY `+expr+`
		ORDER BY outages DESC
		LIMIT $2`,
		days, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ZoneStat
	for rows.Next() {
		var z ZoneStat
		if err := rows.Scan(&z.Zone, &z.Outages, &z.AvgDurationMin); err != nil {
			return nil, err
		}
		out = append(out, z)
	}
	return out, rows.Err()
}

// SaveLocation stores a Telegram user's monitored location.
func (s *Store) SaveLocation(ctx context.Context, userID, chatID int64, firstName, label string, lon, lat float64) (int, error) {
	metrics.BumpQuery(ctx)
	var id int
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO user_locations (telegram_user_id, chat_id, first_name, label, geom)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography)
		RETURNING id`,
		userID, chatID, firstName, label, lon, lat,
	).Scan(&id)
	return id, err
}

// UserLocation is a saved location for a Telegram user.
type UserLocation struct {
	ID      int     `json:"id"`
	Label   string  `json:"label"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	ChatID  int64   `json:"chat_id"`
	UserID  int64   `json:"user_id"`
	HasName bool
}

// UserLocations returns the saved locations for a Telegram user.
func (s *Store) UserLocations(ctx context.Context, userID int64, limit int) ([]UserLocation, error) {
	metrics.BumpQuery(ctx)
	rows, err := s.Pool.Query(ctx, `
		SELECT id, label, ST_Y(geom::geometry) AS lat, ST_X(geom::geometry) AS lon, chat_id
		FROM user_locations
		WHERE telegram_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserLocation
	for rows.Next() {
		var l UserLocation
		if err := rows.Scan(&l.ID, &l.Label, &l.Lat, &l.Lon, &l.ChatID); err != nil {
			return nil, err
		}
		l.UserID = userID
		out = append(out, l)
	}
	return out, rows.Err()
}

// DeleteLocation removes a location owned by userID, returning its label.
func (s *Store) DeleteLocation(ctx context.Context, userID int64, id int) (string, bool, error) {
	metrics.BumpQuery(ctx)
	var label string
	err := s.Pool.QueryRow(ctx, `
		DELETE FROM user_locations
		WHERE id = $1 AND telegram_user_id = $2
		RETURNING label`,
		id, userID,
	).Scan(&label)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return label, true, nil
}

// NearbyUsers are Telegram users with a saved location within radius of
// (lon, lat), filtered by the notification cooldown.
type NearbyUser struct {
	TelegramUserID int64
	ChatID         int64
	Labels         string
}

// NearbyUsers returns users to notify about an event near (lon, lat).
func (s *Store) NearbyUsers(ctx context.Context, lon, lat, radiusM float64, eventType string, cooldownMin int) ([]NearbyUser, error) {
	metrics.BumpQuery(ctx)
	rows, err := s.Pool.Query(ctx, `
		SELECT
		  ul.telegram_user_id,
		  ul.chat_id,
		  STRING_AGG(ul.label, ', ' ORDER BY ul.label) AS labels
		FROM user_locations ul
		WHERE ST_DWithin(
		  ul.geom,
		  ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
		  $3
		)
		AND NOT EXISTS (
		  SELECT 1 FROM notification_log nl
		  WHERE nl.telegram_user_id = ul.telegram_user_id
		    AND nl.event_type = $4
		    AND nl.notified_at > NOW() - ($5 || ' minutes')::INTERVAL
		    AND nl.notified_at > COALESCE(
		      (SELECT MAX(nl2.notified_at)
		       FROM notification_log nl2
		       WHERE nl2.telegram_user_id = ul.telegram_user_id
		         AND nl2.event_type <> $4),
		      '-infinity'::timestamptz
		    )
		)
		GROUP BY ul.telegram_user_id, ul.chat_id`,
		lon, lat, radiusM, eventType, cooldownMin,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NearbyUser
	for rows.Next() {
		var u NearbyUser
		if err := rows.Scan(&u.TelegramUserID, &u.ChatID, &u.Labels); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// LogNotification records a sent notification for cooldown tracking.
func (s *Store) LogNotification(ctx context.Context, userID int64, eventType string) error {
	metrics.BumpQuery(ctx)
	_, err := s.Pool.Exec(ctx,
		"INSERT INTO notification_log (telegram_user_id, event_type) VALUES ($1, $2)",
		userID, eventType,
	)
	return err
}
