package unit_test

import (
	"context"
	"testing"

	"api/internal/application/apperror"
	getoffer "api/internal/application/query/get_offer"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubUserFinder is a hand-written test double for the domain
// repository.UserRepository port, shared by tests that resolve the
// acting principal from its uuid via service.NewUserFinder. Store is a
// no-op: these tests only ever exercise the read path.
type stubUserFinder struct {
	record *entity.UserRecord
	err    error
}

func (s stubUserFinder) FindByUuid(_ context.Context, _ uuid.UUID) (*entity.UserRecord, error) {
	return s.record, s.err
}

func (s stubUserFinder) Store(_ context.Context, _ entity.User, _ string) (*int, error) {
	return nil, nil
}

func userRecordWithAgency(agencyID int) *service.UserFinder {
	return service.NewUserFinder(stubUserFinder{record: &entity.UserRecord{ID: 1, AgencyID: agencyID}})
}

func noUserFound() *service.UserFinder {
	return service.NewUserFinder(stubUserFinder{record: nil})
}

func TestGetOffer_MatchingAgency_SeesDraftOffer(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusDraft}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: offer}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID: offer.UUID,
	})

	require.NoError(t, err)
	assert.Equal(t, offer.UUID, res.UUID)
}

func TestGetOffer_MatchingAgency_SeesPublishedOffer(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusPublished}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: offer}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID: offer.UUID,
	})

	require.NoError(t, err)
	assert.Equal(t, offer.UUID, res.UUID)
}

func TestGetOffer_DifferentAgency_DraftOffer_NotFound(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusDraft}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: offer}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: offer.UUID,
	})

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestGetOffer_DifferentAgency_PublishedOffer_StillNotFound(t *testing.T) {
	// 1 user = 1 agency: even a published offer of another agency is
	// invisible on the private endpoint — get_published_offer serves
	// cross-agency published reads separately, with no identity at all.
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusPublished}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: offer}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: offer.UUID,
	})

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestGetOffer_NotFound_ReturnsErrNotFound(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: nil}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestGetOffer_ActorNotFound_ReturnsErrUnauthenticated(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, noUserFound())

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)
}
