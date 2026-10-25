CREATE TABLE airlines (
    id             serial       PRIMARY KEY,
    source_id      varchar(32),
    icao           char(3),
    name           varchar(200) NOT NULL,
    name_ru        varchar(200) NOT NULL DEFAULT '',
    country_iso2   char(2)      REFERENCES countries(iso2),
    dissolved_on   date,
    iata_codes     char(2)[]    NOT NULL DEFAULT '{}',
    merged_into_id int          REFERENCES airlines(id),
    missing_since  date
);

COMMENT ON TABLE  airlines                IS 'Справочник авиакомпаний; наполняется CLI airlines sync из Wikidata';
COMMENT ON COLUMN airlines.id             IS 'Постоянный id авиакомпании; на него ссылаются сегменты перелётов';
COMMENT ON COLUMN airlines.source_id      IS 'QID элемента Wikidata; NULL у служебной записи';
COMMENT ON COLUMN airlines.icao           IS 'Код ICAO; не уникален; NULL у служебной записи';
COMMENT ON COLUMN airlines.name           IS 'Английское название';
COMMENT ON COLUMN airlines.name_ru        IS 'Русское название, только у авиакомпаний России; пустая строка — нет';
COMMENT ON COLUMN airlines.country_iso2   IS 'Страна; NULL — не указана или неоднозначна';
COMMENT ON COLUMN airlines.dissolved_on   IS 'Дата прекращения деятельности; NULL — действует';
COMMENT ON COLUMN airlines.iata_codes     IS 'Коды IATA; не уникальны';
COMMENT ON COLUMN airlines.merged_into_id IS 'Дубль: источник слил элемент с элементом этой записи; скрыта из поиска';
COMMENT ON COLUMN airlines.missing_since  IS 'Дата пропажи из источника без слияния; скрыта из поиска';

-- unaccent() не IMMUTABLE, поэтому триграммные индексы — по lower(...), как у airports.
CREATE INDEX airlines_name_trgm_idx    ON airlines USING gin (lower(name) gin_trgm_ops);
CREATE INDEX airlines_name_ru_trgm_idx ON airlines USING gin (lower(name_ru) gin_trgm_ops);
CREATE INDEX airlines_country_idx      ON airlines (country_iso2);
CREATE INDEX airlines_source_id_idx    ON airlines (source_id);

-- Служебная запись «Неизвестная»: id 1 фиксирован (контракт api и DEFAULT миграции 022).
INSERT INTO airlines (id, name, name_ru) VALUES (1, 'Unknown', 'Неизвестная');
SELECT setval(pg_get_serial_sequence('airlines', 'id'), 1);
