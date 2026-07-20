package common

import (
	"database/sql/driver"
	"fmt"
	"time"
)

type UTCTime struct {
	time.Time
}

func (t UTCTime) Value() (driver.Value, error) {
	return t.Time, nil
}

func (t *UTCTime) Scan(value interface{}) error {
	if value == nil {
		t.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		t.Time = v
		return nil
	case string:
		parsed, err := time.Parse("2006-01-02 15:04:05.999999-07", v)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339, v)
			if err != nil {
				parsed, err = time.Parse(time.RFC3339Nano, v)
				if err != nil {
					return err
				}
			}
		}
		t.Time = parsed
		return nil
	default:
		return fmt.Errorf("cannot scan %T into UTCTime", value)
	}
}

func Now() UTCTime {
	return UTCTime{Time: time.Now().UTC()}
}

func (t UTCTime) IsZero() bool {
	return t.Time.IsZero()
}

func (t UTCTime) Before(other UTCTime) bool {
	return t.Time.Before(other.Time)
}

func (t UTCTime) After(other UTCTime) bool {
	return t.Time.After(other.Time)
}

func (t UTCTime) Add(d time.Duration) UTCTime {
	return UTCTime{Time: t.Time.Add(d)}
}

func (t UTCTime) Sub(other UTCTime) time.Duration {
	return t.Time.Sub(other.Time)
}
