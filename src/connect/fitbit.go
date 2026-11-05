package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Google turned the Fitbit Web API off on November 2, 2026; Fitbit's data is read through the Google Health API.
func init() {
	Register(&Connector{
		ID: "fitbit", Name: "Fitbit", Env: "ARCHIVE_GOOGLE",
		Info: Info{By: "Google", Description: "Read your sleep, workouts, heart rate, HRV, blood oxygen and weight from Fitbit and Pixel Watch", Category: CatHealth, Icon: IconHeart,
			Scopes: []string{"Read your sleep, exercise, heart and body measurements from Google Health", "Read your email address"}},
		API: "https://health.googleapis.com/v4/users/me",
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://oauth2.googleapis.com/revoke",
		Scopes: []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/googlehealth.sleep.readonly",
			"https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly", "https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly"},
		Params: map[string]string{"access_type": "offline", "prompt": "consent"},
		Busy:   []string{"Quota exceeded", "rateLimitExceeded", "RESOURCE_EXHAUSTED"},
		Who: &Who{Req: Req{URL: "https://openidconnect.googleapis.com/v1/userinfo"}, Names: []Path{"email"}, Lower: true,
			Check: []Step{{Req: Req{URL: "/identity"}}}},
		Actions: map[string]*Action{
			"get": Call[fitbitGet, any](Plan{
				Doc: "List the data points of one Google Health data type, newest first, like { type: \"heart-rate\", filter: " +
					"\"heart_rate.sample_time.physical_time >= \\\"2026-10-01T00:00:00Z\\\"\" }: { dataPoints, nextPageToken }. Types include steps, " +
					"heart-rate, heart-rate-variability, sleep, exercise, daily-resting-heart-rate, daily-heart-rate-variability, daily-oxygen-saturation, " +
					"daily-respiratory-rate, weight, body-fat, distance, floors, active-zone-minutes and vo2-max. A filter names the type in snake_case: " +
					"<type>.interval.start_time or civil_start_time for intervals, <type>.sample_time.physical_time for samples, <type>.date for daily " +
					"types, with >= and < joined by AND",
				Clamp: map[string][2]int{"limit": {100, 10000}},
				Steps: []Step{
					{Only: Test{At: "type", Re: `^[a-z0-9-]+$`, Off: true}, Fail: "type is a data type in kebab-case, like heart-rate"},
					{Req: Req{URL: "/dataTypes/{type}/dataPoints", Query: Q{"filter": "{filter}", "pageSize": "{limit}", "pageToken": "{next}"}}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "sleep", Gap: time.Second, Keep: true, Parts: fitbitMonths, Items: "dataPoints", Page: fitbitPage, Key: fitbitKey("sleep", "interval/startTime"),
				Req: fitbitList("sleep", "pageSize", "25", "filter", `sleep.interval.end_time >= "{in/start}T00:00:00Z" AND sleep.interval.end_time < "{in/after}T00:00:00Z"`),
				Doc: "Each sleep, nights and naps, with minutes in each stage" + fitbitDoc,
				To: Map{Table: "health_sleep",
					Cols: map[string]SQL{"on_date": "archive.sync_day(data#>>'{sleep,interval,civilEndTime,date,year}' || '-' || lpad(data#>>'{sleep,interval,civilEndTime,date,month}', 2, '0') || '-' || lpad(data#>>'{sleep,interval,civilEndTime,date,day}', 2, '0'))",
						"starts_at": "archive.sync_at(data#>>'{sleep,interval,startTime}')", "ends_at": "archive.sync_at(data#>>'{sleep,interval,endTime}')",
						"asleep_minutes": "archive.sync_num(data#>>'{sleep,summary,minutesAsleep}')", "awake_minutes": "archive.sync_num(data#>>'{sleep,summary,minutesAwake}')",
						"deep_minutes": fitbitStage("DEEP"), "rem_minutes": fitbitStage("REM"), "light_minutes": fitbitStage("LIGHT"),
						"efficiency": "round(100 * archive.sync_num(data#>>'{sleep,summary,minutesAsleep}') / nullif(archive.sync_num(data#>>'{sleep,summary,minutesInSleepPeriod}'), 0))",
						"nap":        "archive.sync_bool(data#>'{sleep,metadata,nap}')", "device": "'Fitbit'"}}},
			{Name: "exercise", Gap: time.Second, Keep: true, Parts: fitbitMonths, Items: "dataPoints", Page: fitbitPage, Key: fitbitKey("exercise", "interval/startTime"),
				Req: fitbitList("exercise", "pageSize", "25", "filter", `exercise.interval.civil_start_time >= "{in/start}" AND exercise.interval.civil_start_time < "{in/after}"`),
				Doc: "Each exercise, tracked or logged, with its calories, distance, steps and heart rate" + fitbitDoc,
				To: Map{Table: "health_workout",
					Cols: map[string]SQL{"sport": "lower(coalesce(nullif(data#>>'{exercise,displayName}', ''), replace(data#>>'{exercise,exerciseType}', '_', ' ')))",
						"starts_at": "archive.sync_at(data#>>'{exercise,interval,startTime}')", "ends_at": "archive.sync_at(data#>>'{exercise,interval,endTime}')",
						"minutes":  "coalesce(archive.sync_num(rtrim(data#>>'{exercise,activeDuration}', 's')) / 60, extract(epoch FROM archive.sync_at(data#>>'{exercise,interval,endTime}') - archive.sync_at(data#>>'{exercise,interval,startTime}')) / 60)",
						"calories": "archive.sync_num(data#>>'{exercise,metricsSummary,caloriesKcal}')", "distance": "archive.sync_num(data#>>'{exercise,metricsSummary,distanceMillimeters}') / 1000",
						"steps": "archive.sync_num(data#>>'{exercise,metricsSummary,steps}')", "heart_rate": "archive.sync_num(data#>>'{exercise,metricsSummary,averageHeartRateBeatsPerMinute}')",
						"device": "'Fitbit'"}}},
			{Name: "resting_heart_rate", Gap: time.Second, Keep: true, Parts: fitbitMonths, Items: "dataPoints", Page: fitbitPage, Key: fitbitDay("dailyRestingHeartRate"),
				Req: fitbitDaily("daily-resting-heart-rate", "daily_resting_heart_rate"),
				Doc: "Each day's resting heart rate" + fitbitDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'heart'", "on_date": fitbitDate("dailyRestingHeartRate"), "resting_heart_rate": "archive.sync_num(data#>>'{dailyRestingHeartRate,beatsPerMinute}')"}}},
			{Name: "hrv", Gap: time.Second, Keep: true, Parts: fitbitMonths, Items: "dataPoints", Page: fitbitPage, Key: fitbitDay("dailyHeartRateVariability"),
				Req: fitbitDaily("daily-heart-rate-variability", "daily_heart_rate_variability"),
				Doc: "Each night's heart rate variability (RMSSD), with deep sleep's and the non-REM heart rate" + fitbitDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'hrv'", "on_date": fitbitDate("dailyHeartRateVariability"), "hrv": "archive.sync_num(data#>>'{dailyHeartRateVariability,averageHeartRateVariabilityMilliseconds}')",
						"detail": "(data->'dailyHeartRateVariability') - 'date'"}}},
			{Name: "spo2", Gap: time.Second, Keep: true, Parts: fitbitMonths, Items: "dataPoints", Page: fitbitPage, Key: fitbitDay("dailyOxygenSaturation"),
				Req: fitbitDaily("daily-oxygen-saturation", "daily_oxygen_saturation"),
				Doc: "Each night's average blood oxygen" + fitbitDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'spo2'", "on_date": fitbitDate("dailyOxygenSaturation"), "spo2": "archive.sync_num(data#>>'{dailyOxygenSaturation,averagePercentage}')",
						"detail": "(data->'dailyOxygenSaturation') - 'date'"}}},
			{Name: "respiratory_rate", Gap: time.Second, Keep: true, Parts: fitbitMonths, Items: "dataPoints", Page: fitbitPage, Key: fitbitDay("dailyRespiratoryRate"),
				Req: fitbitDaily("daily-respiratory-rate", "daily_respiratory_rate"),
				Doc: "Each night's breathing rate" + fitbitDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'breathing'", "on_date": fitbitDate("dailyRespiratoryRate"),
						"respiratory_rate": "archive.sync_num(coalesce(data#>>'{dailyRespiratoryRate,breathsPerMinute}', data#>>'{dailyRespiratoryRate,averageBreathsPerMinute}'))",
						"detail":           "(data->'dailyRespiratoryRate') - 'date'"}}},
			{Name: "weight", Gap: time.Second, Keep: true, Parts: fitbitMonths, Items: "dataPoints", Page: fitbitPage, Key: fitbitKey("weight", "sampleTime/physicalTime"),
				Req: fitbitList("weight", "pageSize", "1000", "filter", `weight.sample_time.civil_time >= "{in/start}" AND weight.sample_time.civil_time < "{in/after}"`),
				Doc: "Each weight you logged or a scale sent" + fitbitDoc,
				To: Map{Table: "health_sample",
					Cols: map[string]SQL{"kind": "'weight'", "value": "archive.sync_num(data#>>'{weight,weightGrams}') / 1000", "unit": "'kg'",
						"starts_at": "archive.sync_at(data#>>'{weight,sampleTime,physicalTime}')", "ends_at": "archive.sync_at(data#>>'{weight,sampleTime,physicalTime}')",
						"device": "data#>>'{dataSource,device,displayName}'"}}},
		},
	})
}

const fitbitDoc = ": three years back the first time, a month a call, then the last 28 days each sync, as devices sync late."

var (
	fitbitMonths = &Parts{Span: &Span{First: Shift{-3, 0, 1}, Then: Shift{0, 0, -28}, Step: Shift{0, 1, 0}}, ID: "{start}",
		Vals: Obj{"start": "{start}", "end": "{end}", "after": "{after}"}}
	fitbitPage = Cursor{At: "nextPageToken"}
)

func fitbitList(kind string, q ...string) Req {
	query := Q{"pageToken": "{page}"}
	for i := 0; i+1 < len(q); i += 2 {
		query[q[i]] = q[i+1]
	}
	return Req{URL: "/dataTypes/" + kind + "/dataPoints", Query: query}
}

func fitbitDaily(kind, field string) Req {
	return fitbitList(kind, "pageSize", "1000", "filter", field+`.date >= "{in/start}" AND `+field+`.date < "{in/after}"`)
}

func fitbitKey(field, at string) []Reads {
	return []Reads{{Of: "{name}"}, {Of: "{" + field + "/" + at + "}"}}
}

func fitbitDay(field string) []Reads {
	return []Reads{{Of: "{" + field + "/date/year}-{" + field + "/date/month}-{" + field + "/date/day}"}}
}

func fitbitDate(field string) SQL {
	d := "data#>>'{" + field + ",date,"
	return SQL("archive.sync_day(" + d + "year}' || '-' || lpad(" + d + "month}', 2, '0') || '-' || lpad(" + d + "day}', 2, '0'))")
}

func fitbitStage(stage string) SQL {
	return SQL("archive.sync_num((SELECT s->>'minutes' FROM archive.sync_list(data#>'{sleep,summary,stagesSummary}') s WHERE s->>'type' = '" + stage + "' LIMIT 1))")
}

type fitbitGet struct {
	Type   string `json:"type" jsonschema:"The data type in kebab-case, like heart-rate, sleep, exercise, steps or daily-resting-heart-rate"`
	Filter string `json:"filter,omitempty" jsonschema:"A filter on time, like steps.interval.civil_start_time >= \"2026-10-01\" AND steps.interval.civil_start_time < \"2026-10-02\""`
	Limit  int    `json:"limit,omitempty" jsonschema:"At most this many a page, up to 10000, and 25 for sleep and exercise; 100 when left out"`
	Next   string `json:"next,omitempty" jsonschema:"The nextPageToken of the call before"`
}
