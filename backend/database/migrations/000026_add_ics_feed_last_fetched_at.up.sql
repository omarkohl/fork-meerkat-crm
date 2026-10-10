-- When a calendar app last fetched the feed (updated at most once a minute).
ALTER TABLE ics_feeds ADD COLUMN last_fetched_at DATETIME;
