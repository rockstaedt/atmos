package sqlite

const schema = `
CREATE TABLE IF NOT EXISTS rooms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    first_seen_at DATETIME NOT NULL,
    last_measurement_at DATETIME
);

CREATE TABLE IF NOT EXISTS measurements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    temperature REAL NOT NULL,
    humidity REAL NOT NULL,
    pressure REAL NOT NULL,
    co2 REAL,
    FOREIGN KEY (room_id) REFERENCES rooms(id)
);

-- Critical index for time-range queries
CREATE INDEX IF NOT EXISTS idx_measurements_room_timestamp
ON measurements(room_id, timestamp DESC);

-- Index for latest measurement queries
CREATE INDEX IF NOT EXISTS idx_measurements_timestamp
ON measurements(timestamp DESC);

-- Index for room lookups in measurements
CREATE INDEX IF NOT EXISTS idx_measurements_room
ON measurements(room_id);
`
