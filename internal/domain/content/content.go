// Package content — общие правила содержательных полей записей: текст без
// управляющих символов, ссылки на фото и звёзды отеля. Ими пользуются
// справочник отелей (domain/hotel) и позиции оффера, чтобы одинаковые поля
// проверялись одинаково. Пакет ни от кого не зависит.
package content

import (
	"errors"
	"net/url"
)

// Границы звёзд отеля: 0 — без звёзд.
const (
	MinStars = 0
	MaxStars = 5
)

// Границы запрещённых в тексте управляющих символов.
const (
	lastC0Control = '\u001f'
	deleteControl = '\u007f'
)

// Схемы, допустимые в ссылке на фото.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

var (
	// ErrTextControlChar — в тексте есть управляющий символ C0
	// (U+0000–U+001F), кроме табуляции, перевода строки и возврата каретки,
	// или U+007F. U+0000 PostgreSQL не хранит в TEXT, остальные в тексте для
	// клиента бессмысленны и ломают вывод.
	ErrTextControlChar = errors.New("text contains a forbidden control character")
	// ErrPhotoURLInvalid — ссылка на фото не абсолютный URL со схемой http
	// или https и непустым хостом.
	ErrPhotoURLInvalid = errors.New("photo url must be an absolute http(s) url with a host")
	// ErrStarsInvalid — звёзды вне диапазона MinStars–MaxStars.
	ErrStarsInvalid = errors.New("stars must be between 0 and 5")
)

// CheckText проверяет, что в s нет запрещённых управляющих символов
// (ErrTextControlChar). Табуляция, перевод строки и возврат каретки
// допустимы. Длину и пробелы по краям CheckText не проверяет — это правила
// конкретного поля.
func CheckText(s string) error {
	for _, r := range s {
		if (r <= lastC0Control && r != '\t' && r != '\n' && r != '\r') || r == deleteControl {
			return ErrTextControlChar
		}
	}
	return nil
}

// CheckPhotoURL проверяет, что s — абсолютный URL со схемой http или https
// (без учёта регистра) и непустым хостом (ErrPhotoURLInvalid). Доступность
// ресурса не проверяется, файл не загружается.
func CheckPhotoURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return ErrPhotoURLInvalid
	}
	// url.Parse приводит схему к нижнему регистру.
	if u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS {
		return ErrPhotoURLInvalid
	}
	return nil
}

// CheckStars проверяет, что звёзды в диапазоне MinStars–MaxStars
// (ErrStarsInvalid).
func CheckStars(stars int) error {
	if stars < MinStars || stars > MaxStars {
		return ErrStarsInvalid
	}
	return nil
}
