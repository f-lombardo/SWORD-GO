package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomLowerAlphaNumeric(t *testing.T) {
	l := 10
	randomString := RandomLowerAlphaNumeric(l)

	assert.Len(t, randomString, l)
	assert.Equal(t, strings.ToLower(randomString), randomString)
}
