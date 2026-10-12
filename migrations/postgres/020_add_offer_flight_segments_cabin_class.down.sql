-- Сохранённые классы сегментов теряются.
ALTER TABLE offer_flight_segments
    DROP COLUMN cabin_class;
