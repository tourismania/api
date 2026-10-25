package httpx

import (
	"errors"
	"strconv"
	"strings"
)

// ErrIDListInvalid — в списке id есть значение, не являющееся целым ≥ 1.
var ErrIDListInvalid = errors.New("ids must be positive integers")

// QueryInt разбирает целый параметр query-строки; пустое, нечисловое или
// отрицательное значение заменяется def.
func QueryInt(raw string, def int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return def
	}
	return v
}

// ParseIDList разбирает список id через запятую (`ids=42,57`): каждый
// элемент — целое ≥ 1, иначе ErrIDListInvalid (в т. ч. пустой элемент
// `42,,57`). Повторы убираются, порядок первого вхождения сохраняется.
// Ограничение числа id — дело валидатора вызывающего.
func ParseIDList(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, p := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || id < 1 {
			return nil, ErrIDListInvalid
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}
