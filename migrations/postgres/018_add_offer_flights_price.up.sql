-- Цена перелёта обязательна, а у уже сохранённых перелётов её нет и взять
-- неоткуда: web перелёты в api не отправлял, они могли появиться только при
-- прямых вызовах API. Заполнять нулём нельзя — клиент увидел бы «бесплатный
-- перелёт», поэтому такие перелёты удаляются (сегменты — каскадом).
ALTER TABLE offer_flights
    ADD COLUMN price_minor BIGINT  NULL,
    ADD COLUMN currency    CHAR(3) NULL;

DELETE FROM offer_flights;

-- CHECK на сумму и список валют нет намеренно: справочник валют живёт в коде
-- (internal/domain/money), и его дубль в БД требовал бы миграции на каждую
-- новую валюту.
ALTER TABLE offer_flights
    ALTER COLUMN price_minor SET NOT NULL,
    ALTER COLUMN currency    SET NOT NULL;
