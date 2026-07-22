package unit_test

import (
	"context"
	"testing"

	getoffer "api/internal/application/query/get_offer"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOfferFinder is a hand-written test double for the getoffer.OfferFinder port.
type stubOfferFinder struct {
	offer *entity.Offer
	err   error
}

func (s stubOfferFinder) FindByUUID(_ context.Context, _ uuid.UUID) (*entity.Offer, error) {
	return s.offer, s.err
}

// stubUserFinder is a hand-written test double for the
// application/identity.UserFinder port, shared by tests that resolve
// the acting principal from its uuid.
type stubUserFinder struct {
	record *entity.UserRecord
	err    error
}

func (s stubUserFinder) FindByUuid(_ context.Context, _ uuid.UUID) (*entity.UserRecord, error) {
	return s.record, s.err
}

func userRecordWithAgency(agencyID int) stubUserFinder {
	return stubUserFinder{record: &entity.UserRecord{ID: 1, AgencyID: agencyID}}
}

func TestGetOffer_MatchingAgency_SeesDraftOffer(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusDraft}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer}, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID: offer.UUID,
	})

	require.NoError(t, err)
	assert.Equal(t, offer.UUID, res.UUID)
}

func TestGetOffer_MatchingAgency_SeesPublishedOffer(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusPublished}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer}, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID: offer.UUID,
	})

	require.NoError(t, err)
	assert.Equal(t, offer.UUID, res.UUID)
}

func TestGetOffer_DifferentAgency_DraftOffer_NotFound(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusDraft}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer}, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: offer.UUID,
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestGetOffer_DifferentAgency_PublishedOffer_StillNotFound(t *testing.T) {
	// 1 user = 1 agency: even a published offer of another agency is
	// invisible on the private endpoint — get_published_offer serves
	// cross-agency published reads separately, with no identity at all.
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusPublished}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer}, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: offer.UUID,
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestGetOffer_NotFound_ReturnsErrOfferNotFound(t *testing.T) {
	h := getoffer.NewHandler(stubOfferFinder{offer: nil}, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: uuid.New(),
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestGetOffer_ActorNotFound_ReturnsErrActorNotFound(t *testing.T) {
	h := getoffer.NewHandler(stubOfferFinder{}, stubUserFinder{record: nil})

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: uuid.New(),
	})

	assert.ErrorIs(t, err, service.ErrActorNotFound)
}
