CREATE TABLE hotels (
    id          serial      PRIMARY KEY,
    agency_id   int         NOT NULL REFERENCES agencies(id),
    name        text        NOT NULL,
    stars       smallint    NOT NULL CHECK (stars BETWEEN 0 AND 5),
    address     text        NOT NULL DEFAULT '',
    description text        NOT NULL DEFAULT '',
    photo_urls  text[]      NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz NULL
);

COMMENT ON TABLE  hotels             IS 'Справочник отелей агентства; записи появляются при сохранении оффера';
COMMENT ON COLUMN hotels.id          IS 'Постоянный id отеля; на него ссылаются позиции офферов';
COMMENT ON COLUMN hotels.agency_id   IS 'Агентство, которому принадлежит отель и которое одно его видит';
COMMENT ON COLUMN hotels.name        IS 'Название без пробелов по краям, до 200 символов; не уникально';
COMMENT ON COLUMN hotels.stars       IS 'Звёзды 0–5; 0 — без звёзд';
COMMENT ON COLUMN hotels.address     IS 'Адрес свободным текстом, до 500 символов; пустая строка — не указан';
COMMENT ON COLUMN hotels.description IS 'Описание, до 5000 символов; пустая строка — не указано';
COMMENT ON COLUMN hotels.photo_urls  IS 'Ссылки http(s) на фото во внешних ресурсах, до 20, в порядке показа';
COMMENT ON COLUMN hotels.created_at  IS 'Момент создания записи';
COMMENT ON COLUMN hotels.deleted_at  IS 'Момент мягкого удаления; NULL — запись действует';

-- Поиск идёт по агентству и названию без учёта регистра, удалённые не ищутся.
CREATE INDEX hotels_agency_name_idx ON hotels (agency_id, lower(name)) WHERE deleted_at IS NULL;
