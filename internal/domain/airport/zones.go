package airport

import "time"

// Zones — часовые пояса аэропортов: код ICAO → *time.Location. По ним местное
// время аэропорта превращается в реальный момент. Нет записи — время аэропорта
// считается UTC; нулевое значение (nil) допустимо и означает UTC для всех
// аэропортов.
type Zones map[string]*time.Location

// NewZones строит Zones по IANA-таймзонам городов аэропортов. Пустая или
// неизвестная таймзона — не ошибка: запись не добавляется, и время такого
// аэропорта считается UTC, чтобы неполнота справочника не закрывала аэропорт
// для перелётов.
func NewZones(airports []Airport) Zones {
	zones := make(Zones, len(airports))
	for _, a := range airports {
		if a.City.Timezone == "" {
			continue
		}
		loc, err := time.LoadLocation(a.City.Timezone)
		if err != nil {
			continue
		}
		zones[a.ICAO] = loc
	}
	return zones
}

// Moment возвращает реальный момент для настенных часов wall в аэропорту icao.
// Смещение определяется на эту дату с учётом перевода часов; для неоднозначного
// и несуществующего местного времени вариант выбирает time.Date.
func (z Zones) Moment(icao string, wall time.Time) time.Time {
	loc, ok := z[icao]
	if !ok {
		loc = time.UTC
	}
	year, month, day := wall.Date()
	hour, minute, second := wall.Clock()
	return time.Date(year, month, day, hour, minute, second, 0, loc)
}
