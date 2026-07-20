package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type cronField struct {
	min, max int
	value    int
	any      bool
	step     int
}

func parseCronField(field string, min, max int) (cronField, error) {
	if field == "*" {
		return cronField{min: min, max: max, any: true}, nil
	}

	if strings.Contains(field, "/") {
		parts := strings.SplitN(field, "/", 2)
		if parts[0] != "*" {
			return cronField{}, fmt.Errorf("step only supported with *: %s", field)
		}
		step, err := strconv.Atoi(parts[1])
		if err != nil || step < 1 || step > max-min+1 {
			return cronField{}, fmt.Errorf("invalid step: %s", field)
		}
		return cronField{min: min, max: max, any: true, step: step}, nil
	}

	if strings.Contains(field, ",") {
		v, err := strconv.Atoi(field)
		if err == nil && v >= min && v <= max {
			return cronField{min: min, max: max, value: v}, nil
		}
		return cronField{}, fmt.Errorf("comma-separated values not supported: %s", field)
	}

	v, err := strconv.Atoi(field)
	if err != nil || v < min || v > max {
		return cronField{}, fmt.Errorf("invalid cron field %s: must be %d-%d", field, min, max)
	}
	return cronField{min: min, max: max, value: v}, nil
}

func NextCronTime(expr string, from time.Time) (time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("cron expression must have 5 fields: %s", expr)
	}

	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return time.Time{}, err
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return time.Time{}, err
	}
	day, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return time.Time{}, err
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return time.Time{}, err
	}
	weekday, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return time.Time{}, err
	}

	candidate := time.Date(from.Year(), from.Month(), from.Day(), from.Hour(), from.Minute(), 0, 0, from.Location())
	candidate = candidate.Add(time.Minute)

	for i := 0; i < 525600; i++ {
		if matchField(minute, candidate.Minute()) &&
			matchField(hour, candidate.Hour()) &&
			matchField(day, candidate.Day()) &&
			matchField(month, int(candidate.Month())) &&
			matchField(weekday, int(candidate.Weekday())) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching time found within 1 year for cron: %s", expr)
}

func matchField(f cronField, actual int) bool {
	if f.any {
		if f.step > 0 {
			return actual%f.step == 0
		}
		return true
	}
	return actual == f.value
}
