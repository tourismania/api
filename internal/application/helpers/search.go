package helpers

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// multiSpace — подряд идущие пробельные символы.
var multiSpace = regexp.MustCompile(`\s+`)

// NormalizeSearch приводит строку поиска справочника к виду, в котором её
// ищут и отдают в meta: пробелы по краям обрезаются, подряд идущие
// пробельные символы схлопываются в один пробел. ok — после нормализации в
// строке не меньше minLen символов (не байтов); порог у каждого справочника
// свой.
func NormalizeSearch(raw string, minLen int) (search string, ok bool) {
	search = multiSpace.ReplaceAllString(strings.TrimSpace(raw), " ")
	return search, utf8.RuneCountInString(search) >= minLen
}
