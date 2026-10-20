-- Справочник авиакомпаний (travel-services/airlines). Наполняется только
-- командой `airlines sync` из Wikidata, через HTTP не редактируется.
--
-- Ключ — собственный id записи, а не код ICAO: код ICAO переназначается после
-- закрытия авиакомпании (~13% кодов в Wikidata встречаются у нескольких
-- авиакомпаний, включая закрывшиеся), у действующих бывают общие коды (~4%), а
-- источник отмечает закрытие с задержкой. Ссылка сегмента перелёта по коду
-- требовала бы при каждой записи заново решать, какую авиакомпанию код
-- означает; ссылка по id ставится один раз при выборе в поиске и дальше не
-- вычисляется. Поэтому icao и iata_codes — атрибуты без уникальности.
--
-- source_id — QID элемента Wikidata, по нему синхронизация обновляет запись.
-- Не UNIQUE: идентификатор чужой системы — не наш инвариант (при смене
-- источника его идентификаторы могут совпасть с уже записанными QID).
-- «Одна запись на source_id» проверяет синхронизация до записи.
--
-- merged_into_id и missing_since — факты о самой записи в источнике, а не
-- выбор «главной» среди авиакомпаний с одним кодом (отвергнутый is_current):
--   - merged_into_id — запись оказалась дублем: источник слил её элемент с
--     элементом записи merged_into_id;
--   - missing_since — с этой даты элемента нет в источнике (без слияния).
-- Заполненное поле скрывает запись из поиска; сегменты перелётов, которые уже
-- на неё ссылаются, не меняются. Дата, а не boolean, — видно, как давно
-- запись пропала.
--
-- name_ru заполняется только у авиакомпаний России (иностранные клиенту
-- показываются по-английски); '' вместо NULL — как manager_comment и
-- cabin_class перелёта: одна пустота проще клиентам и коду.
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

-- Триграммные индексы для поиска подстрокой по названиям. unaccent() не
-- IMMUTABLE и в выражение индекса не допускается, поэтому индекс — по
-- lower(...), как у airports (миграция 010).
CREATE INDEX airlines_name_trgm_idx    ON airlines USING gin (lower(name) gin_trgm_ops);
CREATE INDEX airlines_name_ru_trgm_idx ON airlines USING gin (lower(name_ru) gin_trgm_ops);
CREATE INDEX airlines_country_idx      ON airlines (country_iso2);
CREATE INDEX airlines_source_id_idx    ON airlines (source_id);

-- Служебная запись «Неизвестная» — для сегмента, перевозчик которого ещё
-- неизвестен. id = 1 фиксирован: это контракт для клиентов и DEFAULT в
-- миграции 022. Не 0: 0 — нулевое значение int в Go и JSON-клиентах, и
-- забытое поле молча означало бы «Неизвестная»; так оно ловится проверкой
-- обязательности. Без source_id — синхронизация её не создаёт, не меняет и
-- не скрывает.
INSERT INTO airlines (id, name, name_ru) VALUES (1, 'Unknown', 'Неизвестная');

-- Последовательность сдвигается за служебную запись: следующая вставка без id
-- получит 2.
SELECT setval(pg_get_serial_sequence('airlines', 'id'), 1);
