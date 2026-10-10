package services

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentialRoundTrip(t *testing.T) {
	enc, err := EncryptCredential("secret-a", "p@ss wörd")
	require.NoError(t, err)
	assert.NotContains(t, enc, "p@ss")
	dec, err := DecryptCredential("secret-a", enc)
	require.NoError(t, err)
	assert.Equal(t, "p@ss wörd", dec)
}

func TestCredentialEmptyStaysEmpty(t *testing.T) {
	enc, err := EncryptCredential("k", "")
	require.NoError(t, err)
	assert.Empty(t, enc)
	dec, err := DecryptCredential("k", "")
	require.NoError(t, err)
	assert.Empty(t, dec)
}

func TestCredentialEncryptionIsRandomised(t *testing.T) {
	a, err := EncryptCredential("k", "same")
	require.NoError(t, err)
	b, err := EncryptCredential("k", "same")
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "a fresh nonce per encryption")
}

func TestCredentialWrongKeyFails(t *testing.T) {
	enc, err := EncryptCredential("secret-a", "pw")
	require.NoError(t, err)
	_, err = DecryptCredential("secret-b", enc)
	assert.Error(t, err)
}

func TestCredentialTamperedOrCorruptedFails(t *testing.T) {
	enc, err := EncryptCredential("k", "pw")
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(enc)
	require.NoError(t, err)

	flipped := append([]byte{}, raw...)
	flipped[len(flipped)-1] ^= 1
	_, err = DecryptCredential("k", base64.StdEncoding.EncodeToString(flipped))
	assert.Error(t, err, "a changed ciphertext must fail authentication")

	_, err = DecryptCredential("k", base64.StdEncoding.EncodeToString(raw[:5]))
	assert.Error(t, err, "shorter than a nonce")
	_, err = DecryptCredential("k", "not base64 !!")
	assert.Error(t, err)
}
