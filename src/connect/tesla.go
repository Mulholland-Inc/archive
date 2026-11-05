package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "tesla", Name: "Tesla",
		Info: Info{By: "Tesla", Description: "Read your Teslas, their battery, range and mileage, and your Supercharging sessions", Category: CatHome, Icon: IconList,
			Scopes: []string{"Read your profile and your vehicles", "Read your vehicles’ battery, range, mileage and software when they are awake", "Read your charging history and invoices"}},
		API:  "https://fleet-api.prd.na.vn.cloud.tesla.com",
		Pace: 500 * time.Millisecond,
		Endpoint: oauth2.Endpoint{AuthURL: "https://auth.tesla.com/oauth2/v3/authorize", TokenURL: "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Scopes:  []string{"openid", "offline_access", "user_data", "vehicle_device_data", "vehicle_charging_cmds"},
		Trades:  map[string]string{"audience": "https://fleet-api.prd.na.vn.cloud.tesla.com"},
		Options: "Tesla's Fleet API in North America and Asia-Pacific; the app's allowed origin and redirect URI are Archive's, and its partner account is registered with the public key at /.well-known/appspecific/com.tesla.3p.public-key.pem on that origin",
		Who:     &Who{Req: Req{URL: "/api/1/users/me"}, Names: []Path{"response/email", "response/full_name"}, Lower: true},
		Actions: map[string]*Action{
			"get": Call[teslaGet, any](Plan{
				Doc: "Read any of Tesla's Fleet API GET endpoints, like { path: \"/api/1/vehicles\" }, { path: \"/api/1/vehicles/<vin>/vehicle_data\", " +
					"query: \"endpoints=charge_state;climate_state;vehicle_state\" } for a car that is awake, { path: \"/api/1/dx/charging/history\", " +
					"query: \"vin=<vin>&startTime=2026-01-01T00:00:00Z\" }, or { path: \"/api/1/dx/charging/invoice/<contentId>\" }: its answer. " +
					"A car that is asleep answers 408; reading it does not wake it, and Tesla bills each read",
				Steps: []Step{{Req: Req{URL: "{+path}?{+query|}"}}},
			}),
		},
		Sync: []*Sync{
			{Name: "vehicles", Gap: time.Second, Every: 6 * time.Hour, Req: Req{URL: "/api/1/vehicles"}, Items: "response", ID: "vin",
				Hydrate: []Hydrate{Fetch{Soft: true, Merge: true, Par: 2,
					Req: Req{URL: "/api/1/vehicles/{vin}/vehicle_data", Query: Q{"endpoints": "charge_state;vehicle_state;vehicle_config"}}, Items: "response"}},
				Doc: "Every vehicle on your account, with its name, model, trim and color, and its battery, range, charging, odometer and software as it last said " +
					"when awake: listed whole every 6 hours, which deletes those gone. A car that is asleep is not woken, and has only its name, VIN and state until it is next awake.",
				To: Map{Table: "vehicle",
					Cols: map[string]SQL{"vin": "data->>'vin'", "title": "data->>'display_name'", "state": "data->>'state'",
						"model": "data#>>'{vehicle_config,car_type}'", "trim": "data#>>'{vehicle_config,trim_badging}'", "color": "data#>>'{vehicle_config,exterior_color}'",
						"odometer": "archive.sync_num(data#>>'{vehicle_state,odometer}')", "battery": "archive.sync_num(data#>>'{charge_state,battery_level}')",
						"range": "archive.sync_num(data#>>'{charge_state,battery_range}')", "charging": "data#>>'{charge_state,charging_state}'",
						"software": "data#>>'{vehicle_state,car_version}'", "read_at": "archive.sync_at(data#>>'{charge_state,timestamp}')"}}},
			{Name: "charging", Gap: time.Second, Every: 24 * time.Hour, Keep: true, Pass: []int{403, 404},
				Req:   Req{URL: "/api/1/dx/charging/history", Query: Q{"pageNo": "{page}", "pageSize": "50", "sortBy": "StartChargingTime", "sortOrder": "DESC"}},
				Items: "data", ID: "sessionId", Page: Offset{Size: 50, Step: 1},
				Doc: "Every Supercharging session of your vehicles, with where, when, how much energy and what it cost, and its invoice's id: listed whole each day, " +
					"50 a page, keeping those Tesla no longer lists.",
				To: Map{Table: "vehicle_charge",
					Cols: map[string]SQL{"vin": "data->>'vin'", "site": "data->>'siteLocationName'", "country": "data->>'countryCode'",
						"started_at": "archive.sync_at(data->>'chargeStartDateTime')", "ended_at": "archive.sync_at(coalesce(data->>'chargeStopDateTime', data->>'unlatchDateTime'))",
						"energy":  "(SELECT sum(archive.sync_num(f->>'usageBase')) FROM archive.sync_list(data->'fees') f WHERE f->>'feeType' = 'CHARGING')",
						"cost":    "(SELECT archive.sync_money(sum(archive.sync_num(f->>'totalDue'))::text, coalesce(max(f->>'currencyCode'), 'USD')) FROM archive.sync_list(data->'fees') f)",
						"billing": "data->>'billingType'", "invoice": "data#>>'{invoices,0,contentId}'"},
					Refs: map[string]SQL{"vehicle": "data->>'vin'"}}},
		},
	})
}

type teslaGet struct {
	Path  string `json:"path" jsonschema:"The endpoint, like /api/1/vehicles"`
	Query string `json:"query,omitempty" jsonschema:"Its query string, like endpoints=charge_state"`
}
