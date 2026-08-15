package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
)

// ErrBadJSON wraps any JSON decoding failure. Проверяйте её через
// errors.Is и передавайте исходную ошибку в WriteDecodeError — она
// восстанавливает из неё конкретную причину (синтаксис, тип поля,
// формат времени) вместо общего "invalid JSON body".
var ErrBadJSON = errors.New("invalid JSON body")

// DecodeJSON decodes the request body into v and validates it (if a
// validator is supplied). The caller decides how to surface the error.
func DecodeJSON(r *http.Request, v any, validate *validator.Validate) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return ErrBadJSON
		}
		return errors.Join(ErrBadJSON, err)
	}
	if validate != nil {
		if err := validate.Struct(v); err != nil {
			return err
		}
	}
	return nil
}

// WriteDecodeError разбирает ошибку, обёрнутую DecodeJSON в ErrBadJSON, и
// отдаёт максимально конкретный 400 вместо общего "invalid JSON body":
// синтаксическая ошибка JSON — со смещением в байтах; несовпадение типа
// поля — с именем поля, ожидаемым и полученным типом; невалидный формат
// времени (time.Time.UnmarshalJSON, например для departure_at/arrival_at)
// — с подсказкой про RFC3339. Тело запроса, доступное клиенту напрямую,
// никогда не логируется на этой границе — только структура самой ошибки
// decoder'а, поэтому чувствительные значения (пароли и т.п.) в ответ не
// попадают, кроме случая json.UnmarshalTypeError, где попадает значение
// самого несовпавшего поля (валидный trade-off: это ошибка формата, а не
// секрет).
func WriteDecodeError(w http.ResponseWriter, err error) {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf(
			"malformed JSON at byte offset %d: %s", syntaxErr.Offset, syntaxErr.Error(),
		))
		return
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		meta := map[string]any{}
		if typeErr.Field != "" {
			meta["field"] = typeErr.Field
		}
		WriteJSON(w, http.StatusBadRequest, ErrorBody{
			Error: fmt.Sprintf("field %q has the wrong type: expected %s, got %s", typeErr.Field, typeErr.Type, typeErr.Value),
			Code:  http.StatusBadRequest,
			Meta:  meta,
		})
		return
	}

	var timeErr *time.ParseError
	if errors.As(err, &timeErr) {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf(
			"invalid date/time value: %s (expected RFC3339, e.g. \"2026-08-30T12:00:00Z\")", timeErr.Error(),
		))
		return
	}

	// Ни один из известных типов не подошёл (например, unknown field от
	// DisallowUnknownFields или другая пользовательская UnmarshalJSON) —
	// показываем исходное сообщение decoder'а вместо ErrBadJSON целиком.
	if cause := unwrapDecodeCause(err); cause != nil {
		WriteError(w, http.StatusBadRequest, cause.Error())
		return
	}

	WriteError(w, http.StatusBadRequest, "invalid JSON body")
}

// unwrapDecodeCause достаёт из ошибки, склеенной DecodeJSON через
// errors.Join(ErrBadJSON, err), исходную err — саму ErrBadJSON без
// причины (пустое тело запроса) возвращает nil.
func unwrapDecodeCause(err error) error {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return nil
	}
	for _, e := range joined.Unwrap() {
		if !errors.Is(e, ErrBadJSON) {
			return e
		}
	}
	return nil
}
