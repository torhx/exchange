package exchange_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/torhx/exchange"
)

func TestNewSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		start        string
		end          string
		want         string
		wantDuration time.Duration
		wantErr      string
	}{
		{name: "regular day", start: "09:30", end: "16:00", want: "09:30:00-16:00:00", wantDuration: 6*time.Hour + 30*time.Minute},
		{name: "one minute", start: "09:30", end: "09:31", want: "09:30:00-09:31:00", wantDuration: time.Minute},
		{name: "with seconds", start: "09:30:15", end: "09:30:45", want: "09:30:15-09:30:45", wantDuration: 30 * time.Second},
		{name: "until the end of the day", start: "18:00", end: "00:00", want: "18:00:00-00:00:00", wantDuration: 6 * time.Hour},
		{name: "whole day", start: "00:00", end: "00:00", want: "00:00:00-00:00:00", wantDuration: 24 * time.Hour},
		{name: "from midnight", start: "00:00", end: "08:00", want: "00:00:00-08:00:00", wantDuration: 8 * time.Hour},
		{name: "empty", start: "09:30", end: "09:30", wantErr: "session end 09:30:00 must be after its start 09:30:00"},
		{name: "reversed", start: "16:00", end: "09:30", wantErr: "session end 09:30:00 must be after its start 16:00:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := exchange.NewSession(clock(t, tt.start), clock(t, tt.end))
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Equal(t, exchange.Session{}, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, clock(t, tt.start), got.Start())
			assert.Equal(t, clock(t, tt.end), got.End())
			assert.Equal(t, tt.want, got.String())
			assert.Equal(t, tt.wantDuration, got.Duration())
		})
	}
}

func TestSessionContains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		session exchange.Session
		time    string
		want    bool
	}{
		{name: "at the start", session: session(t, "09:30", "16:00"), time: "09:30", want: true},
		{name: "just before the start", session: session(t, "09:30", "16:00"), time: "09:29:59.999999999", want: false},
		{name: "in the middle", session: session(t, "09:30", "16:00"), time: "12:00", want: true},
		{name: "just before the end", session: session(t, "09:30", "16:00"), time: "15:59:59.999999999", want: true},
		{name: "at the end", session: session(t, "09:30", "16:00"), time: "16:00", want: false},
		{name: "midnight is before a session that runs to the end of the day", session: session(t, "18:00", "00:00"), time: "00:00", want: false},
		{name: "last instant of a session that runs to the end of the day", session: session(t, "18:00", "00:00"), time: "23:59:59.999999999", want: true},
		{name: "midnight in the whole day", session: session(t, "00:00", "00:00"), time: "00:00", want: true},
		{name: "any time in the whole day", session: session(t, "00:00", "00:00"), time: "13:37", want: true},
		{name: "any time in the zero value", session: exchange.Session{}, time: "13:37", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.session.Contains(clock(t, tt.time)))
		})
	}
}
