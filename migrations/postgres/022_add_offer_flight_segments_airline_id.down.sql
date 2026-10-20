-- Сохранённые авиакомпании сегментов теряются.
ALTER TABLE offer_flight_segments
    DROP COLUMN airline_id;
