package connect

import (
	"cmp"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Meta Ads, one ad account's campaigns and how they did, read through Meta's Marketing API. Meta's sign-in gives
// tokens that end in 60 days and cannot be refreshed, so this connects as a system user of the business's own
// Meta Business Manager: in Business Settings, Users, System users, a system user with the ad account assigned
// and a token with ads_read, which does not expire, pasted in by an owner with the ad account's id. The token
// lasts until the business revokes it.
//
// It syncs the account's campaigns into marketing_campaign, listed whole each sync, and each campaign's results
// of each day into ad_metric: spend, impressions, reach, clicks, and the purchases and leads Meta credits it
// with. The first sync reads three years of days, as far back as Meta keeps them; each sync after reads the
// last 28 days again, as Meta still credits a day's results for that long. It pushes nothing.
func init() {
	Register(&Connector{
		ID: "meta_ads", Name: "Meta Ads", Integration: true,
		Keys: []Key{
			{Name: "account", Label: "Ad account ID", Help: "In Meta Ads Manager, the account's ID, like 1234567890; act_ before it is fine"},
			{Name: "token", Label: "System user access token", Secret: true,
				Help: "In Meta Business Settings, Users, System users: a system user with this ad account assigned, and a token it generates with ads_read"},
		},
		Authorize: func(r *http.Request, c *Client) { r.Header.Set("Authorization", "Bearer "+c.Key("token")) },
		Account: func(c *Client) (string, error) {
			var a struct {
				Name string `json:"name"`
				ID   string `json:"account_id"`
			}
			err := c.Get(metaAccount(c)+"?fields=name,account_id", &a)
			return cmp.Or(a.Name, a.ID), err
		},
		Actions: map[string]*Action{
			"insights": Act("The account's results over a time, at a level, like { level: \"ad\", since: \"2026-10-01\", until: \"2026-10-31\", "+
				"fields: \"ad_name,spend,impressions,clicks,actions\" }: { data: [...], paging } (pass paging.next as page for more)",
				func(c *Client, in struct {
					Level  string `json:"level,omitempty" jsonschema:"account, campaign, adset or ad; campaign when left out"`
					Since  string `json:"since" jsonschema:"The first day, YYYY-MM-DD"`
					Until  string `json:"until" jsonschema:"The last day, YYYY-MM-DD"`
					Fields string `json:"fields,omitempty" jsonschema:"The fields, comma separated; campaign_name, spend, impressions, clicks and actions when left out"`
					Daily  bool   `json:"daily,omitempty" jsonschema:"A row a day, not one for the whole time"`
					Page   string `json:"page,omitempty" jsonschema:"The paging.next of the call before"`
				}) (any, error) {
					u := in.Page
					if u == "" {
						q := url.Values{"level": {cmp.Or(in.Level, "campaign")}, "time_range": {`{"since":"` + in.Since + `","until":"` + in.Until + `"}`},
							"fields": {cmp.Or(in.Fields, "campaign_name,spend,impressions,clicks,actions")}, "limit": {"500"}}
						if in.Daily {
							q.Set("time_increment", "1")
						}
						u = metaAccount(c) + "/insights?" + q.Encode()
					} else if !strings.HasPrefix(u, metaAPI+"/") {
						return nil, errors.New("page is the paging.next of a call before")
					}
					var out any
					err := c.Get(u, &out)
					return out, err
				}),
		},
		Streams: []*Stream{
			{Name: "campaigns", Gap: time.Second, Pull: metaCampaigns, Table: "marketing_campaign", Rows: metaCampaignRows,
				Doc: "Every campaign of the ad account, deleted ones aside, with its name, objective, status and when it runs: listed whole each sync."},
			{Name: "daily", Gap: time.Second, Pull: metaDaily, Table: "ad_metric", Rows: metaDailyRows,
				Doc: "Each campaign's results of each day, by <campaign id>/<day>: spend, impressions, reach, clicks, and the purchases, their " +
					"value and the leads Meta credits it with. Three years of days the first time, a month at a time, and then the last 28 days " +
					"each sync."},
		},
	})
}

// metaAPI is the version of Meta's Graph API the connector reads.
const metaAPI = "https://graph.facebook.com/v21.0"

// metaAccount is the ad account the keys name, as act_<id>.
func metaAccount(c *Client) string {
	id := strings.TrimPrefix(strings.TrimSpace(c.Key("account")), "act_")
	return metaAPI + "/act_" + url.PathEscape(id)
}

// metaPage is a page of a Graph list.
type metaPage struct {
	Data   []map[string]any `json:"data"`
	Paging struct {
		Next string `json:"next"`
	} `json:"paging"`
}

func metaCampaigns(c *Client, cursor string) (*Page, error) {
	u := cursor
	if u == "" {
		u = metaAccount(c) + "/campaigns?" + url.Values{"fields": {"id,name,objective,effective_status,created_time,updated_time,start_time,stop_time"},
			"limit": {"200"}}.Encode()
	} else if !strings.HasPrefix(u, metaAPI+"/") {
		return nil, errors.New("the campaigns cursor is no Graph URL")
	}
	var res metaPage
	if err := c.Get(u, &res); err != nil {
		return nil, err
	}
	page := &Page{Full: true, More: res.Paging.Next != "", Cursor: res.Paging.Next}
	for _, m := range res.Data {
		page.Records = append(page.Records, Record{ID: str(m, "id"), Data: m, Updated: stamp(str(m, "updated_time"))})
	}
	return page, nil
}

// metaCursor is where the daily stream is: Done says three years have been read once; From and To the days of
// the month under way, and Next its next page.
type metaCursor struct {
	Done bool   `json:"done,omitempty"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	Next string `json:"next,omitempty"`
}

func metaDaily(c *Client, cursor string) (*Page, error) {
	var cur metaCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if cur.From == "" {
		from := today.AddDate(0, 0, -28)
		if !cur.Done {
			from = today.AddDate(-3, 0, 1)
		}
		cur.From = from.Format(time.DateOnly)
	}
	from, _ := time.Parse(time.DateOnly, cur.From)
	to := from.AddDate(0, 1, -1)
	if to.After(today) {
		to = today
	}
	cur.To = to.Format(time.DateOnly)
	u := cur.Next
	if u == "" {
		u = metaAccount(c) + "/insights?" + url.Values{"level": {"campaign"}, "time_increment": {"1"}, "limit": {"500"},
			"time_range": {`{"since":"` + cur.From + `","until":"` + cur.To + `"}`},
			"fields":     {"campaign_id,campaign_name,spend,impressions,reach,clicks,actions,action_values,account_currency,date_start"}}.Encode()
	} else if !strings.HasPrefix(u, metaAPI+"/") {
		return nil, errors.New("the daily cursor is no Graph URL")
	}
	var res metaPage
	if err := c.Get(u, &res); err != nil {
		return nil, err
	}
	page := &Page{}
	for _, m := range res.Data {
		page.Records = append(page.Records, Record{ID: str(m, "campaign_id") + "/" + str(m, "date_start"), Data: m})
	}
	switch {
	case res.Paging.Next != "":
		cur.Next = res.Paging.Next
	case to.Before(today):
		cur = metaCursor{Done: cur.Done, From: to.AddDate(0, 0, 1).Format(time.DateOnly)}
	default:
		cur = metaCursor{Done: true}
	}
	page.More = cur.From != ""
	page.Cursor = cursorFor(cur)
	return page, nil
}

func metaCampaignRows(m map[string]any) *Row {
	return rowOf(map[string]any{"title": str(m, "name"), "channel": "meta ads", "status": str(m, "effective_status"),
		"scheduled_at": instant(str(m, "start_time")), "made_at": instant(str(m, "created_time")), "edited_at": instant(str(m, "updated_time"))})
}

// metaAction is the value of the action of type kind in a list of Meta's actions, like purchases, or nil.
func metaAction(v any, kinds ...string) any {
	for _, a := range objects(v, "") {
		for _, k := range kinds {
			if str(a, "action_type") == k {
				return decimal(a["value"])
			}
		}
	}
	return nil
}

// metaDailyRows is a campaign's day, with the purchases and leads Meta credits it with, as their pixel counts
// them or else as Meta does.
func metaDailyRows(m map[string]any) *Row {
	cur := cmp.Or(str(m, "account_currency"), "USD")
	var value Money
	if v := metaAction(m["action_values"], "purchase", "offsite_conversion.fb_pixel_purchase"); v != nil {
		value = money(v, cur)
	}
	return rowOf(map[string]any{"on_date": day(str(m, "date_start")), "campaign_name": str(m, "campaign_name"), "spend": money(m["spend"], cur),
		"impressions": decimal(m["impressions"]), "reach": decimal(m["reach"]), "clicks": decimal(m["clicks"]),
		"purchases": metaAction(m["actions"], "purchase", "offsite_conversion.fb_pixel_purchase"), "purchase_value": value,
		"leads": metaAction(m["actions"], "lead", "offsite_conversion.fb_pixel_lead")}).ref("campaign", str(m, "campaign_id"))
}
