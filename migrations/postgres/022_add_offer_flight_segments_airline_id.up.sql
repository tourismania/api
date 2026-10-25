-- DEFAULT 1 («Неизвестная») — заполнение уже сохранённых сегментов и совместимость при выкладке,
-- не умолчание контракта: api требует airline_id в каждом сегменте.
ALTER TABLE offer_flight_segments
    ADD COLUMN airline_id int NOT NULL DEFAULT 1 REFERENCES airlines(id);

COMMENT ON COLUMN offer_flight_segments.airline_id IS 'Авиакомпания сегмента — id записи справочника airlines; 1 — «Неизвестная»';
