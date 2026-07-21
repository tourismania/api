# [chore] Явное ON DELETE/ON UPDATE поведение для FK users.agency_id → agencies(id)

> **GitHub issue:** [#17](https://github.com/tourismania/api/issues/17)

## Контекст

Колонка `users.agency_id` ссылается на `agencies(id)` (миграция `014_add_users_agency`), но FK объявлен без явного `ON DELETE`/`ON UPDATE`:

```sql
ALTER TABLE "users"
    ADD COLUMN agency_id INT NULL REFERENCES agencies(id);
```

По умолчанию Postgres использует `NO ACTION` — попытка удалить агентство, на которую ссылается хотя бы один пользователь, завершится ошибкой на уровне БД. Это фактически совпадает с поведением `RESTRICT`, но неявно и не задокументировано в схеме: не видно из DDL, что поведение осознанно выбрано, а не забыто.

Агентства и так удаляются только мягко (`agencies.deleted_at`, `AgencyManager.Deactivate`/`Activate` меняют `status`, а не удаляют строку) — HTTP/CLI-путей для `DELETE FROM agencies` в проекте нет. Тем не менее FK — это гарантия на уровне БД, а не только уровня приложения, поэтому поведение должно быть зафиксировано явно.

**Только `users.agency_id`.** FK `offers.agency_id → agencies(id)` (миграция `015_create_offers`) в этот issue не входит — обсуждается отдельно при необходимости.

## Scope

- Новая миграция `016_set_users_agency_fk_action` (1 действие = 1 миграция: явное переопределение поведения одного constraint):
  - `up`: `ALTER TABLE users DROP CONSTRAINT <имя_constraint>;` затем `ALTER TABLE users ADD CONSTRAINT <имя_constraint> FOREIGN KEY (agency_id) REFERENCES agencies(id) ON DELETE RESTRICT;`
    - Точное имя constraint уточнить через `\d users` / `information_schema.table_constraints` перед миграцией (авто-имя вида `users_agency_id_fkey`, если не переопределялось явно).
    - `ON UPDATE` не задаётся — `agencies.id` это `SERIAL`, значение первичного ключа не меняется, `ON UPDATE CASCADE` не имеет практического смысла.
  - `down`: откат к исходному безымянному поведению (`DROP CONSTRAINT` + `ADD ... REFERENCES agencies(id)` без `ON DELETE`).
- Обновить комментарий/схему в `internal/infrastructure/persistence/postgres/model/user.go` при необходимости (уточнить, что удаление агентства заблокировано на уровне БД, пока есть привязанные пользователи).
- Тесты: integration-тест, подтверждающий, что попытка `DELETE FROM agencies WHERE id = ...` с существующим пользователем возвращает ошибку FK-constraint.

## Acceptance criteria

- [ ] Миграция `016_set_users_agency_fk_action` применяется и откатывается (`make migrate-up` / `make migrate-down`).
- [ ] `\d users` показывает `ON DELETE RESTRICT` для FK на `agencies(id)`.
- [ ] Integration-тест: удаление агентства с привязанным пользователем завершается ошибкой БД (constraint violation), а не проходит молча.
- [ ] `go test ./...`, `go build ./...` — зелёные.
- [ ] `README.md` не требует изменений (нет новых CLI/эндпоинтов).

## Negative constraints (чего НЕ делаем)

- Не трогаем FK `offers.agency_id → agencies(id)` — вне scope.
- Не добавляем HTTP/CLI-эндпоинт для физического удаления агентства — его как не было, так и нет.
- Не меняем nullability `users.agency_id` (остаётся `NOT NULL`, как зафиксировано в issue «Сущность Agency», [#11](https://github.com/tourismania/api/issues/11)).
- Не используем `ON DELETE CASCADE`/`SET NULL` — агентство с активными пользователями не должно исчезать бесследно ни для пользователей, ни для их данных.
