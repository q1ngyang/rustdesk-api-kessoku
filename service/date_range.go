package service

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CreatedAtRange struct {
	From *time.Time
	To   *time.Time
}

func ParseCreatedAtRange(from, to string) (CreatedAtRange, error) {
	result := CreatedAtRange{}
	parse := func(raw string) (*time.Time, error) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, nil
		}
		if len(raw) > 64 {
			return nil, errors.New("date filter is too long")
		}
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, errors.New("date filter must use RFC3339")
		}
		value = value.UTC()
		return &value, nil
	}
	var err error
	if result.From, err = parse(from); err != nil {
		return CreatedAtRange{}, err
	}
	if result.To, err = parse(to); err != nil {
		return CreatedAtRange{}, err
	}
	if result.From != nil && result.To != nil && !result.From.Before(*result.To) {
		return CreatedAtRange{}, errors.New("date filter start must be before end")
	}
	return result, nil
}

func (r CreatedAtRange) Apply(tx *gorm.DB) *gorm.DB {
	if r.From != nil {
		tx = tx.Where("created_at >= ?", *r.From)
	}
	if r.To != nil {
		tx = tx.Where("created_at < ?", *r.To)
	}
	return tx
}
