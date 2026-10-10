-- Перелёты, удалённые up-миграцией, не восстанавливаются.
ALTER TABLE offer_flights
    DROP COLUMN currency,
    DROP COLUMN price_minor;
