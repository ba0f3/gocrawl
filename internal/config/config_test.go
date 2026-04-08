package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoadAllowedOrigins(t *testing.T) {
	viper.Reset()
	viper.Set("ALLOWED_ORIGINS", "http://example.com, http://test.com")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"http://example.com", "http://test.com"}, cfg.Security.AllowedOrigins)
}

func TestLoadAllowedOriginsEmpty(t *testing.T) {
	viper.Reset()
	os.Unsetenv("ALLOWED_ORIGINS")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Empty(t, cfg.Security.AllowedOrigins)
}
