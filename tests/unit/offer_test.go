package unit_test

import (
	"testing"

	"api/internal/domain/entity"
	"api/internal/domain/enum"

	"github.com/stretchr/testify/assert"
)

func TestOffer_IsPublished_TrueForPublishedStatus(t *testing.T) {
	offer := entity.Offer{Status: enum.OfferStatusPublished}
	assert.True(t, offer.IsPublished())
}

func TestOffer_IsPublished_FalseForDraftStatus(t *testing.T) {
	offer := entity.Offer{Status: enum.OfferStatusDraft}
	assert.False(t, offer.IsPublished())
}

func TestOffer_IsPublished_FalseForReadyStatus(t *testing.T) {
	offer := entity.Offer{Status: enum.OfferStatusReady}
	assert.False(t, offer.IsPublished(), "ready is saved but not yet published — same visibility as draft")
}

func TestOfferStatus_IsValid_KnownValues(t *testing.T) {
	assert.True(t, enum.OfferStatusDraft.IsValid())
	assert.True(t, enum.OfferStatusReady.IsValid())
	assert.True(t, enum.OfferStatusPublished.IsValid())
}

func TestOfferStatus_IsValid_UnknownValue_ReturnsFalse(t *testing.T) {
	assert.False(t, enum.OfferStatus("archived").IsValid())
	assert.False(t, enum.OfferStatus("").IsValid())
}
