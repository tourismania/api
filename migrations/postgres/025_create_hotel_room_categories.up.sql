CREATE TABLE hotel_room_categories (
    id          serial      PRIMARY KEY,
    hotel_id    int         NOT NULL REFERENCES hotels(id),
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    photo_urls  text[]      NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz NULL
);

COMMENT ON TABLE  hotel_room_categories             IS 'Категории номеров отелей справочника агентства';
COMMENT ON COLUMN hotel_room_categories.id          IS 'Постоянный id категории; порядок id — порядок создания';
COMMENT ON COLUMN hotel_room_categories.hotel_id    IS 'Отель, которому принадлежит категория';
COMMENT ON COLUMN hotel_room_categories.name        IS 'Название без пробелов по краям, до 200 символов; уникально без учёта регистра среди действующих категорий отеля';
COMMENT ON COLUMN hotel_room_categories.description IS 'Описание, до 2000 символов; пустая строка — не указано';
COMMENT ON COLUMN hotel_room_categories.photo_urls  IS 'Ссылки http(s) на фото во внешних ресурсах, до 20, в порядке показа';
COMMENT ON COLUMN hotel_room_categories.created_at  IS 'Момент создания записи';
COMMENT ON COLUMN hotel_room_categories.deleted_at  IS 'Момент мягкого удаления; NULL — запись действует';

-- Удалённые категории не занимают название: уникальность только среди действующих.
-- Индекс же служит выборке категорий отеля (hotel_id — первый столбец).
CREATE UNIQUE INDEX hotel_room_categories_hotel_name_uniq ON hotel_room_categories (hotel_id, lower(name)) WHERE deleted_at IS NULL;
