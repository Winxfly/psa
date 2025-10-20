-- name: InsertScrapingDate :one
INSERT INTO scraping DEFAULT
VALUES RETURNING id;

-- name: GetLatestScraping :one
SELECT id, scraped_at
FROM scraping
ORDER BY scraped_at DESC LIMIT 1;

-- name: GetAllScrapingDates :many
SELECT id, scraped_at
FROM scraping
ORDER BY scraped_at DESC;