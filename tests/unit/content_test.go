package unit_test

import (
	"testing"

	"api/internal/domain/content"

	"github.com/stretchr/testify/assert"
)

func TestCheckText_ControlCharacters(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		ok   bool
	}{
		"обычный текст":           {text: "Отель у моря", ok: true},
		"пустая строка":           {text: "", ok: true},
		"табуляция":               {text: "a\tb", ok: true},
		"перевод строки":          {text: "a\nb", ok: true},
		"возврат каретки":         {text: "a\r\nb", ok: true},
		"NUL":                     {text: "a\u0000b", ok: false},
		"ESC":                     {text: "a\u001bb", ok: false},
		"DEL":                     {text: "a\u007fb", ok: false},
		"управляющий в конце":     {text: "abc\u0001", ok: false},
		"символы вне ASCII верны": {text: "Ünïcödé — ★", ok: true},
	} {
		t.Run(name, func(t *testing.T) {
			err := content.CheckText(tc.text)
			if tc.ok {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, content.ErrTextControlChar)
			}
		})
	}
}

func TestCheckPhotoURL_AbsoluteHTTPWithHost(t *testing.T) {
	for name, tc := range map[string]struct {
		url string
		ok  bool
	}{
		"https":                {url: "https://cdn.example.com/a.jpg", ok: true},
		"http":                 {url: "http://cdn.example.com/a.jpg?w=800", ok: true},
		"схема заглавными":     {url: "HTTPS://cdn.example.com/a.jpg", ok: true},
		"относительная":        {url: "cdn/a.jpg", ok: false},
		"ftp":                  {url: "ftp://x/a.jpg", ok: false},
		"javascript":           {url: "javascript:alert(1)", ok: false},
		"пустая":               {url: "", ok: false},
		"https без хоста":      {url: "https://", ok: false},
		"https без хоста путь": {url: "https:///a.jpg", ok: false},
		"управляющий символ":   {url: "https://cdn.example.com/a\u0000.jpg", ok: false},
	} {
		t.Run(name, func(t *testing.T) {
			err := content.CheckPhotoURL(tc.url)
			if tc.ok {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, content.ErrPhotoURLInvalid)
			}
		})
	}
}

func TestCheckStars_Range(t *testing.T) {
	for _, stars := range []int{0, 3, 5} {
		assert.NoError(t, content.CheckStars(stars), stars)
	}
	for _, stars := range []int{-1, 6} {
		assert.ErrorIs(t, content.CheckStars(stars), content.ErrStarsInvalid, stars)
	}
}
