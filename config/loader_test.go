package config

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	os.Setenv("HOME", "./testdata")
	id, secret, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "xxx", id)
	assert.Equal(t, "yyy", secret)
}
