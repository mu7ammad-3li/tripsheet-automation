-- 001_create_trips.up.sql
-- Creates the core tables for trip sheet persistence.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE trips (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    odometer_open   INTEGER,
    odometer_close  INTEGER,
    total_miles     INTEGER,
    confidence_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    flagged_fields  TEXT[] NOT NULL DEFAULT '{}',
    status          VARCHAR(20) NOT NULL DEFAULT 'exception',
    validation_errors TEXT[] NOT NULL DEFAULT '{}',
    image_path      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE trip_line_items (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id     UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    date        VARCHAR(20),
    location    TEXT,
    miles       INTEGER,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_trip_line_items_trip_id ON trip_line_items(trip_id);
CREATE INDEX idx_trips_status ON trips(status);
CREATE INDEX idx_trips_created_at ON trips(created_at);
