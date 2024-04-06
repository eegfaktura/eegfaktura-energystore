package utils

import (
	"errors"
	"fmt"
	"time"
)

func GetMonthDuration(from, to time.Time) (startFromYear, months int) {

	y1, M1, d1 := from.Date()
	y2, M2, d2 := to.Date()

	var year, month, day int
	year = int(y2 - y1)
	month = int(M2 - M1)
	day = int(d2 - d1)

	if day < 0 {
		// days in month:
		t := time.Date(y1, M1, 32, 0, 0, 0, 0, to.Location())
		day += 32 - t.Day()
		month--
	}
	if month < 0 {
		month += 12
		year--
	}

	months = month + (year * 12)
	startFromYear = y1
	return
}

func ParseTime(strTime string, fallback int64) (time.Time, error) {
	var y, m, d, hh, mm, ss int
	if _, err := fmt.Sscanf(strTime, "%d.%d.%d %d:%d:%d", &d, &m, &y, &hh, &mm, &ss); err != nil {
		return time.UnixMilli(fallback), err
	}
	return time.Date(y, time.Month(m), d, hh, mm, ss, 0, time.Local), nil
}

func ConvertRowIdToTime(prefix, rowId string) (time.Time, error) {
	var y, m, d, hh, mm, ss int
	if _, err := fmt.Sscanf(rowId, fmt.Sprintf("%s/%%d/%%d/%%d/%%d/%%d/%%d", prefix), &y, &m, &d, &hh, &mm, &ss); err != nil {
		return time.Now(), err
	}
	return time.Date(y, time.Month(m), d, hh, mm, ss, 0, time.Local), nil
}

func ConvertTimeToRowId(prefix, strTime string) (string, error) {
	var y, m, d, hh, mm, ss int
	if _, err := fmt.Sscanf(strTime, "%d.%d.%d %d:%d:%d", &d, &m, &y, &hh, &mm, &ss); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%.4d/%.2d/%.2d/%.2d/%.2d/%.2d", prefix, y, m, d, hh, mm, ss), nil
}

func ConvertUnixTimeToRowId(prefix string, time time.Time) (string, error) {
	return fmt.Sprintf("%s%.4d/%.2d/%.2d/%.2d/%.2d/%.2d",
		prefix,
		time.Year(),
		int(time.Month()),
		time.Day(),
		time.Hour(),
		time.Minute(),
		time.Second()), nil
}

func ConvertRowIdToTimeString(prefix, rawId string, loc *time.Location) (string, *time.Time, error) {
	var y, m, d, hh, mm, ss int
	if _, err := fmt.Sscanf(rawId, fmt.Sprintf("%s/%%d/%%d/%%d/%%d/%%d/%%d", prefix), &y, &m, &d, &hh, &mm, &ss); err != nil {
		return "", nil, err
	}
	time := time.Date(y, time.Month(m), d, hh, mm, 0, 0, loc)
	return fmt.Sprintf("%.2d.%.2d.%.4d %.2d:%.2d:00", d, m, y, hh, mm), &time, nil
}

func CheckTime(previousTime, currentTime *time.Time) bool {

	if previousTime == nil || previousTime.Add(time.Minute*15) == *currentTime {
		return true
	}
	return false
}

func ConvertTimeToStringExcel(t time.Time) string {
	y, m, d := t.Date()
	hh, mm := t.Hour(), t.Minute()
	return fmt.Sprintf("%.2d.%.2d.%.4d %.2d:%.2d:00", d, m, y, hh, mm)
}

func ConvertDate(time time.Time) string {
	year, month, day := time.Date()
	return fmt.Sprintf("%.4d-%.2d-%.2d", year, int(month), day)
}

func DateToString(date time.Time) string {
	return fmt.Sprintf("%.2d.%.2d.%.4d %.2d:%.2d:%.4d", date.Day(), date.Month(), date.Year(), date.Hour(), date.Minute(), date.Second())
}

func StringToTime(date string) time.Time {
	var d, m, y, hh, mm, ss int
	if _, err := fmt.Sscanf(date, "%d.%d.%d %d:%d:%d", &d, &m, &y, &hh, &mm, &ss); err == nil {
		return time.Date(y, time.Month(m), d, hh, mm, ss, 0, time.Local)
	}
	return time.Now()
}

func TruncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

func PeriodToStartEndTime(year, segment int, periodCode string) (time.Time, time.Time, error) {

	switch periodCode {
	case "YM":
		if segment > 0 && segment < 13 {
			return time.Date(year, time.Month(segment), 1, 0, 0, 0, 0, time.UTC),
				time.Date(year, time.Month(segment+1), 0, 0, 0, 0, 0, time.UTC), nil
		}
	case "YQ":
		if segment > 0 && segment < 5 {
			return time.Date(year, time.Month((segment-1)*3+1), 1, 0, 0, 0, 0, time.UTC),
				time.Date(year, time.Month((segment)*3+1), 0, 0, 0, 0, 0, time.UTC), nil
		}
	case "YH":
		if segment > 0 && segment < 3 {
			return time.Date(year, time.Month((segment-1)*6+1), 1, 0, 0, 0, 0, time.UTC),
				time.Date(year, time.Month((segment)*6+1), 0, 0, 0, 0, 0, time.UTC), nil
		}
	case "Y":
		if segment == 0 {
			return time.Date(year, time.Month(1), 1, 0, 0, 0, 0, time.UTC),
				time.Date(year, time.Month(12), 31, 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Now(), time.Now(), errors.New(fmt.Sprintf("Wrong Time-Period (year: %d, segment: %d, type: %s)", year, segment, periodCode))
}
