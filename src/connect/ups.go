package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "ups", Name: "UPS",
		Info: Info{By: "UPS", Description: "Track the UPS packages your mail mentions, and any you ask about", Category: CatHome, Icon: IconInbox,
			Scopes: []string{"Sign in to your UPS account", "Find UPS tracking numbers in the mail you synced", "Look up where each package is with UPS, as you"}},
		API:        "https://onlinetools.ups.com",
		Pace:       250 * time.Millisecond,
		Endpoint:   oauth2.Endpoint{AuthURL: "https://onlinetools.ups.com/security/v1/oauth/authorize", TokenURL: "https://onlinetools.ups.com/security/v1/oauth/token"},
		RefreshURL: "https://onlinetools.ups.com/security/v1/oauth/refresh",
		Params:     map[string]string{"type": "ups_com_api"},
		Options:    "Archive's app at developer.ups.com, with Authorization (OAuth) and Tracking, its callback <Archive API>/connect/ups/callback",
		Sign:       map[string]string{"Authorization": "Bearer {token}", "transactionSrc": "archive"},
		Errors:     &Errors{At: "response/errors/0/message", Code: "response/errors/0/code"},
		Actions: map[string]*Action{
			"track": Call[parcelOne, any](Plan{
				Doc: "Where a UPS package is, by its tracking number, like { number: \"1Z999AA10123456784\" }: its status, delivery date, " +
					"where it is from and going, and every scan, newest first",
				Steps: []Step{{As: "got", Req: Req{URL: "/api/track/v1/details/{number}", Query: Q{"locale": "en_US"}, Header: map[string][]string{"transId": {"{nonce}"}}}}},
				Out:   "{got/trackResponse/shipment/0/package}",
			}),
		},
		Sync: []*Sync{{Name: "packages", Gap: time.Second, Every: 4 * time.Hour, Keep: true, Pass: []int{400, 404},
			Parts: &Parts{Mail: &Scan{Find: `(?i)\b(1Z[0-9A-Z]{16})\b`, Near: "1Z[0-9A-Z]{16}", Days: parcelDays},
				Vals: Obj{"subject": "{subject}", "message": "{message}", "sent_at": "{sent_at}"}},
			Req:   Req{URL: "/api/track/v1/details/{part}", Query: Q{"locale": "en_US"}, Header: map[string][]string{"transId": {"{part}"}}},
			Items: "trackResponse/shipment/0/package", ID: "trackingNumber",
			Set: Obj{"mail_subject": "{in/subject}", "mail_message": "{in/message}"},
			Doc: "Every UPS package whose tracking number, 1Z and 16 more, is in mail synced from your mailboxes in the last " + parcelDaysText +
				", with its status, delivery date, origin, destination, who signed, and each scan: looked up again every 4 hours, " +
				"and kept once its mail is older; a number UPS does not know is passed over.",
			To: Map{Table: "parcel",
				Cols: map[string]SQL{"carrier": "'UPS'", "tracking": "data->>'trackingNumber'",
					"status":      "coalesce(data#>>'{currentStatus,description}', data#>>'{activity,0,status,description}')",
					"status_code": "data#>>'{activity,0,status,type}'", "service": "data#>>'{service,description}'",
					"weight":       "nullif(concat_ws(' ', data#>>'{weight,weight}', data#>>'{weight,unitOfMeasurement}'), '')",
					"delivered":    "data#>>'{activity,0,status,type}' = 'D'",
					"delivered_at": "CASE WHEN data#>>'{activity,0,status,type}' = 'D' THEN " + upsAt("data->'activity'->0") + " END",
					"expected_on":  "(SELECT to_date(d->>'date', 'YYYYMMDD') FROM archive.sync_list(data->'deliveryDate') d WHERE d->>'date' ~ '^\\d{8}$' LIMIT 1)",
					"signed_by":    "nullif(data#>>'{deliveryInformation,receivedBy}', '')",
					"from_city":    upsPlace("ORIGIN", "city"), "from_state": upsPlace("ORIGIN", "stateProvince"), "from_country": upsPlace("ORIGIN", "countryCode"),
					"city": upsPlace("DESTINATION", "city"), "state": upsPlace("DESTINATION", "stateProvince"), "country": upsPlace("DESTINATION", "countryCode"),
					"last_event": "data#>>'{activity,0,status,description}'", "last_event_at": upsAt("data->'activity'->0"),
					"mail_subject": "data->>'mail_subject'", "mail_message": "data->>'mail_message'"},
				Children: []Kids{{Table: "parcel_event", From: "archive.sync_list(data->'activity') a",
					Cols: map[string]SQL{"at": upsAt("a"), "status": "a#>>'{status,description}'", "code": "a#>>'{status,type}'",
						"city": "a#>>'{location,address,city}'", "state": "a#>>'{location,address,stateProvince}'", "country": "a#>>'{location,address,countryCode}'"}}}}}},
	})
}

const (
	parcelDays     = 45
	parcelDaysText = "45 days"
)

func upsAt(a string) SQL {
	return SQL("archive.sync_at(regexp_replace(" + a + "->>'gmtDate', '^(\\d{4})(\\d{2})(\\d{2})$', '\\1-\\2-\\3') || 'T' || " +
		"regexp_replace(" + a + "->>'gmtTime', '^(\\d{2}):?(\\d{2}):?(\\d{2})$', '\\1:\\2:\\3') || 'Z')")
}

func upsPlace(kind, field string) SQL {
	return SQL("(SELECT p#>>'{address," + field + "}' FROM archive.sync_list(data->'packageAddress') p WHERE p->>'type' = '" + kind + "' LIMIT 1)")
}

type parcelOne struct {
	Number string `json:"number" jsonschema:"The tracking number"`
}
