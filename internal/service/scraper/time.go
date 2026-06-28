package scraper

import "time"

const businessTimeZone = "Europe/Moscow"

func nowInBusinessTZ() time.Time {
	loc, err := time.LoadLocation(businessTimeZone)
	if err != nil {
		return time.Now()
	}

	return time.Now().In(loc)
}
