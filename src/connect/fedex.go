package connect

import "time"

func init() {
	Register(&Connector{
		ID: "fedex", Name: "FedEx",
		Info: Info{By: "FedEx", Description: "Track the FedEx packages your mail mentions, and any you ask about", Category: CatHome, Icon: IconInbox,
			Scopes: []string{"Find FedEx tracking numbers in the mail you synced", "Look up where each package is with FedEx"}},
		API:  "https://apis.fedex.com",
		Pace: 250 * time.Millisecond,
		Tap:  true, Env: "ARCHIVE_FEDEX", Host: "https://apis.fedex.com", Options: "the API Key and Secret Key of Archive's Production project at developer.fedex.com, with the Track API",
		Trade: &Req{Method: POST, URL: "/oauth/token",
			Form: Q{"grant_type": "client_credentials", "client_id": "{client/id}", "client_secret": "{client/secret}"}},
		Sign:   map[string]string{"Authorization": "Bearer {key/access}", "X-locale": "en_US"},
		Errors: &Errors{At: "errors/0/message", Code: "errors/0/code"},
		Actions: map[string]*Action{
			"track": Call[parcelOne, any](Plan{
				Doc: "Where a FedEx package is, by its tracking number, like { number: \"123456789012\" }: its latest status, " +
					"delivery and estimated times, where it is from and going, and every scan, newest first",
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/track/v1/trackingnumbers", Body: fedexBody("{number}")}}},
				Out:   "{got/output/completeTrackResults/0/trackResults/0}",
			}),
		},
		Sync: []*Sync{{Name: "packages", Gap: time.Second, Every: 4 * time.Hour, Keep: true, Pass: []int{400, 404},
			Parts: &Parts{Mail: &Scan{Find: `\b(\d{12}|\d{15}|\d{20}|\d{22})\b`, Near: "fedex", Days: parcelDays},
				Vals: Obj{"subject": "{subject}", "message": "{message}", "sent_at": "{sent_at}"}},
			Req: Req{Method: POST, Read: true, URL: "/track/v1/trackingnumbers",
				Body: fedexBody("{part}")},
			Items: "output/completeTrackResults/0/trackResults", ID: "trackingNumberInfo/trackingNumber",
			Skip: []Test{{At: "error/code"}},
			Set:  Obj{"mail_subject": "{in/subject}", "mail_message": "{in/message}"},
			Doc: "Every FedEx package whose tracking number, 12, 15, 20 or 22 digits, is in mail synced from your mailboxes in the last " + parcelDaysText +
				" that says FedEx, with its status, delivery and estimated dates, origin, destination, who signed, and each scan: looked up again every 4 hours, " +
				"and kept once its mail is older; a number FedEx does not know is passed over.",
			To: Map{Table: "parcel",
				Cols: map[string]SQL{"carrier": "'FedEx'", "tracking": "data#>>'{trackingNumberInfo,trackingNumber}'",
					"status":      "coalesce(data#>>'{latestStatusDetail,statusByLocale}', data#>>'{latestStatusDetail,description}')",
					"status_code": "data#>>'{latestStatusDetail,code}'", "service": "data#>>'{serviceDetail,description}'",
					"weight":       "(SELECT nullif(concat_ws(' ', w->>'value', w->>'unit'), '') FROM archive.sync_list(data#>'{packageDetails,weightAndDimensions,weight}') w LIMIT 1)",
					"delivered":    "data#>>'{latestStatusDetail,code}' = 'DL'",
					"delivered_at": fedexWhen("ACTUAL_DELIVERY"),
					"expected_on": "archive.sync_day(coalesce((SELECT d->>'dateTime' FROM archive.sync_list(data->'dateAndTimes') d WHERE d->>'type' = 'ESTIMATED_DELIVERY' LIMIT 1), " +
						"data#>>'{estimatedDeliveryTimeWindow,window,ends}'))",
					"signed_by": "nullif(data#>>'{deliveryDetails,receivedByName}', '')",
					"from_city": "data#>>'{shipperInformation,address,city}'", "from_state": "data#>>'{shipperInformation,address,stateOrProvinceCode}'",
					"from_country": "data#>>'{shipperInformation,address,countryCode}'",
					"city":         "data#>>'{recipientInformation,address,city}'", "state": "data#>>'{recipientInformation,address,stateOrProvinceCode}'",
					"country":    "data#>>'{recipientInformation,address,countryCode}'",
					"last_event": "data#>>'{scanEvents,0,eventDescription}'", "last_event_at": "archive.sync_at(data#>>'{scanEvents,0,date}')",
					"mail_subject": "data->>'mail_subject'", "mail_message": "data->>'mail_message'"},
				Children: []Kids{{Table: "parcel_event", From: "archive.sync_list(data->'scanEvents') e",
					Cols: map[string]SQL{"at": "archive.sync_at(e->>'date')", "status": "e->>'eventDescription'", "code": "e->>'eventType'",
						"city": "e#>>'{scanLocation,city}'", "state": "e#>>'{scanLocation,stateOrProvinceCode}'", "country": "e#>>'{scanLocation,countryCode}'"}}}}}},
	})
}

func fedexWhen(kind string) SQL {
	return SQL("(SELECT archive.sync_at(d->>'dateTime') FROM archive.sync_list(data->'dateAndTimes') d WHERE d->>'type' = '" + kind + "' LIMIT 1)")
}

func fedexBody(number string) Obj {
	return Obj{"includeDetailedScans": true, "trackingInfo": []any{Obj{"trackingNumberInfo": Obj{"trackingNumber": number}}}}
}
