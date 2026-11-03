package connect

import (
	"cmp"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Klaviyo, the email and SMS marketing platform, read only through its API. It reads ARCHIVE_KLAVIYO, an OAuth
// app of a Klaviyo account whose redirect URLs list <API>/connect/klaviyo/callback, with the read scopes here;
// Klaviyo takes the client as HTTP Basic, wants PKCE, and revokes a refresh token the same way. An owner
// connects the account for everyone.
//
// It syncs every profile into crm_contact, the person of their email, and every email and SMS campaign into
// marketing_campaign. Profiles are listed whole the first time and a week after, which deletes those gone, and
// in between read by when they changed since the sync before; campaigns, which are few, are listed whole each
// sync. It pushes nothing: Klaviyo's webhooks are for its partners.
func init() {
	Register(&Connector{
		ID: "klaviyo", Name: "Klaviyo", Integration: true, Env: "ARCHIVE_KLAVIYO", Host: "https://www.klaviyo.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.klaviyo.com/oauth/authorize", TokenURL: "https://a.klaviyo.com/oauth/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeWith: func(rv *Revocation) error {
			id, secret := rv.Client()
			return rv.Form("https://a.klaviyo.com/oauth/revoke", http.Header{"Authorization": {basic(id, secret)}},
				url.Values{"token": {rv.Refresh}, "token_type_hint": {"refresh_token"}})
		},
		Scopes: []string{"accounts:read", "profiles:read", "lists:read", "campaigns:read", "metrics:read"},
		Account: func(c *Client) (string, error) {
			var out struct {
				Data []struct {
					ID         string `json:"id"`
					Attributes struct {
						Contact struct {
							Org string `json:"organization_name"`
						} `json:"contact_information"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := klaviyoGet(c, klaviyoAPI+"/accounts", &out); err != nil {
				return "", err
			}
			if len(out.Data) == 0 {
				return "", errors.New("Klaviyo named no account")
			}
			return cmp.Or(out.Data[0].Attributes.Contact.Org, out.Data[0].ID), nil
		},
		Actions: map[string]*Action{
			"get": Act("Read any of Klaviyo's GET endpoints, like { path: \"/lists\" }, { path: \"/profiles\", query: \"filter=equals(email,'ann@example.com')\" } "+
				"or { path: \"/campaigns/<id>/campaign-messages\" }: its JSON:API answer, { data, links, included } (pass links.next as page for more)",
				func(c *Client, in struct {
					Path  string `json:"path,omitempty" jsonschema:"The endpoint after /api, like /lists or /metrics"`
					Query string `json:"query,omitempty" jsonschema:"Its query, like filter=... or page[size]=50"`
					Page  string `json:"page,omitempty" jsonschema:"The links.next of the call before"`
				}) (any, error) {
					u := in.Page
					if u == "" {
						if !strings.HasPrefix(in.Path, "/") {
							return nil, errors.New("path starts with /, like /lists")
						}
						u = klaviyoAPI + in.Path
						if in.Query != "" {
							u += "?" + in.Query
						}
					} else if !strings.HasPrefix(u, klaviyoAPI+"/") {
						return nil, errors.New("page is the links.next of a call before")
					}
					var out any
					err := klaviyoGet(c, u, &out)
					return out, err
				}),
		},
		Streams: []*Stream{
			{Name: "profiles", Gap: time.Second, Pull: klaviyoProfiles, Table: "crm_contact", Rows: klaviyoProfileRows,
				Doc: "Every profile, by its id, with its email, phone, name, organization, title and location: listed whole the first time and a " +
					"week after, 100 a page, which deletes those gone, and in between those updated since the sync before."},
			{Name: "campaigns", Gap: time.Second, Pull: klaviyoCampaigns, Table: "marketing_campaign", Rows: klaviyoCampaignRows,
				Doc: "Every email and SMS campaign, by its id, with its name, status and when it was scheduled and sent: listed whole each sync, " +
					"so one deleted is deleted."},
		},
	})
}

const (
	klaviyoAPI = "https://a.klaviyo.com/api"
	// klaviyoRevision is the version of Klaviyo's API the connector reads, which every request names.
	klaviyoRevision = "2025-07-15"
)

// klaviyoGet reads u, a Klaviyo API URL, into v, with the revision it is read at.
func klaviyoGet(c *Client, u string, v any) error {
	return c.Send("GET", u, http.Header{"Revision": {klaviyoRevision}, "Accept": {"application/vnd.api+json"}}, nil, v)
}

// klaviyoPage is a page of Klaviyo's JSON:API answer.
type klaviyoPage struct {
	Data  []map[string]any `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

// klaviyoCursor is where the profiles stream is: Since when the sync its records are as of began, Listed when
// its last full listing began, and while a sync is under way, Mode list or delta, Began when it began and Next
// its next page.
type klaviyoCursor struct {
	Since  string `json:"since,omitempty"`
	Listed string `json:"listed,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Began  string `json:"began,omitempty"`
	Next   string `json:"next,omitempty"`
}

func klaviyoProfiles(c *Client, cursor string) (*Page, error) {
	var cur klaviyoCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	now := time.Now().Add(-time.Minute).UTC()
	if cur.Mode == "" {
		cur.Mode, cur.Began, cur.Next = "delta", now.Format(time.RFC3339), ""
		if cur.Since == "" || now.Sub(stamp(cur.Listed)) > 7*24*time.Hour {
			cur.Mode = "list"
		}
	}
	u := cur.Next
	if u == "" {
		q := url.Values{"page[size]": {"100"}}
		if cur.Mode == "delta" {
			q.Set("filter", "greater-than(updated,"+cur.Since+")")
			q.Set("sort", "updated")
		}
		u = klaviyoAPI + "/profiles?" + q.Encode()
	}
	var res klaviyoPage
	if err := klaviyoGet(c, u, &res); err != nil {
		return nil, err
	}
	page := &Page{Full: cur.Mode == "list"}
	for _, m := range res.Data {
		page.Records = append(page.Records, Record{ID: str(m, "id"), Data: m, Updated: stamp(str(obj(m, "attributes"), "updated"))})
	}
	if res.Links.Next != "" {
		cur.Next, page.More = res.Links.Next, true
	} else {
		if cur.Mode == "list" {
			cur.Listed = cur.Began
		}
		cur = klaviyoCursor{Since: cur.Began, Listed: cur.Listed}
	}
	page.Cursor = cursorFor(cur)
	return page, nil
}

// klaviyoCampaigns lists every campaign of each channel, as Klaviyo lists one channel at a time; the cursor is
// the channel under way and its next page.
func klaviyoCampaigns(c *Client, cursor string) (*Page, error) {
	var cur struct {
		Channel string `json:"channel,omitempty"`
		Next    string `json:"next,omitempty"`
	}
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	if cur.Channel == "" {
		cur.Channel = "email"
	}
	u := cur.Next
	if u == "" {
		u = klaviyoAPI + "/campaigns?" + url.Values{"filter": {"equals(messages.channel,'" + cur.Channel + "')"}}.Encode()
	}
	var res klaviyoPage
	if err := klaviyoGet(c, u, &res); err != nil {
		return nil, err
	}
	page := &Page{Full: true}
	for _, m := range res.Data {
		m["channel"] = cur.Channel
		page.Records = append(page.Records, Record{ID: str(m, "id"), Data: m, Updated: stamp(str(obj(m, "attributes"), "updated_at"))})
	}
	switch {
	case res.Links.Next != "":
		cur.Next = res.Links.Next
	case cur.Channel == "email":
		cur.Channel, cur.Next = "sms", ""
	default:
		cur.Channel, cur.Next = "", ""
	}
	page.More = cur.Channel != ""
	page.Cursor = cursorFor(cur)
	return page, nil
}

// klaviyoProfileRows is a profile as a contact, the person of its email.
func klaviyoProfileRows(m map[string]any) *Row {
	a := obj(m, "attributes")
	loc := obj(a, "location")
	email := Email(str(a, "email"))
	return rowOf(map[string]any{"first_name": str(a, "first_name"), "last_name": str(a, "last_name"), "email": email, "phone": str(a, "phone_number"),
		"job_title": str(a, "title"), "company_name": str(a, "organization"), "stage": "subscriber", "city": str(loc, "city"), "state": str(loc, "region"),
		"country": str(loc, "country"), "made_at": instant(str(a, "created")), "edited_at": instant(str(a, "updated"))}).person("person", email)
}

func klaviyoCampaignRows(m map[string]any) *Row {
	a := obj(m, "attributes")
	return rowOf(map[string]any{"title": str(a, "name"), "channel": str(m, "channel"), "status": str(a, "status"), "archived": flag(a, "archived"),
		"scheduled_at": instant(str(a, "scheduled_at")), "sent_at": instant(str(a, "send_time")), "made_at": instant(str(a, "created_at")),
		"edited_at": instant(str(a, "updated_at"))})
}
