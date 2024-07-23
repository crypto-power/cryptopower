package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ararog/timeago"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	secondInMinute = 60
	secondsInHour  = 60 * secondInMinute
	secondsInDay   = 24 * secondsInHour
)

// File holds all time manipulation implementations required for a given page UI.

// TimeAgo returns the elapsed time since now.
func TimeAgo(timestamp int64) string {
	timeAgo, _ := timeago.TimeAgoWithTime(time.Now(), time.Unix(timestamp, 0))
	return timeAgo
}

// FormatDateOrTime formats the provided timestamp parameter as follows.
// If the provided matches to time within today, the time difference between now
// and then is returned.
// If it matches to time within the previous one day, "yesterday" is returned.
// Otherwise the formatted time and date is returned.
func FormatDateOrTime(timestamp int64) string {
	utcTime := time.Unix(timestamp, 0).UTC()
	timeDiff := int(time.Now().UTC().Sub(utcTime).Seconds())

	if timeDiff <= secondsInDay {
		// Provided timestamp is within the same day(Today).
		return TimeAgo(timestamp)
	} else if timeDiff <= 2*secondsInDay {
		// Provided timestamp is within the previous day.
		return values.String(values.StrYesterday)
	}

	t := strings.Split(utcTime.Format(time.UnixDate), " ")
	t2 := t[2]
	year := strconv.Itoa(utcTime.Year())
	if t[2] == "" {
		t2 = t[3]
	}
	return fmt.Sprintf("%s %s, %s", t[1], t2, year)
}

// TimeFormat formats the provided `secs` parameter into:
//   - Day(s) if the seconds exceed or are equal to more than 1 day.
//   - Hour(s) if the seconds exceed or are equal to more than 1 hour.
//   - Minute(s) if the seconds exceed or are equal to more than 1 minute.
//   - Otherwises returns the actual seconds passed.
//
// Parameter `long` sets the long format of time description formatted to otherwise
// it sets the short format.
func TimeFormat(secs int, long bool) string {
	val := "s"
	if long {
		val = " " + values.String(values.StrSeconds)
	}

	if secs >= secondsInDay {
		val = "d"
		if long {
			val = " " + values.String(values.StrDays)
		}
		days := secs / secondsInDay
		return fmt.Sprintf("%d%s", days, val)
	} else if secs >= secondsInHour {
		val = "h"
		if long {
			val = " " + values.String(values.StrHours)
		}
		hours := secs / secondsInHour
		return fmt.Sprintf("%d%s", hours, val)
	} else if secs >= secondInMinute {
		val = "m"
		if long {
			val = " " + values.String(values.StrMinutes)
		}
		mins := secs / secondInMinute
		return fmt.Sprintf("%d%s", mins, val)
	}

	return fmt.Sprintf("%d %s", secs, val)
}

// SecondsToDays takes time in seconds and returns its string equivalent in the format ddhhmm.
func SecondsToDays(totalTimeLeft int64) string {
	q, r := divMod(totalTimeLeft, secondsInDay)
	timeLeft := time.Duration(r) * time.Second
	if q > 0 {
		return fmt.Sprintf("%dd%s", q, timeLeft.String())
	}
	return timeLeft.String()
}

// divMod divides a numerator by a denominator and returns its quotient and remainder.
func divMod(numerator, denominator int64) (quotient, remainder int64) {
	quotient = numerator / denominator // integer division, decimals are truncated
	remainder = numerator % denominator
	return
}
