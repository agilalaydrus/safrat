package repository

import (
	"context"
	"time"
)

// FunnelReport is everything one operator's funnel screen needs.
type FunnelReport struct {
	Steps       []FunnelStepCount
	Sources     []FunnelSourceCount
	Hours       []FunnelHourCount
	Places      []FunnelPlaceCount
	Articles    []FunnelArticleCount
	ChannelAges []FunnelChannelAge
	Daily       []FunnelDayCount
}

type FunnelDayCount struct {
	Day           string
	Visitors      int32
	Registrations int32
}

type FunnelStepCount struct {
	Step     string
	Visitors int32
	Events   int32
}

type FunnelSourceCount struct {
	Source        string
	Visitors      int32
	Registrations int32
}

type FunnelHourCount struct {
	Hour     int32
	Visitors int32
}

type FunnelPlaceCount struct {
	Province string
	City     string
	Visitors int32
}

type FunnelArticleCount struct {
	Slug          string
	Readers       int32
	Registrations int32
}

type FunnelChannelAge struct {
	Source     string
	AverageAge float64
	Sample     int32
}

// Report reads one operator's funnel.
//
// operatorID is required and never optional: an empty one would silently return
// the platform's own rows to a travel agency. The scoping lives here rather than
// in a handler for the same reason branch isolation does — a caller that forgets
// to pass it must get nothing, not everything.
func (r *FunnelRepository) Report(ctx context.Context, operatorID string, days int32) (FunnelReport, error) {
	report := FunnelReport{}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return report, err
	}
	if days <= 0 || days > 365 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -int(days))

	// Steps and sources come from the daily summaries, never from raw rows: a
	// screen that recomputes millions of events is a screen that stops opening.
	steps, err := r.pool.Query(ctx, `
		SELECT step, SUM(visitors)::int, SUM(events)::int
		FROM funnel_daily
		WHERE operator_id = $1 AND day >= $2::date
		GROUP BY step`, operator, since)
	if err != nil {
		return report, err
	}
	defer steps.Close()
	for steps.Next() {
		var row FunnelStepCount
		if err := steps.Scan(&row.Step, &row.Visitors, &row.Events); err != nil {
			return report, err
		}
		report.Steps = append(report.Steps, row)
	}
	if err := steps.Err(); err != nil {
		return report, err
	}

	// Sources carry both halves so the screen can order by the half that
	// matters. Registrations come from the registration rows rather than from
	// SELESAI events, because attribution on the row survives the visitor token
	// resetting at midnight.
	sources, err := r.pool.Query(ctx, `
		WITH visits AS (
		  SELECT COALESCE(NULLIF(utm_source, ''), 'langsung') AS source, SUM(visitors)::int AS visitors
		  FROM funnel_daily
		  WHERE operator_id = $1 AND day >= $2::date AND step = 'LANDING'
		  GROUP BY 1
		), signups AS (
		  SELECT COALESCE(NULLIF(utm_source, ''), 'langsung') AS source, COUNT(*)::int AS registrations
		  FROM pilgrim_registrations
		  WHERE operator_id = $1 AND created_at >= $2
		  GROUP BY 1
		)
		SELECT COALESCE(v.source, s.source), COALESCE(v.visitors, 0), COALESCE(s.registrations, 0)
		FROM visits v
		FULL OUTER JOIN signups s ON s.source = v.source
		ORDER BY COALESCE(s.registrations, 0) DESC, COALESCE(v.visitors, 0) DESC`, operator, since)
	if err != nil {
		return report, err
	}
	defer sources.Close()
	for sources.Next() {
		var row FunnelSourceCount
		if err := sources.Scan(&row.Source, &row.Visitors, &row.Registrations); err != nil {
			return report, err
		}
		report.Sources = append(report.Sources, row)
	}
	if err := sources.Err(); err != nil {
		return report, err
	}

	// Hours, places and articles need the raw rows, so they only reach as far
	// back as the retention allows. The screen says so rather than presenting a
	// short window as if it were the whole period.
	hours, err := r.pool.Query(ctx, `
		SELECT EXTRACT(HOUR FROM occurred_at AT TIME ZONE 'Asia/Jakarta')::int AS hour,
		       COUNT(DISTINCT visitor_hash)::int
		FROM funnel_events
		WHERE operator_id = $1 AND occurred_at >= $2
		GROUP BY 1 ORDER BY 1`, operator, since)
	if err != nil {
		return report, err
	}
	defer hours.Close()
	for hours.Next() {
		var row FunnelHourCount
		if err := hours.Scan(&row.Hour, &row.Visitors); err != nil {
			return report, err
		}
		report.Hours = append(report.Hours, row)
	}
	if err := hours.Err(); err != nil {
		return report, err
	}

	places, err := r.pool.Query(ctx, `
		SELECT province, city, COUNT(DISTINCT visitor_hash)::int
		FROM funnel_events
		WHERE operator_id = $1 AND occurred_at >= $2 AND province <> ''
		GROUP BY 1, 2 ORDER BY 3 DESC LIMIT 20`, operator, since)
	if err != nil {
		return report, err
	}
	defer places.Close()
	for places.Next() {
		var row FunnelPlaceCount
		if err := places.Scan(&row.Province, &row.City, &row.Visitors); err != nil {
			return report, err
		}
		report.Places = append(report.Places, row)
	}
	if err := places.Err(); err != nil {
		return report, err
	}

	// An article's readers, and how many of those same visitors later finished
	// a registration. This is what makes content measurable rather than a
	// number nobody can act on.
	articles, err := r.pool.Query(ctx, `
		WITH readers AS (
		  SELECT article_slug, visitor_hash
		  FROM funnel_events
		  WHERE operator_id = $1 AND occurred_at >= $2 AND step = 'ARTIKEL' AND article_slug <> ''
		  GROUP BY 1, 2
		), finished AS (
		  SELECT DISTINCT visitor_hash FROM funnel_events
		  WHERE operator_id = $1 AND occurred_at >= $2 AND step = 'SELESAI'
		)
		SELECT r.article_slug, COUNT(*)::int,
		       COUNT(*) FILTER (WHERE f.visitor_hash IS NOT NULL)::int
		FROM readers r
		LEFT JOIN finished f ON f.visitor_hash = r.visitor_hash
		GROUP BY 1 ORDER BY 2 DESC LIMIT 20`, operator, since)
	if err != nil {
		return report, err
	}
	defer articles.Close()
	for articles.Next() {
		var row FunnelArticleCount
		if err := articles.Scan(&row.Slug, &row.Readers, &row.Registrations); err != nil {
			return report, err
		}
		report.Articles = append(report.Articles, row)
	}
	if err := articles.Err(); err != nil {
		return report, err
	}

	// Day by day, so a screen can show whether the trend is going anywhere.
	// Visits come from the summaries and registrations from the rows, joined on
	// the Jakarta calendar day rather than on UTC — a registration at 01:00 WIB
	// belongs to that morning, not to the night before.
	daily, err := r.pool.Query(ctx, `
		WITH visits AS (
		  SELECT day, SUM(visitors)::int AS visitors
		  FROM funnel_daily
		  WHERE operator_id = $1 AND day >= $2::date AND step = 'LANDING'
		  GROUP BY 1
		), signups AS (
		  SELECT (created_at AT TIME ZONE 'Asia/Jakarta')::date AS day, COUNT(*)::int AS registrations
		  FROM pilgrim_registrations
		  WHERE operator_id = $1 AND created_at >= $2
		  GROUP BY 1
		)
		SELECT to_char(COALESCE(v.day, s.day), 'YYYY-MM-DD'),
		       COALESCE(v.visitors, 0), COALESCE(s.registrations, 0)
		FROM visits v
		FULL OUTER JOIN signups s ON s.day = v.day
		ORDER BY 1`, operator, since)
	if err != nil {
		return report, err
	}
	defer daily.Close()
	for daily.Next() {
		var row FunnelDayCount
		if err := daily.Scan(&row.Day, &row.Visitors, &row.Registrations); err != nil {
			return report, err
		}
		report.Daily = append(report.Daily, row)
	}
	if err := daily.Err(); err != nil {
		return report, err
	}

	// Age of people who registered, per channel. Visitors have no age: nothing
	// knows it, and the demographic figures in analytics tools are guesses from
	// an advertising profile rather than measurements.
	ages, err := r.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(utm_source, ''), 'langsung') AS source,
		       AVG(EXTRACT(YEAR FROM AGE(date_of_birth)))::float8,
		       COUNT(*)::int
		FROM pilgrim_registrations
		WHERE operator_id = $1 AND created_at >= $2 AND date_of_birth IS NOT NULL
		GROUP BY 1 ORDER BY 3 DESC`, operator, since)
	if err != nil {
		return report, err
	}
	defer ages.Close()
	for ages.Next() {
		var row FunnelChannelAge
		if err := ages.Scan(&row.Source, &row.AverageAge, &row.Sample); err != nil {
			return report, err
		}
		report.ChannelAges = append(report.ChannelAges, row)
	}
	return report, ages.Err()
}
