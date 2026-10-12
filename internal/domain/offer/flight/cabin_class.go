package flight

import "errors"

// CabinClass — класс обслуживания (класс салона) сегмента перелёта.
// Набор значений — продуктовый справочник: он живёт только здесь, в
// домене, presentation значения не перечисляет. Как добавить,
// переименовать или удалить класс — docs/cabin-classes.md.
//
// Тип лежит в пакете flight, а не в отдельном пакете-справочнике (как
// money): класс салона — атрибут сегмента перелёта и вне перелёта смысла не
// имеет; у других позиций оффера (круизы, поезда) свои классы, общего
// набора с авиаклассами у них нет. Тип рядом с сущностью, как offer.Status
// рядом с Offer.
type CabinClass string

// Классы обслуживания. Значения — строчные латинские буквы и "_" (без "+" и
// пробелов): они уходят в JSON, query и имена констант клиентов.
const (
	// CabinClassUnknown — класс неизвестен (не передан). Не бизнес-умолчание:
	// значение за клиента не подставляется.
	CabinClassUnknown     CabinClass = ""
	CabinClassEconomy     CabinClass = "economy"
	CabinClassComfort     CabinClass = "comfort"
	CabinClassComfortPlus CabinClass = "comfort_plus"
	CabinClassBusiness    CabinClass = "business"
)

// ErrCabinClassInvalid возвращается New, если класс сегмента не входит в
// CabinClasses и не пуст. Сравнение точное, без нормализации регистра и
// пробелов: значения машинные, а не текст.
var ErrCabinClassInvalid = errors.New("flight segment cabin class is not supported")

// CabinClasses возвращает известные классы обслуживания (без
// CabinClassUnknown) в порядке от эконома к бизнесу. Каждый вызов отдаёт
// новый срез — вызывающий может его менять.
func CabinClasses() []CabinClass {
	return []CabinClass{
		CabinClassEconomy,
		CabinClassComfort,
		CabinClassComfortPlus,
		CabinClassBusiness,
	}
}

// IsValid сообщает, что класс — один из CabinClasses или CabinClassUnknown.
func (c CabinClass) IsValid() bool {
	if c == CabinClassUnknown {
		return true
	}
	for _, known := range CabinClasses() {
		if c == known {
			return true
		}
	}
	return false
}
