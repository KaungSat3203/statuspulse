CREATE UNIQUE INDEX IF NOT EXISTS idx_incidents_one_open_per_monitor
    ON incidents (monitor_id)
    WHERE recovered_at IS NULL;
