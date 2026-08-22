package clock

import (
	"time"
)

var beijing = time.FixedZone("CST", 8*60*60)

func Now() time.Time {
	return time.Now().In(beijing)
}

func Format(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(beijing).Format("2006-01-02 15:04:05")
}

func Since(t time.Time) time.Duration {
	if t.IsZero() {
		return 0
	}
	return Now().Sub(t.In(beijing))
}
