package exchange

import (
	"fmt"
	"time"

	"github.com/talav/civil"
)

// A Session is a stretch of the day during which an exchange trades, in the
// exchange's own time zone: 09:30 to 16:00. An exchange that stops for lunch
// has two sessions a day.
//
// A session lies within one day. Its start is part of it and its end is not.
// An end of midnight means the end of the day, so 00:00 to 00:00 is the
// whole day.
//
// A Session comes only from a constructor, so it is never empty or reversed.
// Sessions are comparable with ==. The zero value is the whole day.
type Session struct {
	// start is the time of day at which trading begins.
	start civil.Time

	// end is the time of day at which trading stops. Midnight stands for the
	// end of the day, not its beginning.
	end civil.Time
}

// NewSession returns the session from start to end.
//
// It returns an error if end is not after start. An end of midnight is the
// end of the day and is after any start.
func NewSession(start, end civil.Time) (Session, error) {
	s := Session{start: start, end: end}
	if err := s.validate(); err != nil {
		return Session{}, err
	}

	return s, nil
}

// Start returns the time of day at which trading begins.
func (s Session) Start() civil.Time { return s.start }

// End returns the time of day at which trading stops. Midnight means the end
// of the day.
func (s Session) End() civil.Time { return s.end }

// Duration returns how long the session lasts on a day without a clock
// change: 6h30m for 09:30 to 16:00.
func (s Session) Duration() time.Duration {
	return s.endOffset() - s.start.SinceMidnight()
}

// Contains reports whether trading is on at the time of day t. The start is
// inside the session and the end is outside it.
func (s Session) Contains(t civil.Time) bool {
	offset := t.SinceMidnight()

	return offset >= s.start.SinceMidnight() && offset < s.endOffset()
}

// String returns the session as its start and end with a hyphen between
// them: "09:30:00-16:00:00".
func (s Session) String() string {
	return s.start.String() + "-" + s.end.String()
}

func (s Session) validate() error {
	if s.endOffset() <= s.start.SinceMidnight() {
		return fmt.Errorf("session end %s must be after its start %s", s.end, s.start)
	}

	return nil
}

// endsAtMidnight reports whether the session runs to the end of the day.
func (s Session) endsAtMidnight() bool {
	return s.end == civil.Time{}
}

// endOffset returns how far into the day the session ends: a full day for an
// end of midnight.
func (s Session) endOffset() time.Duration {
	if s.endsAtMidnight() {
		return hoursPerDay * time.Hour
	}

	return s.end.SinceMidnight()
}
