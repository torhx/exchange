// Package exchange provides Exchange: a venue where instruments trade, with
// the facts about it that do not depend on the date. Those are its code, its
// name, its country, its time zone, the days of the week it trades and its
// regular sessions.
//
// A list of well-known exchanges is built in and comes from
// NewExchangeFromCode or NewExchangeFromString. Any other exchange comes from
// NewExchange, or from a file read by NewExchangesFromCSV.
//
// A Code identifies an exchange. It is a type of its own, so that a time
// zone, a ticker or any other text cannot be passed where a code is meant.
// Each built-in exchange has a named Code, such as XNYS.
//
// An Exchange knows nothing of holidays, early closes or any other dated
// fact; a calendar adds those. Its regular hours say when it trades on an
// ordinary trading day.
package exchange

import (
	"bytes"
	"database/sql/driver"
	_ "embed" // for the built-in list of exchanges
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/talav/civil"
	"github.com/talav/country"
)

const (
	daysPerWeek = 7
	hoursPerDay = 24

	// csvHeader is the first record of a file of exchanges.
	csvHeader = "code,name,country,time_zone,weekdays,sessions"
)

// The positions of the fields of a record in a file of exchanges, in the
// order of csvHeader.
const (
	codeColumn = iota
	nameColumn
	countryColumn
	timeZoneColumn
	weekdaysColumn
	sessionsColumn

	// columnCount is the number of fields in a record.
	columnCount
)

// builtInCSV is the built-in list of exchanges, in the format that
// NewExchangesFromCSV reads.
//
//go:embed exchanges.csv
var builtInCSV []byte

// An Exchange is a venue where instruments trade: a stock exchange, a
// futures exchange, a crypto exchange. It holds what is true of the venue on
// any ordinary day.
//
// An Exchange comes only from a constructor, so it always has a code, a
// name, a country, a time zone that exists, at least one trading day in the
// week and at least one session. Two exchanges are the same venue when their
// codes are equal; an Exchange is not comparable with ==, but its Code is.
//
// The zero value is not an exchange. It stands for a missing one, IsValid
// reports false for it, and other methods are meaningless on it.
type Exchange struct {
	// code identifies the exchange. It is never the zero Code.
	code Code

	// name is the name of the exchange for people to read.
	name string

	// country is the country the exchange is in. It is never the zero
	// Country.
	country country.Country

	// location is the time zone the exchange keeps its hours in. Its trading
	// day is a calendar day there.
	location *time.Location

	// weekdays marks the days of the week on which the exchange trades,
	// indexed by time.Weekday. At least one is set.
	weekdays [daysPerWeek]bool

	// sessions are the regular sessions of a trading day in the exchange's
	// time zone, in order and without overlap. There is at least one.
	sessions []Session
}

// NewExchange returns the exchange with the given code and name in the given
// country, which keeps its hours in the IANA time zone timeZone, trades on
// weekdays and holds the given regular sessions on each of them.
//
// It returns an error if the code is the zero Code, if the name is empty, if
// the country is the zero Country, if the time zone is unknown, if no weekday
// is given or one is given twice, or if there are no sessions or they are not
// in order without overlap.
func NewExchange(code Code, name string, country country.Country, timeZone string, weekdays []time.Weekday, sessions []Session) (Exchange, error) {
	if timeZone == "" {
		return Exchange{}, errors.New("time zone is missing")
	}

	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return Exchange{}, fmt.Errorf("time zone %q: %w", timeZone, err)
	}

	week, err := weekOf(weekdays)
	if err != nil {
		return Exchange{}, err
	}

	e := Exchange{
		code:     code,
		name:     strings.TrimSpace(name),
		country:  country,
		location: location,
		weekdays: week,
		sessions: slices.Clone(sessions),
	}
	if err := e.validate(); err != nil {
		return Exchange{}, err
	}

	return e, nil
}

// NewExchangeFromCode returns the built-in exchange with the given code.
//
// It returns an error if the built-in list has no exchange with that code.
func NewExchangeFromCode(code Code) (Exchange, error) {
	exchanges, err := Exchanges()
	if err != nil {
		return Exchange{}, err
	}

	for _, e := range exchanges {
		if e.code == code {
			return e, nil
		}
	}

	return Exchange{}, fmt.Errorf("invalid exchange %q: not in the built-in list", code)
}

// NewExchangeFromString returns the built-in exchange with the code written
// in s, such as XNYS. The code is read as NewCode reads one.
//
// It returns an error if s is not a code, or if the built-in list has no
// exchange with that code.
func NewExchangeFromString(s string) (Exchange, error) {
	code, err := NewCode(s)
	if err != nil {
		return Exchange{}, err
	}

	return NewExchangeFromCode(code)
}

// NewExchangesFromCSV reads exchanges from CSV text, one for each record.
// The first record must be the header
//
//	code,name,country,time_zone,weekdays,sessions
//
// and each later record gives the arguments of NewExchange: the code as
// NewCode reads one, the country as country.NewCountryFromString reads one,
// weekdays as three-letter English day names and sessions as start-end pairs
// of times, each list separated by spaces. A line that starts with # is a
// comment.
//
//	XTKS,Tokyo Stock Exchange,JP,Asia/Tokyo,Mon Tue Wed Thu Fri,09:00-11:30 12:30-15:30
//
// It returns an error, naming the line, if the text is not such a file, if a
// record does not make an exchange, or if a code appears twice.
func NewExchangesFromCSV(r io.Reader) ([]Exchange, error) {
	reader := csv.NewReader(r)
	reader.Comment = '#'
	reader.FieldsPerRecord = columnCount

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	if strings.Join(header, ",") != csvHeader {
		return nil, fmt.Errorf("header must be %s", csvHeader)
	}

	var exchanges []Exchange

	seen := make(map[Code]bool)

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return exchanges, nil
		}

		if err != nil {
			return nil, err
		}

		line, _ := reader.FieldPos(0)

		e, err := newExchangeFromRecord(record)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}

		if seen[e.code] {
			return nil, fmt.Errorf("line %d: exchange %s appears twice", line, e.code)
		}

		seen[e.code] = true

		exchanges = append(exchanges, e)
	}
}

// Exchanges returns the built-in exchanges, in the order of the list.
func Exchanges() ([]Exchange, error) {
	return NewExchangesFromCSV(bytes.NewReader(builtInCSV))
}

// IsValid reports whether e is an exchange. It is false only for the zero
// value, which stands for a missing one; every Exchange from a constructor
// is valid.
func (e Exchange) IsValid() bool { return e.code.IsValid() }

// Code returns the code of the exchange, such as XNYS.
func (e Exchange) Code() Code { return e.code }

// Name returns the name of the exchange for people to read.
func (e Exchange) Name() string { return e.name }

// Country returns the country the exchange is in, such as US for XNYS.
func (e Exchange) Country() country.Country { return e.country }

// TimeZone returns the IANA name of the time zone the exchange keeps its
// hours in, such as America/New_York.
func (e Exchange) TimeZone() string { return e.location.String() }

// Location returns the time zone the exchange keeps its hours in.
func (e Exchange) Location() *time.Location { return e.location }

// Weekdays returns the days of the week on which the exchange trades, Sunday
// first.
func (e Exchange) Weekdays() []time.Weekday {
	var days []time.Weekday

	for day, trades := range e.weekdays {
		if trades {
			days = append(days, time.Weekday(day))
		}
	}

	return days
}

// TradesOn reports whether the exchange trades on the given day of the
// week. It knows nothing of holidays.
func (e Exchange) TradesOn(weekday time.Weekday) bool {
	return weekday >= time.Sunday && weekday <= time.Saturday && e.weekdays[weekday]
}

// Sessions returns the regular sessions of a trading day, in order.
func (e Exchange) Sessions() []Session {
	return slices.Clone(e.sessions)
}

// Open returns the time of day at which the first regular session starts.
func (e Exchange) Open() civil.Time {
	return e.sessions[0].start
}

// Close returns the time of day at which the last regular session ends.
// Midnight means the end of the day.
func (e Exchange) Close() civil.Time {
	return e.sessions[len(e.sessions)-1].end
}

// TradingDuration returns how long the exchange trades on a regular trading
// day, breaks left out: 6h30m for 09:30 to 16:00.
func (e Exchange) TradingDuration() time.Duration {
	var total time.Duration

	for _, session := range e.sessions {
		total += session.Duration()
	}

	return total
}

// IsAlwaysOpen reports whether the exchange trades around the clock on
// every day of the week, as a crypto exchange does.
func (e Exchange) IsAlwaysOpen() bool {
	return len(e.Weekdays()) == daysPerWeek && e.TradingDuration() == hoursPerDay*time.Hour
}

// DateOf returns the date on the exchange's calendar at the instant t: the
// trading date a fill or a quote belongs to.
func (e Exchange) DateOf(t time.Time) civil.Date {
	return civil.NewDateFromTime(t.In(e.location))
}

// DateTimeOf returns what a calendar and a clock at the exchange show at the
// instant t.
func (e Exchange) DateTimeOf(t time.Time) civil.DateTime {
	return civil.NewDateTimeFromTime(t.In(e.location))
}

// InstantOf returns the instant at which a calendar and a clock at the
// exchange show dt.
//
// A wall time that the exchange's time zone skips or repeats on that day has
// no single instant. InstantOf returns the one that time.Date picks.
func (e Exchange) InstantOf(dt civil.DateTime) time.Time {
	return dt.In(e.location)
}

// IsRegularHours reports whether the instant t falls on a weekday the
// exchange trades and inside one of its regular sessions.
//
// It knows nothing of holidays, early closes or late opens: it is true at
// noon on a holiday that falls on a trading weekday.
func (e Exchange) IsRegularHours(t time.Time) bool {
	local := t.In(e.location)
	if !e.weekdays[local.Weekday()] {
		return false
	}

	timeOfDay := civil.NewTimeFromTime(local)

	return slices.ContainsFunc(e.sessions, func(s Session) bool { return s.Contains(timeOfDay) })
}

// RegularOpen returns the instant at which the first regular session starts
// on the date d. It is the open of an ordinary trading day, whether or not
// the exchange trades on d.
func (e Exchange) RegularOpen(d civil.Date) time.Time {
	return d.At(e.Open()).In(e.location)
}

// RegularClose returns the instant at which the last regular session ends on
// the date d. It is the close of an ordinary trading day, whether or not the
// exchange trades on d. A calendar compares it with the real close to find
// an early one.
func (e Exchange) RegularClose(d civil.Date) time.Time {
	last := e.sessions[len(e.sessions)-1]
	if last.endsAtMidnight() {
		d = d.AddDays(1)
	}

	return d.At(last.end).In(e.location)
}

// String returns the code of the exchange as text.
func (e Exchange) String() string { return e.code.String() }

// MarshalText returns the code of the exchange. It never fails. It makes
// Exchange an encoding.TextMarshaler, which encoding/json uses for values
// and for map keys.
func (e Exchange) MarshalText() ([]byte, error) {
	return e.code.MarshalText()
}

// UnmarshalText sets e to the built-in exchange that NewExchangeFromString
// reads from text. If the text is not such a code, it returns the error and
// leaves e as it was. It makes *Exchange an encoding.TextUnmarshaler.
//
// A code alone does not give the time zone or the hours of an exchange
// outside the built-in list, so the text of one cannot be read back.
func (e *Exchange) UnmarshalText(text []byte) error {
	parsed, err := NewExchangeFromString(string(text))
	if err != nil {
		return err
	}

	*e = parsed

	return nil
}

// Value returns the code of the exchange for use as a query argument. It
// makes Exchange a driver.Valuer.
func (e Exchange) Value() (driver.Value, error) {
	return e.code.Value()
}

// Scan sets e from a database column value. It makes *Exchange a
// sql.Scanner.
//
// A string or []byte must be a code that NewExchangeFromString accepts. NULL
// is an error; scan a nullable column into a sql.Null[Exchange]. On error e
// is unchanged.
func (e *Exchange) Scan(src any) error {
	switch value := src.(type) {
	case string:
		return e.UnmarshalText([]byte(value))
	case []byte:
		return e.UnmarshalText(value)
	case nil:
		return errors.New("invalid exchange: NULL")
	default:
		return fmt.Errorf("invalid exchange: unsupported type %T", src)
	}
}

func (e Exchange) validate() error {
	if !e.code.IsValid() {
		return errors.New("code is missing")
	}

	if e.name == "" {
		return errors.New("name is missing")
	}

	if !e.country.IsValid() {
		return errors.New("country is missing")
	}

	if len(e.Weekdays()) == 0 {
		return errors.New("no trading weekdays")
	}

	if len(e.sessions) == 0 {
		return errors.New("no sessions")
	}

	for i := 1; i < len(e.sessions); i++ {
		previous, next := e.sessions[i-1], e.sessions[i]
		if previous.endOffset() > next.start.SinceMidnight() {
			return fmt.Errorf("session %s must start after session %s ends", next, previous)
		}
	}

	return nil
}

// newExchangeFromRecord returns the exchange that one CSV record describes.
func newExchangeFromRecord(record []string) (Exchange, error) {
	code, err := NewCode(record[codeColumn])
	if err != nil {
		return Exchange{}, err
	}

	exchangeCountry, err := country.NewCountryFromString(record[countryColumn])
	if err != nil {
		return Exchange{}, err
	}

	weekdays, err := parseWeekdays(record[weekdaysColumn])
	if err != nil {
		return Exchange{}, err
	}

	sessions, err := parseSessions(record[sessionsColumn])
	if err != nil {
		return Exchange{}, err
	}

	return NewExchange(code, record[nameColumn], exchangeCountry, record[timeZoneColumn], weekdays, sessions)
}

// weekOf returns the trading week that lists exactly the given days.
func weekOf(weekdays []time.Weekday) ([daysPerWeek]bool, error) {
	var week [daysPerWeek]bool

	for _, day := range weekdays {
		if day < time.Sunday || day > time.Saturday {
			return week, fmt.Errorf("weekday %d must be in 0..6", int(day))
		}

		if week[day] {
			return week, fmt.Errorf("weekday %s is given twice", day)
		}

		week[day] = true
	}

	return week, nil
}

// parseWeekdays reads days of the week written as three-letter English
// names separated by spaces, such as "Mon Tue Wed Thu Fri".
func parseWeekdays(s string) ([]time.Weekday, error) {
	names := strings.Fields(s)
	weekdays := make([]time.Weekday, 0, len(names))

	for _, name := range names {
		day := slices.IndexFunc(dayNames(), func(known string) bool { return strings.EqualFold(known, name) })
		if day < 0 {
			return nil, fmt.Errorf("weekday %q must be one of %s", name, strings.Join(dayNames(), " "))
		}

		weekdays = append(weekdays, time.Weekday(day))
	}

	return weekdays, nil
}

// dayNames returns the three-letter names of the days of the week, Sunday
// first, so that a name's position is its time.Weekday.
func dayNames() []string {
	names := make([]string, daysPerWeek)
	for day := range names {
		names[day] = time.Weekday(day).String()[:3]
	}

	return names
}

// parseSessions reads sessions written as start-end pairs of times separated
// by spaces, such as "09:00-11:30 12:30-15:30".
func parseSessions(s string) ([]Session, error) {
	fields := strings.Fields(s)
	sessions := make([]Session, 0, len(fields))

	for _, field := range fields {
		startText, endText, ok := strings.Cut(field, "-")
		if !ok {
			return nil, fmt.Errorf("session %q must be start-end, such as 09:30-16:00", field)
		}

		start, err := civil.NewTimeFromString(startText)
		if err != nil {
			return nil, fmt.Errorf("session %q: %w", field, err)
		}

		end, err := civil.NewTimeFromString(endText)
		if err != nil {
			return nil, fmt.Errorf("session %q: %w", field, err)
		}

		session, err := NewSession(start, end)
		if err != nil {
			return nil, err
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}
