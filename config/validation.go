package config

import (
	"api/internal/domain/money"
	createofferhttp "api/internal/presentation/http/api/v1/offer/create"
	updateofferhttp "api/internal/presentation/http/api/v1/offer/update"
	"api/internal/presentation/http/httpx"

	"github.com/go-playground/validator/v10"
)

// RegisterValidations регистрирует на валидаторе проверки, которым нужна
// доменная логика, — сейчас цену перелёта по системному справочнику
// валют. Вызывается из NewContainer и из HTTP-тестов, чтобы тесты
// проверяли те же правила, что и сервер.
func RegisterValidations(v *validator.Validate) {
	v.RegisterStructValidation(
		httpx.PriceValidation(parseMoney),
		createofferhttp.FlightInput{}, updateofferhttp.FlightInput{},
	)
}

// parseMoney — httpx.PriceParser поверх справочника валют money: только
// проверяет сумму, сам результат разбора нужен лишь application-слою.
func parseMoney(amount, currency string) error {
	_, err := money.Parse(amount, currency)
	return err
}
