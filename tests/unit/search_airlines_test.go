package unit_test

import (
	"context"
	"testing"

	"api/internal/application/apperror"
	searchairlines "api/internal/application/query/search_airlines"
	"api/internal/domain/airline"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingSearcher запоминает фильтр, с которым use case пошёл в репозиторий.
type recordingSearcher struct {
	got    airline.Filter
	called bool
}

func (r *recordingSearcher) Search(_ context.Context, f airline.Filter) (airline.SearchResult, error) {
	r.got, r.called = f, true
	return airline.SearchResult{}, nil
}

func searchQuery(search *string, country string, ids ...int) searchairlines.Query {
	return searchairlines.Query{Search: search, Country: country, IDs: ids, Limit: 20}
}

func strp(s string) *string { return &s }

func TestSearchAirlines_SearchNormalized(t *testing.T) {
	repo := &recordingSearcher{}

	res, err := searchairlines.NewHandler(repo).Handle(context.Background(), searchQuery(strp("  Turkish \t  Airlines "), ""))

	require.NoError(t, err)
	assert.Equal(t, "Turkish Airlines", repo.got.Search)
	assert.Equal(t, "Turkish Airlines", *res.Search, "в meta — нормализованная строка")
	assert.Nil(t, res.Country)
}

func TestSearchAirlines_ShortOrMissingSearch_ErrSearchTooShort(t *testing.T) {
	for name, q := range map[string]searchairlines.Query{
		"SU":                    searchQuery(strp("SU"), ""),
		"только пробелы":        searchQuery(strp("   "), ""),
		"пустая":                searchQuery(strp(""), ""),
		"без search и без ids":  searchQuery(nil, ""),
		"короткая вместе с ids": searchQuery(strp("SU"), "", 42),
	} {
		t.Run(name, func(t *testing.T) {
			repo := &recordingSearcher{}

			_, err := searchairlines.NewHandler(repo).Handle(context.Background(), q)

			assert.ErrorIs(t, err, searchairlines.ErrSearchTooShort)
			assert.False(t, repo.called, "в репозиторий не ходим")
		})
	}
}

func TestSearchAirlines_ThreeRunesAfterNormalization_OK(t *testing.T) {
	for _, s := range []string{"тур", " S U "} {
		_, err := searchairlines.NewHandler(&recordingSearcher{}).Handle(context.Background(), searchQuery(strp(s), ""))
		assert.NoError(t, err, "%q", s)
	}
}

func TestSearchAirlines_IDsWithoutSearch_NoNameFilter(t *testing.T) {
	repo := &recordingSearcher{}

	res, err := searchairlines.NewHandler(repo).Handle(context.Background(), searchQuery(nil, "", 42, 57))

	require.NoError(t, err)
	assert.Equal(t, "", repo.got.Search)
	assert.Equal(t, []int{42, 57}, repo.got.IDs)
	assert.Nil(t, res.Search, "meta.search — null")
}

func TestSearchAirlines_Country_UppercasedOrFieldError(t *testing.T) {
	repo := &recordingSearcher{}
	res, err := searchairlines.NewHandler(repo).Handle(context.Background(), searchQuery(strp("air"), "tr"))
	require.NoError(t, err)
	assert.Equal(t, "TR", repo.got.CountryISO2)
	assert.Equal(t, "TR", *res.Country)

	for _, raw := range []string{"TUR", "1", "T", "т1"} {
		repo := &recordingSearcher{}

		_, err := searchairlines.NewHandler(repo).Handle(context.Background(), searchQuery(strp("air"), raw))

		fe, ok := apperror.AsFieldError(err)
		require.True(t, ok, "%q: %v", raw, err)
		assert.Equal(t, "Country", fe.Field)
		assert.Equal(t, "iso2", fe.Rule)
		assert.ErrorIs(t, err, searchairlines.ErrCountryInvalid)
		assert.False(t, repo.called)
	}
}

func TestSearchAirlines_MinSearchLen(t *testing.T) {
	assert.Equal(t, 3, searchairlines.MinSearchLen)
}
