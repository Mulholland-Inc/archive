package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "nest", Name: "Google Nest",
		Info: Info{By: "Google", Description: "Read your Nest thermostats, cameras and doorbells, and change a thermostat when you ask", Category: CatHome, Icon: IconList,
			Scopes: []string{"Read the homes, rooms and devices you share with Archive", "Change a thermostat’s mode and temperature when you ask"}},
		API:  "https://smartdevicemanagement.googleapis.com/v1/enterprises/{option/project}",
		Busy: []string{"RESOURCE_EXHAUSTED", "Rate limited"},
		Endpoint: oauth2.Endpoint{AuthURL: "https://nestservices.google.com/partnerconnections/{option/project}/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke:  "https://oauth2.googleapis.com/revoke",
		Scopes:  []string{"https://www.googleapis.com/auth/sdm.service"},
		Params:  map[string]string{"access_type": "offline", "prompt": "consent"},
		Options: "?project=<Device Access project ID>; the OAuth client is a Google Cloud web client the Device Access project names",
		Who: &Who{Req: Req{URL: "/structures"}, Names: []Path{"structures/0/traits/sdm.structures.traits.Info/customName", "structures/0/name"},
			Find: `^(?:.*/)?([^/]+)$`},
		Errors: &Errors{At: "error/message", Code: "error/status"},
		Actions: map[string]*Action{
			"devices": Call[struct{}, any](Plan{
				Doc:   "Every device you shared, with its type, room and traits as it reads now, like a thermostat's sdm.devices.traits.Temperature",
				Steps: []Step{{As: "got", Req: Req{URL: "/devices"}}},
				Out:   "{got/devices|[]}",
			}),
			"command": Call[nestCommand, any](Plan{
				Doc: "Send a device a command, like { device: \"<id>\", command: \"sdm.devices.commands.ThermostatTemperatureSetpoint.SetHeat\", params: { heatCelsius: 20 } }, " +
					"or ThermostatMode.SetMode with { mode: \"HEAT\" }, \"COOL\", \"HEATCOOL\" or \"OFF\", ThermostatEco.SetMode with { mode: \"MANUAL_ECO\" } or \"OFF\", " +
					"ThermostatTemperatureSetpoint.SetCool with coolCelsius, SetRange with both, or Fan.SetTimer with { timerMode: \"ON\", duration: \"900s\" }. " +
					"Only when someone asks; a camera's stream commands are not for Archive",
				Steps: []Step{{Req: Req{Method: POST, URL: "/devices/{device}:executeCommand", Body: Obj{"command": "{command}", "params": "{params}"}}}},
			}),
		},
		Sync: []*Sync{{Name: "devices", Gap: time.Second, Every: time.Hour, Req: Req{URL: "/devices"}, Items: "devices", Key: []Reads{{Of: "{name}", Find: `([^/]+)$`}},
			Doc: "Every thermostat, camera, doorbell and display you shared, with its room, whether it is online, and a thermostat's mode, heating or cooling, " +
				"temperature, humidity and set points as they read now: listed whole every hour, which deletes those you stop sharing. " +
				"A camera's motion, person and doorbell events reach a partner only as they happen, so none are here.",
			To: Map{Table: "home_device",
				Cols: map[string]SQL{"kind": "lower(regexp_replace(data->>'type', '^.*\\.', ''))", "title": "coalesce(nullif(" + nestTrait("Info", "customName") + ", ''), data#>>'{parentRelations,0,displayName}')",
					"room": "data#>>'{parentRelations,0,displayName}'", "online": nestTrait("Connectivity", "status") + " = 'ONLINE'",
					"mode": nestTrait("ThermostatMode", "mode"), "eco": nestTrait("ThermostatEco", "mode"), "hvac": nestTrait("ThermostatHvac", "status"),
					"temperature": "archive.sync_num(" + nestTrait("Temperature", "ambientTemperatureCelsius") + ")",
					"humidity":    "archive.sync_num(" + nestTrait("Humidity", "ambientHumidityPercent") + ")",
					"heat_to":     "archive.sync_num(" + nestTrait("ThermostatTemperatureSetpoint", "heatCelsius") + ")",
					"cool_to":     "archive.sync_num(" + nestTrait("ThermostatTemperatureSetpoint", "coolCelsius") + ")",
					"scale":       nestTrait("Settings", "temperatureScale"),
					"traits":      "coalesce((SELECT jsonb_agg(regexp_replace(k, '^sdm\\.devices\\.traits\\.', '') ORDER BY k) FROM jsonb_object_keys(data->'traits') k), '[]'::jsonb)"}}}},
	})
}

func nestTrait(trait, field string) SQL {
	return SQL("data#>>'{traits,sdm.devices.traits." + trait + "," + field + "}'")
}

type nestCommand struct {
	Device  string         `json:"device" jsonschema:"The device's id, the end of its name from devices"`
	Command string         `json:"command" jsonschema:"The command, like sdm.devices.commands.ThermostatMode.SetMode"`
	Params  map[string]any `json:"params" jsonschema:"Its parameters, like { mode: \"HEAT\" }"`
}
