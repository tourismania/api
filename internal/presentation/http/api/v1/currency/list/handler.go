// Package listcurrencieshttp — HTTP-граница запроса списка валют.
package listcurrencieshttp

import (
	"net/http"

	listcurrencies "api/internal/application/query/list_currencies"
	"api/internal/presentation/http/httpx"
)

// cacheControl — справочник меняется только с релизом api, поэтому клиент
// может кешировать ответ, как и поиск аэропортов.
const cacheControl = "private, max-age=3600"

// CurrencyResponse — одна валюта в ответе.
type CurrencyResponse struct {
	// Code — код ISO 4217.
	Code string `json:"code" example:"RUB"`
	// Exponent — число знаков после точки в сумме этой валюты.
	Exponent int `json:"exponent" example:"2"`
}

// ListCurrenciesResponse — тело ответа со всеми валютами справочника.
type ListCurrenciesResponse struct {
	Data []CurrencyResponse `json:"data"`
}

// Handler отдаёт список доступных валют.
type Handler struct {
	useCase listcurrencies.UseCase
}

// NewHandler создаёт обработчик.
func NewHandler(uc listcurrencies.UseCase) *Handler {
	return &Handler{useCase: uc}
}

// Handle — http.HandlerFunc.
//
//	@Summary      List currencies
//	@Description  Returns the system currency list: ISO 4217 code and number of decimal places, in directory order.
//	@Tags         Currencies
//	@Produce      json
//	@Success      200  {object}  ListCurrenciesResponse
//	@Failure      401  {object}  httpx.ErrorBody
//	@Failure      500  {object}  httpx.ErrorBody
//	@Security     Bearer
//	@Router       /api/v1/currencies [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	res, err := h.useCase.Handle(r.Context(), listcurrencies.Query{})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	data := make([]CurrencyResponse, 0, len(res.Currencies))
	for _, c := range res.Currencies {
		data = append(data, CurrencyResponse{Code: c.Code, Exponent: c.Exponent})
	}
	w.Header().Set("Cache-Control", cacheControl)
	httpx.WriteJSON(w, http.StatusOK, ListCurrenciesResponse{Data: data})
}
