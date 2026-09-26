package unit_test

import (
	"api/internal/domain/offer"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOffer_IsPublished_TrueForPublishedStatus(t *testing.T) {
	o := offer.Offer{Status: offer.StatusPublished}
	assert.True(t, o.IsPublished())
}

func TestOffer_IsPublished_FalseForDraftStatus(t *testing.T) {
	o := offer.Offer{Status: offer.StatusDraft}
	assert.False(t, o.IsPublished())
}

func TestOffer_IsPublished_FalseForReadyStatus(t *testing.T) {
	o := offer.Offer{Status: offer.StatusReady}
	assert.False(t, o.IsPublished(), "ready is saved but not yet published — same visibility as draft")
}

func TestOfferStatus_IsValid_KnownValues(t *testing.T) {
	assert.True(t, offer.StatusDraft.IsValid())
	assert.True(t, offer.StatusReady.IsValid())
	assert.True(t, offer.StatusPublished.IsValid())
}

func TestOfferStatus_IsValid_UnknownValue_ReturnsFalse(t *testing.T) {
	assert.False(t, offer.Status("archived").IsValid())
	assert.False(t, offer.Status("").IsValid())
}
