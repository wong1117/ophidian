package common

import "time"

type UTCTime struct {
	time.Time
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
