# Issue №24 — Заменить Docker-образ Kafka: bitnami/kafka → apache/kafka

- **GitHub Issue:** №24
- **Дата:** 2026-09-16
- **Статус:** реализовано (ветка `bugfix/24`)

## Контекст

`docker-compose up kafka` перестал работать: Bitnami свернул публичные теги на Docker Hub, образ `bitnami/kafka:3.7` больше не резолвится:

```
Error response from daemon: failed to resolve reference "docker.io/bitnami/kafka:3.7": docker.io/bitnami/kafka:3.7: not found
```

## Решение

Сервис `kafka` переведён на официальный образ **`apache/kafka:3.7.2`** (та же версия Kafka 3.7, KRaft single-node, конфигурация по официальному примеру apache/kafka `docker/examples/docker-compose-files/single-node/plaintext`):

- Переменные `KAFKA_CFG_*` (bitnami-специфика) → `KAFKA_*` (официальный образ транслирует их в `server.properties`); `ALLOW_PLAINTEXT_LISTENER` удалена (bitnami-специфика).
- Топология не менялась: брокер по-прежнему доступен внутри compose-сети как `kafka:9092` (advertised listener тот же) — конфигурация приложения не тронута.
- Добавлен фиксированный `CLUSTER_ID`: с персистентным volume случайный id при каждом старте конфликтовал бы с уже отформатированным хранилищем.
- Для single-node явно снижены replication-factor'ы системных топиков до 1 (`KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR` и др.) — у bitnami это были дефолты, у официального образа дефолты рассчитаны на кластер.
- Данные персистятся в новом volume `kafka_data` (смонтирован в `/var/lib/kafka/data` через `KAFKA_LOG_DIRS`). Старый volume `kafka` c bitnami-раскладкой (`/bitnami/kafka`) несовместим и не переиспользуется — его можно удалить: `docker volume rm <project>_kafka`.

## Проверено

- `docker-compose config` — валиден.
- `docker-compose up -d kafka` — брокер стартует без ошибок (`Kafka Server started`).
- Создание/листинг/удаление топика через `kafka-topics.sh --bootstrap-server kafka:9092` — работает.
- `meta.properties` и данные лежат в volume; рестарт контейнера проходит чисто (0 ERROR в логах).

## Negative constraints (соблюдены)

- Код приложения и его переменные окружения не менялись — правка только в `docker-compose.yml`.
- Версия Kafka не поднималась (3.7 → 3.7.2, тот же minor; апгрейд мажора/минора — отдельной задачей при необходимости).
