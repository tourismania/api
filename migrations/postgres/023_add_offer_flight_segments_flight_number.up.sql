-- DEFAULT '' («номер неизвестен») — заполнение уже сохранённых сегментов и совместимость при
-- выкладке. VARCHAR(10) — лимит контракта (validate:"max=10" у номера в DTO сегмента, символы, а
-- не байты): менять вместе с тегом DTO. CHECK на формат нет намеренно — номер хранится как введён.
ALTER TABLE offer_flight_segments
    ADD COLUMN flight_number VARCHAR(10) NOT NULL DEFAULT '';

COMMENT ON COLUMN offer_flight_segments.flight_number IS 'Номер рейса сегмента в том виде, как введён (без формата и нормализации); пустая строка — номер неизвестен';
