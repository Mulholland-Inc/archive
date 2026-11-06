package connect

import "time"

func init() {
	Register(&Connector{
		ID: "meta_ads", Name: "Meta Ads", Integration: true,
		Info: Info{By: "Meta", Description: "Read an ad account’s campaigns and what they spent and got each day", Category: CatMarketing, Icon: IconUsers, Scopes: []string{"Read the ad account’s campaigns and results"}},
		Keys: []Key{
			{Name: "account", Label: "Ad account ID", Help: "In Meta Ads Manager, the account's ID, like 1234567890; act_ before it is fine"},
			{Name: "token", Label: "System user access token", Secret: true,
				Help: "In Meta Business Settings, Users, System users: a system user with this ad account assigned, and a token it generates with ads_read"},
		},
		Sign: map[string]string{"Authorization": "Bearer {key/token}"},
		API:  "https://graph.facebook.com/v23.0/act_{account}",
		Let:  []Step{{As: "account", In: "{key/account}", Find: `^(?:act_)?(.*)$`}},
		Who:  &Who{Req: Req{URL: "?fields=name,account_id"}, Names: []Path{"name", "account_id"}},
		Actions: map[string]*Action{
			"insights": Call[metaFind, any](Plan{
				Doc: "The account's results over a time, at a level, like { level: \"ad\", since: \"2026-10-01\", until: \"2026-10-31\", " +
					"fields: \"ad_name,spend,impressions,clicks,actions\" }: { data: [...], paging } (pass paging.next as page for more)",
				Steps: []Step{
					{If: "daily", As: "by", In: "1"},
					{If: "page", Req: Req{URL: "{+page}"}},
					{Unless: "page", Req: Req{URL: "/insights", Query: Q{"level": "{level|campaign}", "time_range": `{"since":"{since}","until":"{until}"}`,
						"fields": "{fields|campaign_name,spend,impressions,clicks,actions}", "limit": "500", "time_increment": "{by}"}}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "campaigns", Gap: time.Second, Keep: true, Req: Req{URL: "/campaigns", Query: Q{"fields": "id,name,objective,effective_status,created_time,updated_time,start_time,stop_time", "limit": "200"}},
				Items: "data", Updated: "updated_time", Page: metaPage,
				Doc: "Every campaign of the ad account, with its name, objective, status and when it runs: listed each sync; one archived or " +
					"deleted there, which Meta stops listing, is kept, as its days of results name it.",
				To: Map{Table: "marketing_campaign",
					Cols: map[string]SQL{"title": "data->>'name'", "channel": "'meta ads'", "status": "data->>'effective_status'", "scheduled_at": "archive.sync_at(data->>'start_time')",
						"made_at": "archive.sync_at(data->>'created_time')", "edited_at": "archive.sync_at(data->>'updated_time')"}}},
			{Name: "daily", Gap: time.Second, Keep: true, Page: metaPage, Items: "data", Key: []Reads{{Of: "{campaign_id}/{date_start}"}},
				Parts: &Parts{Span: &Span{First: Shift{-3, 0, 1}, Then: Shift{0, 0, -28}, Step: Shift{0, 1, 0}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}},
				Req: Req{URL: "/insights", Query: Q{"level": "campaign", "time_increment": "1", "limit": "500", "time_range": `{"since":"{in/start}","until":"{in/end}"}`,
					"fields": "campaign_id,campaign_name,spend,impressions,reach,clicks,actions,action_values,account_currency,date_start"}},
				Doc: "Each campaign's results of each day, by <campaign id>/<day>: spend, impressions, reach, clicks, and the purchases, their " +
					"value and the leads Meta credits it with. Three years of days the first time, a month at a time, and then the last 28 days " +
					"each sync.",
				To: Map{Table: "ad_metric",
					Cols: map[string]SQL{"on_date": "archive.sync_day(data->>'date_start')", "campaign_name": "data->>'campaign_name'",
						"spend":       "archive.sync_money(data->>'spend', coalesce(nullif(data->>'account_currency', ''), 'USD'))",
						"impressions": "archive.sync_num(data->>'impressions')", "reach": "archive.sync_num(data->>'reach')", "clicks": "archive.sync_num(data->>'clicks')",
						"purchases": "archive.sync_num((SELECT a->>'value' FROM archive.sync_list(data->'actions') a WHERE a->>'action_type' IN ('purchase', 'offsite_conversion.fb_pixel_purchase') LIMIT 1))",
						"purchase_value": `archive.sync_money((SELECT a->>'value' FROM archive.sync_list(data->'action_values') a WHERE a->>'action_type' IN ('purchase', 'offsite_conversion.fb_pixel_purchase') LIMIT 1),
							coalesce(nullif(data->>'account_currency', ''), 'USD'))`,
						"leads": "archive.sync_num((SELECT a->>'value' FROM archive.sync_list(data->'actions') a WHERE a->>'action_type' IN ('lead', 'offsite_conversion.fb_pixel_lead') LIMIT 1))"},
					Refs: map[string]SQL{"campaign": "data->>'campaign_id'"}}},
		},
	})
}

var metaPage = NextURL{At: "paging/next", Drop: []string{"access_token"}}

type metaFind struct {
	Level  string `json:"level,omitempty" jsonschema:"account, campaign, adset or ad; campaign when left out"`
	Since  string `json:"since" jsonschema:"The first day, YYYY-MM-DD"`
	Until  string `json:"until" jsonschema:"The last day, YYYY-MM-DD"`
	Fields string `json:"fields,omitempty" jsonschema:"The fields, comma separated; campaign_name, spend, impressions, clicks and actions when left out"`
	Daily  bool   `json:"daily,omitempty" jsonschema:"A row a day, not one for the whole time"`
	Page   string `json:"page,omitempty" jsonschema:"The paging.next of the call before"`
}
