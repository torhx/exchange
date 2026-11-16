package exchange_test

import (
	"encoding"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/talav/civil"

	"github.com/torhx/exchange"
)

// builtIn returns a built-in exchange that the test knows to exist.
func builtIn(t *testing.T, code string) exchange.Exchange {
	t.Helper()

	e, err := exchange.NewExchangeFromString(code)
	require.NoError(t, err)

	return e
}

// code builds a code that the test knows to be well formed.
func code(t *testing.T, text string) exchange.Code {
	t.Helper()

	c, err := exchange.NewCode(text)
	require.NoError(t, err)

	return c
}

// clock builds a time of day from text such as "09:30".
func clock(t *testing.T, text string) civil.Time {
	t.Helper()

	tm, err := civil.NewTimeFromString(text)
	require.NoError(t, err)

	return tm
}

// day builds a date from text such as "2026-03-09".
func day(t *testing.T, text string) civil.Date {
	t.Helper()

	d, err := civil.NewDateFromString(text)
	require.NoError(t, err)

	return d
}

// session builds a session from two times of day such as "09:30", "16:00".
func session(t *testing.T, start, end string) exchange.Session {
	t.Helper()

	s, err := exchange.NewSession(clock(t, start), clock(t, end))
	require.NoError(t, err)

	return s
}

// utc builds an instant from RFC 3339 text in UTC, without the Z.
func utc(t *testing.T, text string) time.Time {
	t.Helper()

	instant, err := time.Parse(time.RFC3339, text+"Z")
	require.NoError(t, err)

	return instant
}

func workweek() []time.Weekday {
	return []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
}

func TestNewExchange(t *testing.T) {
	t.Parallel()

	regular := []exchange.Session{session(t, "09:30", "16:00")}

	tests := []struct {
		name     string
		code     exchange.Code
		exchName string
		timeZone string
		weekdays []time.Weekday
		sessions []exchange.Session
		wantErr  string
	}{
		{name: "market identifier", code: code(t, "XMIL"), exchName: " Borsa Italiana ", timeZone: "Europe/Rome", weekdays: workweek(), sessions: regular},
		{name: "long code without an identifier", code: code(t, "KRAKEN"), exchName: "Kraken", timeZone: "UTC", weekdays: workweek(), sessions: regular},
		{name: "two sessions with a break", code: code(t, "XMIL"), exchName: "Borsa Italiana", timeZone: "Europe/Rome", weekdays: workweek(), sessions: []exchange.Session{session(t, "09:00", "12:00"), session(t, "13:00", "17:30")}},
		{name: "sessions that touch", code: code(t, "XMIL"), exchName: "Borsa Italiana", timeZone: "Europe/Rome", weekdays: workweek(), sessions: []exchange.Session{session(t, "09:00", "12:00"), session(t, "12:00", "17:30")}},
		{name: "one trading day a week", code: code(t, "XMIL"), exchName: "Borsa Italiana", timeZone: "Europe/Rome", weekdays: []time.Weekday{time.Sunday}, sessions: regular},
		{name: "zero code", exchName: "X", timeZone: "UTC", weekdays: workweek(), sessions: regular, wantErr: "code is missing"},
		{name: "empty name", code: code(t, "XMIL"), exchName: "  ", timeZone: "UTC", weekdays: workweek(), sessions: regular, wantErr: "name is missing"},
		{name: "empty time zone", code: code(t, "XMIL"), exchName: "X", timeZone: "", weekdays: workweek(), sessions: regular, wantErr: "time zone is missing"},
		{name: "unknown time zone", code: code(t, "XMIL"), exchName: "X", timeZone: "Mars/Olympus", weekdays: workweek(), sessions: regular, wantErr: `time zone "Mars/Olympus": unknown time zone Mars/Olympus`},
		{name: "no weekdays", code: code(t, "XMIL"), exchName: "X", timeZone: "UTC", sessions: regular, wantErr: "no trading weekdays"},
		{name: "weekday given twice", code: code(t, "XMIL"), exchName: "X", timeZone: "UTC", weekdays: []time.Weekday{time.Monday, time.Monday}, sessions: regular, wantErr: "weekday Monday is given twice"},
		{name: "weekday past Saturday", code: code(t, "XMIL"), exchName: "X", timeZone: "UTC", weekdays: []time.Weekday{7}, sessions: regular, wantErr: "weekday 7 must be in 0..6"},
		{name: "negative weekday", code: code(t, "XMIL"), exchName: "X", timeZone: "UTC", weekdays: []time.Weekday{-1}, sessions: regular, wantErr: "weekday -1 must be in 0..6"},
		{name: "no sessions", code: code(t, "XMIL"), exchName: "X", timeZone: "UTC", weekdays: workweek(), wantErr: "no sessions"},
		{name: "sessions that overlap", code: code(t, "XMIL"), exchName: "X", timeZone: "UTC", weekdays: workweek(), sessions: []exchange.Session{session(t, "09:00", "12:00"), session(t, "11:00", "17:30")}, wantErr: "session 11:00:00-17:30:00 must start after session 09:00:00-12:00:00 ends"},
		{name: "sessions out of order", code: code(t, "XMIL"), exchName: "X", timeZone: "UTC", weekdays: workweek(), sessions: []exchange.Session{session(t, "13:00", "17:30"), session(t, "09:00", "12:00")}, wantErr: "session 09:00:00-12:00:00 must start after session 13:00:00-17:30:00 ends"},
		{name: "session after one that ends the day", code: code(t, "XMIL"), exchName: "X", timeZone: "UTC", weekdays: workweek(), sessions: []exchange.Session{session(t, "09:00", "00:00"), session(t, "23:00", "23:30")}, wantErr: "session 23:00:00-23:30:00 must start after session 09:00:00-00:00:00 ends"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := exchange.NewExchange(tt.code, tt.exchName, tt.timeZone, tt.weekdays, tt.sessions)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.False(t, got.IsValid())

				return
			}

			require.NoError(t, err)
			assert.True(t, got.IsValid())
			assert.Equal(t, tt.code, got.Code())
			assert.Equal(t, tt.code.String(), got.String())
			assert.Equal(t, strings.TrimSpace(tt.exchName), got.Name())
			assert.Equal(t, tt.timeZone, got.TimeZone())
			assert.Equal(t, tt.timeZone, got.Location().String())
			assert.Equal(t, tt.sessions, got.Sessions())
		})
	}
}

func TestNewExchangeCopiesItsSessions(t *testing.T) {
	t.Parallel()

	sessions := []exchange.Session{session(t, "09:30", "16:00")}

	e, err := exchange.NewExchange(code(t, "XMIL"), "Borsa Italiana", "Europe/Rome", workweek(), sessions)
	require.NoError(t, err)

	sessions[0] = session(t, "10:00", "11:00")
	e.Sessions()[0] = session(t, "12:00", "13:00")

	assert.Equal(t, []exchange.Session{session(t, "09:30", "16:00")}, e.Sessions())
}

func TestNewExchangeFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		code         string
		wantCode     string
		wantName     string
		wantTimeZone string
		wantWeekdays []time.Weekday
		wantSessions []exchange.Session
		wantDuration time.Duration
		wantErr      string
	}{
		{
			name: "New York", code: "XNYS", wantCode: "XNYS", wantName: "New York Stock Exchange", wantTimeZone: "America/New_York",
			wantWeekdays: workweek(), wantSessions: []exchange.Session{session(t, "09:30", "16:00")}, wantDuration: 6*time.Hour + 30*time.Minute,
		},
		{
			name: "lower case with spaces", code: " xnas\n", wantCode: "XNAS", wantName: "Nasdaq", wantTimeZone: "America/New_York",
			wantWeekdays: workweek(), wantSessions: []exchange.Session{session(t, "09:30", "16:00")}, wantDuration: 6*time.Hour + 30*time.Minute,
		},
		{
			name: "options in Chicago", code: "XCBO", wantCode: "XCBO", wantName: "Cboe Options Exchange", wantTimeZone: "America/Chicago",
			wantWeekdays: workweek(), wantSessions: []exchange.Session{session(t, "08:30", "15:00")}, wantDuration: 6*time.Hour + 30*time.Minute,
		},
		{
			name: "London", code: "XLON", wantCode: "XLON", wantName: "London Stock Exchange", wantTimeZone: "Europe/London",
			wantWeekdays: workweek(), wantSessions: []exchange.Session{session(t, "08:00", "16:30")}, wantDuration: 8*time.Hour + 30*time.Minute,
		},
		{
			name: "lunch break", code: "XTKS", wantCode: "XTKS", wantName: "Tokyo Stock Exchange", wantTimeZone: "Asia/Tokyo",
			wantWeekdays: workweek(), wantSessions: []exchange.Session{session(t, "09:00", "11:30"), session(t, "12:30", "15:30")}, wantDuration: 5*time.Hour + 30*time.Minute,
		},
		{
			name: "always open", code: "BINANCE", wantCode: "BINANCE", wantName: "Binance", wantTimeZone: "UTC",
			wantWeekdays: []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday},
			wantSessions: []exchange.Session{session(t, "00:00", "00:00")}, wantDuration: 24 * time.Hour,
		},
		{name: "empty", code: "", wantErr: `invalid exchange code "": must be 2 to 12 letters or digits`},
		{name: "not a code", code: "X-NYS", wantErr: `invalid exchange code "X-NYS": must be 2 to 12 letters or digits`},
		{name: "unknown code", code: "XXXX", wantErr: `invalid exchange "XXXX": not in the built-in list`},
		{name: "acronym, not the exchange's code", code: "nyse", wantErr: `invalid exchange "NYSE": not in the built-in list`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := exchange.NewExchangeFromString(tt.code)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.False(t, got.IsValid())

				return
			}

			require.NoError(t, err)
			assert.True(t, got.IsValid())
			assert.Equal(t, code(t, tt.wantCode), got.Code())

			byCode, err := exchange.NewExchangeFromCode(code(t, tt.wantCode))
			require.NoError(t, err)
			assert.Equal(t, got, byCode)
			assert.Equal(t, tt.wantName, got.Name())
			assert.Equal(t, tt.wantTimeZone, got.TimeZone())
			assert.Equal(t, tt.wantWeekdays, got.Weekdays())
			assert.Equal(t, tt.wantSessions, got.Sessions())
			assert.Equal(t, tt.wantSessions[0].Start(), got.Open())
			assert.Equal(t, tt.wantSessions[len(tt.wantSessions)-1].End(), got.Close())
			assert.Equal(t, tt.wantDuration, got.TradingDuration())
			assert.Equal(t, tt.wantDuration == 24*time.Hour, got.IsAlwaysOpen())
		})
	}
}

func TestExchanges(t *testing.T) {
	t.Parallel()

	exchanges, err := exchange.Exchanges()
	require.NoError(t, err)

	codes := make([]string, len(exchanges))

	for i, e := range exchanges {
		codes[i] = e.Code().String()

		assert.True(t, e.IsValid(), e.String())
		assert.NotEmpty(t, e.Name(), e.String())
		assert.NotEmpty(t, e.Sessions(), e.String())

		again, err := exchange.NewExchangeFromCode(e.Code())
		require.NoError(t, err)
		assert.Equal(t, e, again)
	}

	assert.Equal(t, []string{
		"XNYS", "XNAS", "ARCX", "XASE", "BATS", "XCBO", "XCBF", "XTSE", "XLON", "XPAR", "XAMS", "XETR", "XSWX",
		"XTKS", "XHKG", "XSHG", "XKRX", "XBOM", "XSES", "XASX", "BINANCE", "COINBASE",
	}, codes)
}

func TestNewExchangesFromCSV(t *testing.T) {
	t.Parallel()

	const header = "code,name,time_zone,weekdays,sessions\n"

	tests := []struct {
		name      string
		text      string
		wantCodes []string
		wantErr   string
	}{
		{name: "one exchange", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon Tue Wed Thu Fri,09:00-17:30\n", wantCodes: []string{"XMIL"}},
		{
			name:      "comments, mixed case and two sessions",
			text:      "# my venues\n" + header + "xmil,Borsa Italiana,Europe/Rome,mon TUE Wed thu Fri,09:00-12:00 13:00-17:30\n# another\nKRAKEN,Kraken,UTC,Sun Mon Tue Wed Thu Fri Sat,00:00-00:00\n",
			wantCodes: []string{"XMIL", "KRAKEN"},
		},
		{name: "header alone", text: header, wantCodes: []string{}},
		{name: "empty text", text: "", wantErr: "read header: EOF"},
		{name: "other header", text: "mic,name,tz,days,hours\nXMIL,Borsa Italiana,Europe/Rome,Mon,09:00-17:30\n", wantErr: "header must be code,name,time_zone,weekdays,sessions"},
		{name: "header with too few fields", text: "code,name\n", wantErr: "read header: record on line 1: wrong number of fields"},
		{name: "record with too few fields", text: header + "XMIL,Borsa Italiana,Europe/Rome\n", wantErr: "record on line 2: wrong number of fields"},
		{name: "unknown weekday", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon Funday,09:00-17:30\n", wantErr: `line 2: weekday "Funday" must be one of Sun Mon Tue Wed Thu Fri Sat`},
		{name: "weekday given twice", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon Mon,09:00-17:30\n", wantErr: "line 2: weekday Monday is given twice"},
		{name: "no weekdays", text: header + "XMIL,Borsa Italiana,Europe/Rome,,09:00-17:30\n", wantErr: "line 2: no trading weekdays"},
		{name: "session without a hyphen", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon,09:00\n", wantErr: `line 2: session "09:00" must be start-end, such as 09:30-16:00`},
		{name: "session with a bad start", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon,9am-17:30\n", wantErr: `line 2: session "9am-17:30": invalid time "9am": must be HH:MM, HH:MM:SS or HH:MM:SS.fffffffff`},
		{name: "session with a bad end", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon,09:00-25:00\n", wantErr: `line 2: session "09:00-25:00": invalid time "25:00": hour 25 must be in 0..23`},
		{name: "session that ends before it starts", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon,17:30-09:00\n", wantErr: "line 2: session end 09:00:00 must be after its start 17:30:00"},
		{name: "no sessions", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon,\n", wantErr: "line 2: no sessions"},
		{name: "unknown time zone", text: header + "XMIL,Borsa Italiana,Europe/Atlantis,Mon,09:00-17:30\n", wantErr: `line 2: time zone "Europe/Atlantis": unknown time zone Europe/Atlantis`},
		{name: "bad code on the third line", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon,09:00-17:30\nX,Short,UTC,Mon,09:00-17:30\n", wantErr: `line 3: invalid exchange code "X": must be 2 to 12 letters or digits`},
		{name: "code twice", text: header + "XMIL,Borsa Italiana,Europe/Rome,Mon,09:00-17:30\nxmil,Again,UTC,Mon,09:00-17:30\n", wantErr: "line 3: exchange XMIL appears twice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := exchange.NewExchangesFromCSV(strings.NewReader(tt.text))
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)

			codes := make([]string, len(got))
			for i, e := range got {
				codes[i] = e.Code().String()
			}

			assert.Equal(t, tt.wantCodes, codes)
		})
	}
}

func TestExchangeIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exchange exchange.Exchange
		want     bool
	}{
		{name: "zero value", exchange: exchange.Exchange{}, want: false},
		{name: "built-in exchange", exchange: builtIn(t, "XNYS"), want: true},
		{name: "always-open exchange", exchange: builtIn(t, "COINBASE"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.exchange.IsValid())
		})
	}
}

func TestExchangeTradesOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exchange exchange.Exchange
		weekday  time.Weekday
		want     bool
	}{
		{name: "Monday in New York", exchange: builtIn(t, "XNYS"), weekday: time.Monday, want: true},
		{name: "Friday in New York", exchange: builtIn(t, "XNYS"), weekday: time.Friday, want: true},
		{name: "Saturday in New York", exchange: builtIn(t, "XNYS"), weekday: time.Saturday, want: false},
		{name: "Sunday in New York", exchange: builtIn(t, "XNYS"), weekday: time.Sunday, want: false},
		{name: "Sunday on an always-open exchange", exchange: builtIn(t, "BINANCE"), weekday: time.Sunday, want: true},
		{name: "day past Saturday", exchange: builtIn(t, "BINANCE"), weekday: 7, want: false},
		{name: "negative day", exchange: builtIn(t, "BINANCE"), weekday: -1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.exchange.TradesOn(tt.weekday))
		})
	}
}

func TestExchangeDateOf(t *testing.T) {
	t.Parallel()

	// 02:00 UTC on March 10 is still the evening of March 9 in New York and
	// already late morning on March 10 in Tokyo.
	instant := utc(t, "2026-03-10T02:00:00")

	tests := []struct {
		name         string
		exchange     exchange.Exchange
		wantDate     string
		wantDateTime string
	}{
		{name: "west of UTC", exchange: builtIn(t, "XNYS"), wantDate: "2026-03-09", wantDateTime: "2026-03-09T22:00:00"},
		{name: "east of UTC", exchange: builtIn(t, "XTKS"), wantDate: "2026-03-10", wantDateTime: "2026-03-10T11:00:00"},
		{name: "in UTC", exchange: builtIn(t, "BINANCE"), wantDate: "2026-03-10", wantDateTime: "2026-03-10T02:00:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantDate, tt.exchange.DateOf(instant).String())

			local := tt.exchange.DateTimeOf(instant)
			assert.Equal(t, tt.wantDateTime, local.String())
			assert.True(t, instant.Equal(tt.exchange.InstantOf(local)), "InstantOf must give the instant back")
		})
	}
}

func TestExchangeIsRegularHours(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exchange exchange.Exchange
		instant  time.Time
		want     bool
	}{
		{name: "New York at the open, daylight time", exchange: builtIn(t, "XNYS"), instant: utc(t, "2026-03-09T13:30:00"), want: true},
		{name: "New York a second before the open", exchange: builtIn(t, "XNYS"), instant: utc(t, "2026-03-09T13:29:59"), want: false},
		{name: "New York a second before the close", exchange: builtIn(t, "XNYS"), instant: utc(t, "2026-03-09T19:59:59"), want: true},
		{name: "New York at the close", exchange: builtIn(t, "XNYS"), instant: utc(t, "2026-03-09T20:00:00"), want: false},
		{name: "New York at the open, standard time", exchange: builtIn(t, "XNYS"), instant: utc(t, "2026-01-05T14:30:00"), want: true},
		{name: "New York on a Saturday", exchange: builtIn(t, "XNYS"), instant: utc(t, "2026-03-07T15:00:00"), want: false},
		{name: "New York at noon on a holiday that is a weekday", exchange: builtIn(t, "XNYS"), instant: utc(t, "2026-11-26T17:00:00"), want: true},
		{name: "Tokyo in the morning session", exchange: builtIn(t, "XTKS"), instant: utc(t, "2026-03-09T02:00:00"), want: true},
		{name: "Tokyo at lunch", exchange: builtIn(t, "XTKS"), instant: utc(t, "2026-03-09T03:00:00"), want: false},
		{name: "Tokyo in the afternoon session", exchange: builtIn(t, "XTKS"), instant: utc(t, "2026-03-09T04:00:00"), want: true},
		{name: "always-open exchange on a Sunday night", exchange: builtIn(t, "BINANCE"), instant: utc(t, "2026-03-08T23:59:59"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.exchange.IsRegularHours(tt.instant))
		})
	}
}

func TestExchangeRegularOpenAndClose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		exchange  exchange.Exchange
		date      string
		wantOpen  time.Time
		wantClose time.Time
	}{
		{name: "New York in standard time", exchange: builtIn(t, "XNYS"), date: "2026-11-27", wantOpen: utc(t, "2026-11-27T14:30:00"), wantClose: utc(t, "2026-11-27T21:00:00")},
		{name: "New York in daylight time", exchange: builtIn(t, "XNYS"), date: "2026-07-01", wantOpen: utc(t, "2026-07-01T13:30:00"), wantClose: utc(t, "2026-07-01T20:00:00")},
		{name: "Chicago options close with New York", exchange: builtIn(t, "XCBO"), date: "2026-07-01", wantOpen: utc(t, "2026-07-01T13:30:00"), wantClose: utc(t, "2026-07-01T20:00:00")},
		{name: "Tokyo closes after its second session", exchange: builtIn(t, "XTKS"), date: "2026-07-01", wantOpen: utc(t, "2026-07-01T00:00:00"), wantClose: utc(t, "2026-07-01T06:30:00")},
		{name: "always-open exchange closes at the next midnight", exchange: builtIn(t, "BINANCE"), date: "2026-12-31", wantOpen: utc(t, "2026-12-31T00:00:00"), wantClose: utc(t, "2027-01-01T00:00:00")},
		{name: "a Saturday still has regular times", exchange: builtIn(t, "XLON"), date: "2026-01-03", wantOpen: utc(t, "2026-01-03T08:00:00"), wantClose: utc(t, "2026-01-03T16:30:00")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			open := tt.exchange.RegularOpen(day(t, tt.date))
			closing := tt.exchange.RegularClose(day(t, tt.date))

			assert.True(t, tt.wantOpen.Equal(open), "open %s", open)
			assert.True(t, tt.wantClose.Equal(closing), "close %s", closing)
			assert.Equal(t, tt.exchange.Location(), open.Location())
		})
	}
}

func TestExchangeText(t *testing.T) {
	t.Parallel()

	var (
		_ encoding.TextMarshaler   = exchange.Exchange{}
		_ encoding.TextUnmarshaler = &exchange.Exchange{}
	)

	custom, err := exchange.NewExchange(code(t, "KRAKEN"), "Kraken", "UTC", workweek(), []exchange.Session{session(t, "00:00", "00:00")})
	require.NoError(t, err)

	tests := []struct {
		name      string
		exchange  exchange.Exchange
		want      string
		readsBack bool
	}{
		{name: "built-in exchange", exchange: builtIn(t, "XNYS"), want: "XNYS", readsBack: true},
		{name: "built-in exchange with a long code", exchange: builtIn(t, "COINBASE"), want: "COINBASE", readsBack: true},
		{name: "exchange outside the list", exchange: custom, want: "KRAKEN", readsBack: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			text, err := tt.exchange.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(text))

			value, err := tt.exchange.Value()
			require.NoError(t, err)
			assert.Equal(t, tt.want, value)

			var parsed exchange.Exchange

			err = parsed.UnmarshalText(text)
			if !tt.readsBack {
				require.Error(t, err)
				assert.False(t, parsed.IsValid())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.exchange, parsed)
		})
	}
}

func TestExchangeJSON(t *testing.T) {
	t.Parallel()

	type payload struct {
		Listing exchange.Exchange     `json:"listing"`
		Volumes map[exchange.Code]int `json:"volumes"`
	}

	in := payload{Listing: builtIn(t, "XNAS"), Volumes: map[exchange.Code]int{builtIn(t, "ARCX").Code(): 7}}

	data, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"listing":"XNAS","volumes":{"ARCX":7}}`, string(data))

	var out payload

	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, in, out)
}

func TestExchangeScan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		src     any
		want    string
		wantErr string
	}{
		{name: "string", src: "XLON", want: "XLON"},
		{name: "bytes in lower case", src: []byte("xtks"), want: "XTKS"},
		{name: "null", src: nil, wantErr: "invalid exchange: NULL"},
		{name: "unsupported type", src: int64(1), wantErr: "invalid exchange: unsupported type int64"},
		{name: "unknown code", src: "NYSE", wantErr: `invalid exchange "NYSE": not in the built-in list`},
		{name: "not a code", src: "N", wantErr: `invalid exchange code "N": must be 2 to 12 letters or digits`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := builtIn(t, "XNYS")

			err := got.Scan(tt.src)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Equal(t, "XNYS", got.String(), "a failed scan must leave the exchange unchanged")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.String())
		})
	}
}
