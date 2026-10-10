package listcurrencies

import (
	"context"

	"api/internal/domain/money"
)

// UseCase — порт, от которого зависит presentation-слой.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// Handler читает справочник валют. Справочник живёт в коде, поэтому
// зависимостей у обработчика нет.
type Handler struct{}

// NewHandler создаёт обработчик.
func NewHandler() *Handler {
	return &Handler{}
}

// Handle реализует UseCase.
func (h *Handler) Handle(_ context.Context, _ Query) (Result, error) {
	currencies := money.Currencies()
	out := make([]CurrencyResult, 0, len(currencies))
	for _, c := range currencies {
		out = append(out, CurrencyResult{Code: c.Code, Exponent: c.Exponent})
	}
	return Result{Currencies: out}, nil
}
