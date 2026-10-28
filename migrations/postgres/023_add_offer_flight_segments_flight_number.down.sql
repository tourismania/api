-- Сохранённые номера рейсов сегментов теряются.
ALTER TABLE offer_flight_segments
    DROP COLUMN flight_number;
