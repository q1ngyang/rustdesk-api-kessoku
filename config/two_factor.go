package config

import (
	"errors"
	"time"
)

type TwoFactor struct {
	Enabled      bool          `mapstructure:"enabled"`
	Issuer       string        `mapstructure:"issuer"`
	KeyFile      string        `mapstructure:"key-file"`
	ChallengeTTL time.Duration `mapstructure:"challenge-ttl"`
}

func (t TwoFactor) EffectiveChallengeTTL() time.Duration {
	if t.ChallengeTTL == 0 {
		return 5 * time.Minute
	}
	return t.ChallengeTTL
}

func (t TwoFactor) Validate() error {
	if !t.Enabled {
		return nil
	}
	if !validGovernanceText(t.Issuer, 80) {
		return errors.New("two-factor.issuer is invalid")
	}
	if !validFileReference(t.KeyFile) {
		return errors.New("two-factor.key-file is invalid")
	}
	if ttl := t.EffectiveChallengeTTL(); ttl < time.Minute || ttl > 10*time.Minute {
		return errors.New("two-factor.challenge-ttl must be between one and ten minutes")
	}
	return nil
}
