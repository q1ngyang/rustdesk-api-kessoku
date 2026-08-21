package model

// SecurityInvariantLock serializes database-wide security invariants across
// every Kessoku replica. It contains no secret or user data.
type SecurityInvariantLock struct {
	Name       string `gorm:"size:64;primaryKey"`
	Generation uint64 `gorm:"not null;default:0"`
}
