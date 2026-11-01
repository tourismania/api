// Package offerhttp — общие формы запроса и ответа хендлеров ресурса offer:
// create, update, get и get_public.
package offerhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// localDateTimeLayout — формат местного времени сегмента: без смещения и
// суффикса Z.
const localDateTimeLayout = "2006-01-02T15:04:05"

// jsonNull — литерал null в теле запроса.
const jsonNull = "null"

// LocalDateTime — местные дата и время аэропорта в формате
// YYYY-MM-DDTHH:MM:SS. Значение со смещением, с Z или в другом формате не
// разбирается, поэтому смещение не может быть молча отброшено. Внутри —
// настенные часы в time.UTC.
type LocalDateTime struct {
	wall time.Time
}

// NewLocalDateTime оборачивает настенные часы wall.
func NewLocalDateTime(wall time.Time) LocalDateTime {
	return LocalDateTime{wall: wall}
}

// Time возвращает настенные часы в time.UTC.
func (d LocalDateTime) Time() time.Time {
	return d.wall
}

// UnmarshalJSON разбирает строку в формате localDateTimeLayout. Литерал null,
// как у time.Time, не меняет значение: его отклонит validate:"required".
// Исходная ошибка разбора не оборачивается: иначе httpx.WriteDecodeError
// подсказал бы формат RFC3339, которого здесь нет.
func (d *LocalDateTime) UnmarshalJSON(data []byte) error {
	if string(data) == jsonNull {
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return invalidLocalDateTime(data)
	}
	parsed, err := time.Parse(localDateTimeLayout, raw)
	// time.Parse молча принимает дробные секунды, которых нет в формате, поэтому
	// значение ещё сверяется с собственной записью.
	if err != nil || parsed.Format(localDateTimeLayout) != raw {
		return invalidLocalDateTime(data)
	}
	d.wall = parsed
	return nil
}

// MarshalJSON записывает настенные часы в формате localDateTimeLayout.
func (d LocalDateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.wall.Format(localDateTimeLayout))
}

// invalidLocalDateTime — ошибка разбора значения data.
func invalidLocalDateTime(data []byte) error {
	return fmt.Errorf(
		"invalid date/time value: %s (expected local YYYY-MM-DDTHH:MM:SS without timezone)",
		bytes.TrimSpace(data),
	)
}
