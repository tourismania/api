package httpx

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrIDListInvalid — в списке id есть значение, не являющееся целым ≥ 1.
	ErrIDListInvalid = errors.New("ids must be positive integers")
	// ErrISO2Invalid — значение не код страны ISO 3166-1 alpha-2 (две латинские буквы).
	ErrISO2Invalid = errors.New("country must be an ISO 3166-1 alpha-2 code")

	// iso2Pattern — код страны ISO 3166-1 alpha-2 в любом регистре.
	iso2Pattern = regexp.MustCompile(`^[A-Za-z]{2}$`)
)

// ParseISO2 проверяет формат кода страны (`country=tr`) и приводит его к
// верхнему регистру; не две латинские буквы — ErrISO2Invalid. Есть ли такая
// страна в справочнике, не проверяет: неизвестный код просто ничего не найдёт.
func ParseISO2(raw string) (string, error) {
	if !iso2Pattern.MatchString(raw) {
		return "", ErrISO2Invalid
	}
	return strings.ToUpper(raw), nil
}

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
