# exchange

[![tag](https://img.shields.io/github/tag/torhx/exchange.svg)](https://github.com/torhx/exchange/tags)
[![Go Reference](https://pkg.go.dev/badge/github.com/torhx/exchange.svg)](https://pkg.go.dev/github.com/torhx/exchange)
[![CI](https://github.com/torhx/exchange/workflows/Exchange%20CI/badge.svg)](https://github.com/torhx/exchange/actions)
[![codecov](https://codecov.io/gh/torhx/exchange/graph/badge.svg)](https://codecov.io/gh/torhx/exchange)
[![License](https://img.shields.io/github/license/torhx/exchange)](./LICENSE)

Trading venues for Go: an exchange with its code, name, country, time zone, trading weekdays and regular sessions, that cannot hold an invalid value.

## Features

- **The exchange owns its time zone**: ask it for the trading date of an instant, with no location passed around
- **22 exchanges built in**: the main US, European and Asia-Pacific venues, and two crypto exchanges
- **Regular sessions, breaks included**: Tokyo's morning and afternoon sessions are two sessions, not one
- **Always-open venues**: a crypto exchange trades the whole day, every day
- **Your own exchanges**: from a constructor, or from a CSV file in the format of the built-in list
- **Valid by construction**: constructors return an error instead of a venue with no hours or an unknown time zone
- **Text, JSON and SQL**: written as its code
- **Two dependencies**: [`github.com/talav/civil`](https://github.com/talav/civil) for dates and times of day, and [`github.com/talav/country`](https://github.com/talav/country) for countries

An exchange holds only what is true on any ordinary day. Holidays, early closes and events are dated facts and belong to a calendar, which is a separate component.

## Installation

```bash
go get github.com/torhx/exchange
```

Requires Go 1.27 or later, and the IANA time zone database on the machine. A program that may run without it, such as in a minimal container, imports `time/tzdata`.

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/torhx/exchange"
)

func main() {
    nyse, err := exchange.NewExchangeFromString("XNYS")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(nyse.Name())            // New York Stock Exchange
    fmt.Println(nyse.TimeZone())        // America/New_York
    fmt.Println(nyse.Open())            // 09:30:00
    fmt.Println(nyse.Close())           // 16:00:00
    fmt.Println(nyse.TradingDuration()) // 6h30m0s

    // 02:00 UTC on March 10 is still March 9 in New York.
    instant := time.Date(2026, time.March, 10, 2, 0, 0, 0, time.UTC)
    fmt.Println(nyse.DateOf(instant)) // 2026-03-09
}
```

## Usage

### Identity

```go
e.Code()     // XNYS, an exchange.Code
e.Name()     // "New York Stock Exchange"
e.Country()  // US, a country.Country
e.TimeZone() // "America/New_York"
e.Location() // the *time.Location
e.IsValid()  // true; false only for exchange.Exchange{}
```

`Code` is a type of its own, so a time zone, a ticker or any other text cannot be passed where an exchange code is meant:

```go
code, err := exchange.NewCode("xnys") // XNYS; read in any letter case
code.String()                         // "XNYS"
code == nyse.Code()                   // true

nyse, err := exchange.NewExchangeFromCode(code) // the same as NewExchangeFromString("XNYS")
```

An `Exchange` cannot be compared with `==`, because it holds a list of sessions. Its `Code` can, and works as a map key. Two exchanges are the same venue when their codes are equal.

#### Why a Code and not a MIC

A code is the ISO 10383 market identifier code (MIC) where the venue has one, and the built-in exchanges use theirs. But `Code` is not limited to the four characters of a MIC: it is 2 to 12 letters or digits.

- **Not every venue has a MIC.** Several large crypto exchanges are not in the ISO list, or are listed only for one legal entity or their derivatives arm. A strict MIC type could not name them without inventing a four-character code that a later ISO assignment could collide with.
- **The four-character rule would promise little.** It checks the shape of a code and cannot tell a real identifier from a made-up one without the whole ISO list.

Every real MIC is a valid `Code`. `NewCode` checks the form only, not that an exchange with the code exists.

### Regular hours

```go
e.Weekdays()            // [Monday Tuesday Wednesday Thursday Friday]
e.TradesOn(time.Sunday) // false
e.Sessions()            // [09:30:00-16:00:00]
e.Open()                // 09:30:00
e.Close()               // 16:00:00
e.TradingDuration()     // 6h30m
e.IsAlwaysOpen()        // false
```

A session is a stretch of one day, in the exchange's time zone. Its start is part of it and its end is not. An end of midnight means the end of the day, so `00:00-00:00` is the whole day.

```go
tokyo, _ := exchange.NewExchangeFromString("XTKS")
tokyo.Sessions()        // [09:00:00-11:30:00 12:30:00-15:30:00]
tokyo.TradingDuration() // 5h30m
```

### Dates and instants

```go
e.DateOf(instant)     // the trading date an instant belongs to
e.DateTimeOf(instant) // what the calendar and clock at the exchange show
e.InstantOf(dt)       // the instant of a local date and time

e.RegularOpen(date)  // the instant the first session starts on a date
e.RegularClose(date) // the instant the last session ends on a date

e.IsRegularHours(instant) // on a trading weekday, inside a session
```

These use the regular hours only. `IsRegularHours` is true at noon on a holiday that falls on a weekday, and `RegularClose` gives 16:00 on a day that closes at 13:00. A calendar compares `RegularClose` with the real close to find an early one.

### Your own exchanges

From a constructor:

```go
open, _ := civil.NewTimeFromString("09:00")
closing, _ := civil.NewTimeFromString("17:30")
day, _ := exchange.NewSession(open, closing)

code, _ := exchange.NewCode("XMIL")

milan, err := exchange.NewExchange(
    code, "Borsa Italiana", country.IT, "Europe/Rome",
    []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
    []exchange.Session{day},
)
```

Or from a CSV file in the format of the built-in list:

```csv
# my venues
code,name,country,time_zone,weekdays,sessions
XMIL,Borsa Italiana,IT,Europe/Rome,Mon Tue Wed Thu Fri,09:00-17:30
KRAKEN,Kraken,US,UTC,Sun Mon Tue Wed Thu Fri Sat,00:00-00:00
```

```go
exchanges, err := exchange.NewExchangesFromCSV(file)
```

An error names the line it came from.

### JSON and SQL

An exchange is written as its code and read back from the built-in list:

```go
type Listing struct {
    Exchange exchange.Exchange `json:"exchange"`
}

// {"exchange":"XNAS"}
```

`Scan` accepts a `string` or a `[]byte`. `NULL` is an error; scan a nullable column into a `sql.Null[exchange.Exchange]`.

A code alone does not give the time zone or the hours of an exchange outside the built-in list, so decoding one is an error. An application with its own exchanges stores and reads an `exchange.Code`, which encodes the same way and accepts any well-formed code, and looks the exchange up in the list it loaded.

## Built-in Exchanges

`exchange.Exchanges()` returns them all.

| Code | Name | Country | Time zone | Sessions |
|------|------|---------|-----------|----------|
| `XNYS` | New York Stock Exchange | US | America/New_York | 09:30-16:00 |
| `XNAS` | Nasdaq | US | America/New_York | 09:30-16:00 |
| `ARCX` | NYSE Arca | US | America/New_York | 09:30-16:00 |
| `XASE` | NYSE American | US | America/New_York | 09:30-16:00 |
| `BATS` | Cboe BZX U.S. Equities Exchange | US | America/New_York | 09:30-16:00 |
| `XCBO` | Cboe Options Exchange | US | America/Chicago | 08:30-15:00 |
| `XCBF` | Cboe Futures Exchange | US | America/Chicago | 08:30-15:15 |
| `XTSE` | Toronto Stock Exchange | CA | America/Toronto | 09:30-16:00 |
| `XLON` | London Stock Exchange | GB | Europe/London | 08:00-16:30 |
| `XPAR` | Euronext Paris | FR | Europe/Paris | 09:00-17:30 |
| `XAMS` | Euronext Amsterdam | NL | Europe/Amsterdam | 09:00-17:30 |
| `XETR` | Xetra | DE | Europe/Berlin | 09:00-17:30 |
| `XSWX` | SIX Swiss Exchange | CH | Europe/Zurich | 09:00-17:30 |
| `XTKS` | Tokyo Stock Exchange | JP | Asia/Tokyo | 09:00-11:30, 12:30-15:30 |
| `XHKG` | Hong Kong Exchanges and Clearing | HK | Asia/Hong_Kong | 09:30-12:00, 13:00-16:00 |
| `XSHG` | Shanghai Stock Exchange | CN | Asia/Shanghai | 09:30-11:30, 13:00-15:00 |
| `XKRX` | Korea Exchange | KR | Asia/Seoul | 09:00-15:30 |
| `XBOM` | BSE | IN | Asia/Kolkata | 09:15-15:30 |
| `XSES` | Singapore Exchange | SG | Asia/Singapore | 09:00-17:00 |
| `XASX` | ASX | AU | Australia/Sydney | 10:00-16:00 |
| `BINANCE` | Binance | US | UTC | always open |
| `COINBASE` | Coinbase | US | UTC | always open |

All but the last two trade Monday to Friday.

The list is [`exchanges.csv`](./exchanges.csv), and its header names the source of every value: codes, names and countries from the ISO 10383 market identifier list, time zones and hours from the [exchange_calendars](https://github.com/gerrymanoim/exchange_calendars) project, and the options and crypto hours from the [LEAN](https://github.com/QuantConnect/Lean) market-hours database.

## What It Does Not Hold

- **Dated facts.** Holidays, early closes and late opens belong to a calendar.
- **Hours that differ by product.** Some index options on Cboe close at 15:15, not 15:00. That is a fact about the instrument.
- **Sessions that cross midnight.** A futures session that opens the evening before cannot be written yet, so the CME exchanges are not in the list.
- **Extended hours.** Only regular sessions are held; pre-market and post-market are not.

## Zero Values

An exchange is built by a constructor, but Go still lets anyone write `exchange.Exchange{}`. That value is not an exchange. It stands for a missing one: `IsValid` returns false, and other methods are meaningless on it.

`exchange.Session{}` is the whole day, a real session.

## Testing

```bash
go test -race -cover ./...
```

## License

MIT License. See [LICENSE](./LICENSE).
