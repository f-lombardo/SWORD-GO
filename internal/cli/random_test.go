package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomLowerAlphaNumeric(t *testing.T) {
	l := 10
	randomString := randomLowerAlphaNumeric(l)

	assert.Len(t, randomString, l)
	assert.Equal(t, strings.ToLower(randomString), randomString)
}
