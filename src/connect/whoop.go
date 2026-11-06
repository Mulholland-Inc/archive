package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "whoop", Name: "WHOOP", Env: "ARCHIVE_WHOOP",
		Info: Info{By: "WHOOP", Description: "Read your recovery, sleep, strain and workouts from your WHOOP", Category: CatHealth, Icon: IconHeart,
			Scopes: []string{"Read your recovery, sleep, strain, workouts, profile and body measurements"}},
		API: "https://api.prod.whoop.com/developer",
		Endpoint: oauth2.Endpoint{AuthURL: "https://api.prod.whoop.com/oauth/oauth2/auth", TokenURL: "https://api.prod.whoop.com/oauth/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: DELETE, URL: "https://api.prod.whoop.com/developer/v2/user/access", Header: map[string][]string{"Authorization": {"Bearer {access}"}}},
		Scopes:   []string{"offline", "read:recovery", "read:cycles", "read:sleep", "read:workout", "read:profile", "read:body_measurement"},
		Who:      &Who{Req: Req{URL: "/v2/user/profile/basic"}, Names: []Path{"email", "user_id"}, Lower: true},
		Actions: map[string]*Action{
			"get": Call[whoopGet, any](Plan{
				Doc: "Read any of WHOOP's v2 lists or records, like { path: \"/v2/recovery\", start: \"2026-10-01T00:00:00Z\" }, " +
					"{ path: \"/v2/cycle/<id>/recovery\" } or { path: \"/v2/user/measurement/body\" }: a list is { records, next_token }, " +
					"25 a page. Lists are /v2/recovery, /v2/cycle, /v2/activity/sleep and /v2/activity/workout",
				Steps: []Step{
					{Only: Test{At: "path", Re: `^/v2/[A-Za-z0-9_/-]+$`, Off: true}, Fail: "path is an endpoint after /developer, like /v2/recovery"},
					{Req: Req{URL: "{+path}", Query: Q{"start": "{start}", "end": "{end}", "nextToken": "{next}", "limit": "25"}}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "recovery", Gap: time.Second, Keep: true, Parts: whoopMonths, Req: whoopList("/v2/recovery"), Items: "records", Page: whoopPage,
				Key: []Reads{{Of: "{cycle_id}"}}, Skip: whoopUnscored,
				Doc: "Each day's recovery: the score, resting heart rate, HRV, blood oxygen and skin temperature, by its cycle" + whoopDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'recovery'", "on_date": "(archive.sync_at(data->>'created_at') AT TIME ZONE 'UTC')::date",
						"score": "archive.sync_num(data#>>'{score,recovery_score}')", "resting_heart_rate": "archive.sync_num(data#>>'{score,resting_heart_rate}')",
						"hrv": "archive.sync_num(data#>>'{score,hrv_rmssd_milli}')", "spo2": "archive.sync_num(data#>>'{score,spo2_percentage}')",
						"detail": "jsonb_build_object('skin_temp_celsius', data#>'{score,skin_temp_celsius}', 'user_calibrating', data#>'{score,user_calibrating}', 'sleep_id', data->'sleep_id')"}}},
			{Name: "strain", Gap: time.Second, Keep: true, Parts: whoopMonths, Req: whoopList("/v2/cycle"), Items: "records", Page: whoopPage, Skip: whoopUnscored,
				Doc: "Each physiological cycle, waking to waking, with its day strain, calories and heart rate, by its id" + whoopDoc,
				To: Map{Table: "health_day",
					Cols: map[string]SQL{"kind": "'strain'", "on_date": whoopLocal + "::date", "score": "archive.sync_num(data#>>'{score,strain}')",
						"calories": "round(archive.sync_num(data#>>'{score,kilojoule}') / 4.184)",
						"detail":   "jsonb_build_object('average_heart_rate', data#>'{score,average_heart_rate}', 'max_heart_rate', data#>'{score,max_heart_rate}', 'start', data->'start', 'end', data->'end')"}}},
			{Name: "sleep", Gap: time.Second, Keep: true, Parts: whoopMonths, Req: whoopList("/v2/activity/sleep"), Items: "records", Page: whoopPage, Skip: whoopUnscored,
				Doc: "Each sleep, nights and naps, with its stages, performance, efficiency and breathing, by its id" + whoopDoc,
				To: Map{Table: "health_sleep",
					Cols: map[string]SQL{"on_date": "(archive.sync_at(data->>'end') AT TIME ZONE 'UTC' + " + whoopOffset + ")::date",
						"starts_at": "archive.sync_at(data->>'start')", "ends_at": "archive.sync_at(data->>'end')",
						"asleep_minutes": "round((archive.sync_num(data#>>'{score,stage_summary,total_light_sleep_time_milli}') + archive.sync_num(data#>>'{score,stage_summary,total_slow_wave_sleep_time_milli}') + " +
							"archive.sync_num(data#>>'{score,stage_summary,total_rem_sleep_time_milli}')) / 60000)",
						"deep_minutes":     "round(archive.sync_num(data#>>'{score,stage_summary,total_slow_wave_sleep_time_milli}') / 60000)",
						"rem_minutes":      "round(archive.sync_num(data#>>'{score,stage_summary,total_rem_sleep_time_milli}') / 60000)",
						"light_minutes":    "round(archive.sync_num(data#>>'{score,stage_summary,total_light_sleep_time_milli}') / 60000)",
						"awake_minutes":    "round(archive.sync_num(data#>>'{score,stage_summary,total_awake_time_milli}') / 60000)",
						"efficiency":       "archive.sync_num(data#>>'{score,sleep_efficiency_percentage}')",
						"score":            "archive.sync_num(data#>>'{score,sleep_performance_percentage}')",
						"respiratory_rate": "archive.sync_num(data#>>'{score,respiratory_rate}')", "nap": "archive.sync_bool(data->'nap')", "device": "'WHOOP'"}}},
			{Name: "workouts", Gap: time.Second, Keep: true, Parts: whoopMonths, Req: whoopList("/v2/activity/workout"), Items: "records", Page: whoopPage, Skip: whoopUnscored,
				Doc: "Each workout with its sport, strain, heart rate, calories and distance, by its id" + whoopDoc,
				To: Map{Table: "health_workout",
					Cols: map[string]SQL{"sport": "lower(replace(data->>'sport_name', '-', ' '))", "starts_at": "archive.sync_at(data->>'start')", "ends_at": "archive.sync_at(data->>'end')",
						"minutes":  "round(extract(epoch FROM archive.sync_at(data->>'end') - archive.sync_at(data->>'start')) / 60)",
						"calories": "round(archive.sync_num(data#>>'{score,kilojoule}') / 4.184)", "distance": "archive.sync_num(data#>>'{score,distance_meter}')",
						"heart_rate": "archive.sync_num(data#>>'{score,average_heart_rate}')", "max_heart_rate": "archive.sync_num(data#>>'{score,max_heart_rate}')",
						"strain": "archive.sync_num(data#>>'{score,strain}')", "device": "'WHOOP'"}}},
		},
	})
}

const (
	whoopDoc        = ": three years back the first time, a month at a time, 25 a page, then the last 28 days each sync, as WHOOP scores recent days late. Those not scored yet wait for a later sync."
	whoopOffset SQL = "CASE WHEN data->>'timezone_offset' ~ '^[+-]\\d{2}:\\d{2}$' THEN (data->>'timezone_offset')::interval ELSE interval '0' END"
	whoopLocal  SQL = "(archive.sync_at(data->>'start') AT TIME ZONE 'UTC' + " + whoopOffset + ")"
)

var (
	whoopMonths   = &Parts{Span: &Span{First: Shift{-3, 0, 1}, Then: Shift{0, 0, -28}, Step: Shift{0, 1, 0}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}}
	whoopPage     = Cursor{At: "next_token"}
	whoopUnscored = []Test{{At: "score_state", Not: "SCORED"}}
)

func whoopList(path string) Req {
	return Req{URL: path, Query: Q{"start": "{in/start}T00:00:00.000Z", "end": "{in/end}T23:59:59.999Z", "limit": "25", "nextToken": "{page}"}}
}

type whoopGet struct {
	Path  string `json:"path" jsonschema:"The endpoint after /developer, like /v2/recovery, /v2/cycle, /v2/activity/sleep, /v2/activity/workout, /v2/user/profile/basic or /v2/cycle/<id>/sleep"`
	Start string `json:"start,omitempty" jsonschema:"For a list, the earliest time, like 2026-10-01T00:00:00Z"`
	End   string `json:"end,omitempty" jsonschema:"For a list, the latest time; now when left out"`
	Next  string `json:"next,omitempty" jsonschema:"The next_token of the call before, for its next page"`
}
