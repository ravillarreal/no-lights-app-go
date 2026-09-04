CREATE EXTENSION IF NOT EXISTS postgis;

-- ─── Ubicaciones guardadas por usuarios del bot de Telegram ──────────────────

CREATE TABLE IF NOT EXISTS user_locations (
  id               SERIAL PRIMARY KEY,
  telegram_user_id BIGINT       NOT NULL,
  chat_id          BIGINT       NOT NULL,
  first_name       TEXT,
  label            TEXT         NOT NULL DEFAULT 'Mi ubicación',
  geom             GEOGRAPHY(POINT, 4326) NOT NULL,
  created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_locations_geom
  ON user_locations USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_user_locations_user_id
  ON user_locations (telegram_user_id);

-- ─── Log de cooldown para notificaciones ─────────────────────────────────────

CREATE TABLE IF NOT EXISTS notification_log (
  id               SERIAL PRIMARY KEY,
  telegram_user_id BIGINT      NOT NULL,
  event_type       TEXT        NOT NULL,
  notified_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_log_lookup
  ON notification_log (telegram_user_id, event_type, notified_at);

-- ─── Eventos de corte de luz ─────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS outage_events (
  id             SERIAL PRIMARY KEY,
  usuario_id     TEXT                   NOT NULL,
  geom           GEOGRAPHY(POINT, 4326) NOT NULL,
  -- Reverse geocoding (se llena de forma asíncrona tras el reporte)
  neighborhood   TEXT,   -- barrio / urbanización
  municipality   TEXT,   -- municipio / ciudad
  state          TEXT,   -- estado / provincia
  country        TEXT,
  raw_address    JSONB,
  -- Timestamps
  started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ended_at       TIMESTAMPTZ,
  duration_min   INTEGER GENERATED ALWAYS AS (
    CASE WHEN ended_at IS NOT NULL
      THEN EXTRACT(EPOCH FROM (ended_at - started_at))::INTEGER / 60
      ELSE NULL
    END
  ) STORED
);

CREATE INDEX IF NOT EXISTS idx_outage_events_geom
  ON outage_events USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_outage_events_started
  ON outage_events (started_at DESC);
CREATE INDEX IF NOT EXISTS idx_outage_events_neighborhood
  ON outage_events (neighborhood, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_outage_events_municipality
  ON outage_events (municipality, started_at DESC);
-- Índice parcial para cortes activos (sin cierre)
CREATE INDEX IF NOT EXISTS idx_outage_events_open
  ON outage_events (usuario_id) WHERE ended_at IS NULL;

-- ─── Vista de resumen para dashboards ────────────────────────────────────────
-- Agrega cortes completados por zona y día. Metabase / Grafana la consumen directamente.

CREATE OR REPLACE VIEW v_outage_summary AS
SELECT
  COALESCE(neighborhood, '(sin barrio)')    AS neighborhood,
  COALESCE(municipality, '(sin municipio)') AS municipality,
  COALESCE(state,        '(sin estado)')    AS state,
  COALESCE(country,      '(sin país)')      AS country,
  DATE_TRUNC('day',  started_at) AS day,
  DATE_TRUNC('week', started_at) AS week,
  DATE_TRUNC('month',started_at) AS month,
  COUNT(*)                        AS total_outages,
  ROUND(AVG(duration_min)::NUMERIC, 1) AS avg_duration_min,
  MAX(duration_min)               AS max_duration_min,
  SUM(duration_min)               AS total_outage_min
FROM outage_events
WHERE ended_at IS NOT NULL
GROUP BY
  COALESCE(neighborhood, '(sin barrio)'),
  COALESCE(municipality, '(sin municipio)'),
  COALESCE(state,        '(sin estado)'),
  COALESCE(country,      '(sin país)'),
  DATE_TRUNC('day',  started_at),
  DATE_TRUNC('week', started_at),
  DATE_TRUNC('month',started_at);
