use chrono::{DateTime, Days, NaiveDate, NaiveDateTime, Offset, TimeDelta, TimeZone};

pub fn parse(value: &str, now: DateTime<impl TimeZone>) -> Result<i64, String> {
    let value = value.trim();
    if value.is_empty() {
        return Ok(0);
    }
    if let Some(days) = value
        .strip_suffix('d')
        .filter(|days| !days.is_empty() && days.bytes().all(|digit| digit.is_ascii_digit()))
    {
        let days = days
            .parse::<i64>()
            .map_err(|_| format!("invalid --since {value:?}: days are out of range"))?;
        let date = now
            .naive_local()
            .checked_sub_days(Days::new(days as u64))
            .ok_or_else(|| format!("invalid --since {value:?}: date is out of range"))?;
        return local_time(&now.timezone(), date)
            .map(|at| at.timestamp_millis())
            .ok_or_else(|| format!("invalid --since {value:?}: date is out of range"));
    }
    if let Some(duration) = duration_ns(value) {
        if duration < 0 {
            return Err(format!(
                "invalid --since {value:?}: the duration must not be negative"
            ));
        }
        return now
            .checked_sub_signed(TimeDelta::nanoseconds(duration))
            .map(|at| at.timestamp_millis())
            .ok_or_else(|| format!("invalid --since {value:?}: date is out of range"));
    }
    if value.len() == 10
        && let Ok(date) = NaiveDate::parse_from_str(value, "%Y-%m-%d")
    {
        let midnight = date.and_hms_opt(0, 0, 0).expect("midnight is valid");
        if let Some(at) = local_time(&now.timezone(), midnight) {
            return Ok(at.timestamp_millis());
        }
    }
    if let Ok(at) = DateTime::parse_from_rfc3339(value) {
        return Ok(at.timestamp_millis());
    }
    Err(format!(
        "invalid --since {value:?}: use a duration such as 30d or 12h, or a date such as 2026-10-01"
    ))
}

fn local_time<Tz: TimeZone>(tz: &Tz, local: NaiveDateTime) -> Option<DateTime<Tz>> {
    // Resolve transitions as time.Date does. If applying the offset for this clock time
    // interpreted as UTC crosses a transition, use the offset at the candidate instant.
    let offset = |utc: NaiveDateTime| {
        TimeDelta::seconds(i64::from(
            tz.offset_from_utc_datetime(&utc).fix().local_minus_utc(),
        ))
    };
    let candidate = local.checked_sub_signed(offset(local))?;
    let resolved = local.checked_sub_signed(offset(candidate))?;
    Some(tz.from_utc_datetime(&resolved))
}

/// Parse a Go-style duration in signed nanoseconds.
pub fn duration_ns(mut value: &str) -> Option<i64> {
    let negative = value.starts_with('-');
    value = value.strip_prefix(['-', '+']).unwrap_or(value);
    if value == "0" {
        return Some(0);
    }
    if value.is_empty() {
        return None;
    }

    let mut total = 0u64;
    while !value.is_empty() {
        let (integer, rest) = digits(value);
        let (fraction, rest) = rest.strip_prefix('.').map_or(("", rest), digits);
        if integer.is_empty() && fraction.is_empty() {
            return None;
        }
        let (unit, rest) = rest.split_at(
            rest.find(|c: char| c.is_ascii_digit() || c == '.')
                .unwrap_or(rest.len()),
        );
        let multiplier = match unit {
            "ns" => 1,
            "us" | "µs" | "μs" => 1_000,
            "ms" => 1_000_000,
            "s" => 1_000_000_000,
            "m" => 60_000_000_000,
            "h" => 3_600_000_000_000,
            _ => return None,
        };
        let integer = if integer.is_empty() {
            0
        } else {
            integer.parse::<u64>().ok()?
        };
        let whole = integer.checked_mul(multiplier)?;
        let fractional = fraction_ns(fraction, multiplier);
        total = total.checked_add(whole)?.checked_add(fractional)?;
        value = rest;
    }
    if negative && total == 1 << 63 {
        return Some(i64::MIN);
    }
    let total = i64::try_from(total).ok()?;
    Some(if negative { -total } else { total })
}

fn digits(value: &str) -> (&str, &str) {
    let end = value.bytes().take_while(u8::is_ascii_digit).count();
    value.split_at(end)
}

fn fraction_ns(fraction: &str, multiplier: u64) -> u64 {
    let mut mantissa = 0;
    let mut scale = 1.0;
    for digit in fraction.bytes() {
        if mantissa > i64::MAX as u64 / 10 {
            break;
        }
        let next = mantissa * 10 + u64::from(digit - b'0');
        if next > 1 << 63 {
            break;
        }
        mantissa = next;
        scale *= 10.0;
    }
    (mantissa as f64 * (multiplier as f64 / scale)) as u64
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::{FixedOffset, Utc};

    #[test]
    fn parses_empty_duration_calendar_date_and_rfc3339() {
        let tz = FixedOffset::west_opt(8 * 3600).unwrap();
        let now = tz.with_ymd_and_hms(2026, 11, 3, 12, 0, 0).unwrap();
        let expected = |value: &str| {
            DateTime::parse_from_rfc3339(value)
                .unwrap()
                .timestamp_millis()
        };
        for (input, at) in [
            (" 2d ", "2026-11-01T12:00:00-08:00"),
            ("1h30m", "2026-11-03T10:30:00-08:00"),
            ("0", "2026-11-03T12:00:00-08:00"),
            ("2026-10-01", "2026-10-01T00:00:00-08:00"),
            ("2026-10-01T01:30:00.123+03:00", "2026-09-30T22:30:00.123Z"),
        ] {
            assert_eq!(parse(input, now), Ok(expected(at)), "{input}");
        }
        assert_eq!(parse(" ", now), Ok(0));
    }

    #[test]
    fn rejects_invalid_and_overflowing_values() {
        for value in [
            "-1s",
            "7days",
            "d",
            "9223372036854775808d",
            "1000000000d",
            "2026-02-31",
        ] {
            assert!(
                parse(value, Utc::now())
                    .unwrap_err()
                    .starts_with("invalid --since")
            );
        }
    }

    #[test]
    fn distinguishes_calendar_days_from_elapsed_hours_across_dst() {
        use chrono_tz::America::Los_Angeles;

        let now = Los_Angeles.with_ymd_and_hms(2026, 3, 9, 12, 0, 0).unwrap();
        let expected = |value: &str| {
            DateTime::parse_from_rfc3339(value)
                .unwrap()
                .timestamp_millis()
        };
        assert_eq!(parse("2d", now), Ok(expected("2026-03-07T12:00:00-08:00")));
        assert_eq!(parse("48h", now), Ok(expected("2026-03-07T11:00:00-08:00")));
        assert_eq!(
            parse("2026-03-08", now),
            Ok(expected("2026-03-08T00:00:00-08:00"))
        );

        // Go resolves the nonexistent 02:30 on transition day to 01:30 PST.
        let now = Los_Angeles.with_ymd_and_hms(2026, 3, 9, 2, 30, 0).unwrap();
        assert_eq!(parse("1d", now), Ok(expected("2026-03-08T01:30:00-08:00")));
    }

    #[test]
    fn handles_unicode_duration_units_precision_and_arithmetic_bounds() {
        for (input, expected) in [
            ("1µs2μs3us", Some(6_000)),
            ("+.5m1.s", Some(31_000_000_000)),
            ("-9223372036854775808ns", Some(i64::MIN)),
            ("9223372036854775807ns1ns", None),
            ("9223372036854775808ns", None),
            ("20000000000000000000h", None),
            ("1.123456789012345678901s", Some(1_123_456_789)),
            ("1🐌", None),
            (".s", None),
            ("1", None),
        ] {
            assert_eq!(duration_ns(input), expected, "{input}");
        }
    }
}
