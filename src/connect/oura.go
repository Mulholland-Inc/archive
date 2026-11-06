package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "oura", Name: "Oura", Env: "ARCHIVE_OURA",
		Info: Info{By: "Oura", Description: "Read your sleep, readiness, activity, heart rate and workouts from your Oura ring", Category: CatHealth, Icon: IconHeart,
			Scopes: []string{"Read your sleep, readiness, activity, blood oxygen, heart rate and workouts, and your email"}},
		API: "https://api.ouraring.com/v2/usercollection",
		Endpoint: oauth2.Endpoint{AuthURL: "https://cloud.ouraring.com/oauth/authorize", TokenURL: "https://api.ouraring.com/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{URL: "https://api.ouraring.com/oauth/revoke", Query: Q{"access_token": "{access}"}},
		Scopes:   []string{"email", "personal", "daily", "heartrate", "workout", "spo2"},
		Who:      &Who{Req: Req{URL: "/personal_info"}, Names: []Path{"email", "id"}, Lower: true},
		Actions: map[string]*Action{
			"get": Call[ouraGet, any](Plan{
				Doc: "Read any of Oura's v2 usercollection lists, like { path: \"daily_readiness\", start: \"2026-10-01\", end: \"2026-10-31\" }: " +
					"{ data, next_token }. Lists are daily_sleep, sleep, daily_readiness, daily_activity, daily_spo2, daily_stress, daily_resilience, " +
					"daily_cardiovascular_age, vo2_max, workout, session, sleep_time, rest_mode_period and enhanced_tag by day, and heartrate by " +
					"start and end times, at most 30 days. The last week when start is left out",
				Steps: []Step{
					{Only: Test{At: "path", Re: `^[a-z0-9_]+$`, Off: true}, Fail: "path is one of Oura's lists, like daily_sleep or heartrate"},
					{Only: Test{At: "path", Is: "heartrate"}, Req: Req{URL: "/heartrate",
						Query: Q{"start_datetime": "{start|}", "end_datetime": "{end|}", "next_token": "{next|}"}}},
					{Only: Test{At: "path", Not: "heartrate"}, Req: Req{URL: "/{path}",
						Query: Q{"start_date": "{start|}", "end_date": "{end|}", "next_token": "{next|}"}}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "readiness", Gap: time.Second, Keep: true, Parts: ouraMonths, Req: ouraList("daily_readiness"), Items: "data", Page: ouraPage,
				Doc: "Each day's readiness score, temperature deviation and the contributors to the score" + ouraDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'readiness'", "on_date": "archive.sync_day(data->>'day')", "score": "archive.sync_num(data->>'score')",
						"temperature": "archive.sync_num(data->>'temperature_deviation')", "detail": "data->'contributors'"}}},
			{Name: "daily_sleep", Gap: time.Second, Keep: true, Parts: ouraMonths, Req: ouraList("daily_sleep"), Items: "data", Page: ouraPage,
				Doc: "Each day's sleep score and its contributors" + ouraDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'sleep'", "on_date": "archive.sync_day(data->>'day')", "score": "archive.sync_num(data->>'score')", "detail": "data->'contributors'"}}},
			{Name: "activity", Gap: time.Second, Keep: true, Parts: ouraMonths, Req: ouraList("daily_activity"), Items: "data", Page: ouraPage,
				Doc: "Each day's activity score, steps, calories, walking distance and minutes of activity" + ouraDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'activity'", "on_date": "archive.sync_day(data->>'day')", "score": "archive.sync_num(data->>'score')",
						"steps": "archive.sync_num(data->>'steps')", "active_calories": "archive.sync_num(data->>'active_calories')",
						"calories": "archive.sync_num(data->>'total_calories')", "distance": "archive.sync_num(data->>'equivalent_walking_distance')",
						"active_minutes": "round((archive.sync_num(data->>'high_activity_time') + archive.sync_num(data->>'medium_activity_time')) / 60)",
						"detail":         "data->'contributors'"}}},
			{Name: "spo2", Gap: time.Second, Keep: true, Parts: ouraMonths, Req: ouraList("daily_spo2"), Items: "data", Page: ouraPage,
				Doc: "Each night's average blood oxygen and breathing disturbance index, for Gen3 rings and later" + ouraDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'spo2'", "on_date": "archive.sync_day(data->>'day')", "spo2": "archive.sync_num(data#>>'{spo2_percentage,average}')",
						"detail": "jsonb_build_object('breathing_disturbance_index', data->'breathing_disturbance_index')"}}},
			{Name: "sleep", Gap: time.Second, Keep: true, Parts: ouraMonths, Req: ouraList("sleep"), Items: "data", Page: ouraPage,
				Skip: []Test{{At: "type", Is: "deleted"}},
				Doc:  "Each sleep, nights and naps, with its stages, efficiency, heart rate, HRV and breathing" + ouraDoc,
				To: Map{Table: "health_sleep",
					Cols: map[string]SQL{"on_date": "archive.sync_day(data->>'day')", "starts_at": "archive.sync_at(data->>'bedtime_start')", "ends_at": "archive.sync_at(data->>'bedtime_end')",
						"asleep_minutes": "round(archive.sync_num(data->>'total_sleep_duration') / 60)", "deep_minutes": "round(archive.sync_num(data->>'deep_sleep_duration') / 60)",
						"rem_minutes": "round(archive.sync_num(data->>'rem_sleep_duration') / 60)", "light_minutes": "round(archive.sync_num(data->>'light_sleep_duration') / 60)",
						"awake_minutes": "round(archive.sync_num(data->>'awake_time') / 60)", "efficiency": "archive.sync_num(data->>'efficiency')",
						"score": "archive.sync_num(data#>>'{readiness,score}')", "heart_rate": "archive.sync_num(data->>'average_heart_rate')",
						"lowest_heart_rate": "archive.sync_num(data->>'lowest_heart_rate')", "hrv": "archive.sync_num(data->>'average_hrv')",
						"respiratory_rate": "archive.sync_num(data->>'average_breath')", "nap": "data->>'type' IN ('late_nap', 'rest')", "device": "'Oura ring'"}}},
			{Name: "workouts", Gap: time.Second, Keep: true, Parts: ouraMonths, Req: ouraList("workout"), Items: "data", Page: ouraPage,
				Doc: "Each workout, detected or logged, with its activity, calories and distance" + ouraDoc,
				To: Map{Table: "health_workout",
					Cols: map[string]SQL{"sport": "lower(replace(coalesce(nullif(data->>'label', ''), data->>'activity'), '_', ' '))",
						"starts_at": "archive.sync_at(data->>'start_datetime')", "ends_at": "archive.sync_at(data->>'end_datetime')",
						"minutes":  "round(extract(epoch FROM archive.sync_at(data->>'end_datetime') - archive.sync_at(data->>'start_datetime')) / 60)",
						"calories": "archive.sync_num(data->>'calories')", "distance": "archive.sync_num(data->>'distance')", "device": "'Oura ring'"}}},
			{Name: "heart_rate", Gap: time.Second, Keep: true, Items: "data", Page: ouraPage, Key: []Reads{{Of: "{timestamp}/{source}"}},
				Parts: &Parts{Span: &Span{First: Shift{-1, 0, 1}, Then: Shift{0, 0, -3}, Step: Shift{0, 0, 30}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}},
				Req:   Req{URL: "/heartrate", Query: Q{"start_datetime": "{in/start}T00:00:00Z", "end_datetime": "{in/end}T23:59:59Z", "next_token": "{page}"}},
				Doc: "Your heart rate through each day and night, every few minutes, by <time>/<source>: a year back the first time, 30 days a call, " +
					"then the last 3 days each sync.",
				To: Map{Table: "health_sample",
					Cols: map[string]SQL{"kind": "'heart_rate'", "value": "archive.sync_num(data->>'bpm')", "unit": "'count/min'",
						"starts_at": "archive.sync_at(data->>'timestamp')", "ends_at": "archive.sync_at(data->>'timestamp')", "device": "'Oura ring'"}}},
		},
	})
}

const ouraDoc = ", by its id: three years back the first time, a month a call, then the last 28 days each sync, as Oura revises recent days."

var (
	ouraMonths = &Parts{Span: &Span{First: Shift{-3, 0, 1}, Then: Shift{0, 0, -28}, Step: Shift{0, 1, 0}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}}
	ouraPage   = Cursor{At: "next_token"}
)

func ouraList(path string) Req {
	return Req{URL: "/" + path, Query: Q{"start_date": "{in/start}", "end_date": "{in/end}", "next_token": "{page}"}}
}

type ouraGet struct {
	Path  string `json:"path" jsonschema:"The list, like daily_sleep, sleep, daily_readiness, daily_activity, workout or heartrate"`
	Start string `json:"start,omitempty" jsonschema:"The first day, like 2026-10-01; for heartrate a time, like 2026-10-01T00:00:00Z"`
	End   string `json:"end,omitempty" jsonschema:"The last day, or for heartrate the last time"`
	Next  string `json:"next,omitempty" jsonschema:"The next_token of the call before, for its next page"`
}
