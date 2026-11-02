package connect

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// HubSpot's CRM, read only: contacts, companies and deals. It reads ARCHIVE_HUBSPOT, an app of a HubSpot
// developer account whose redirect URLs list <API>/connect/hubspot/callback and whose required scopes are
// exactly the Scopes here. HubSpot's own v3 token endpoint takes the client in the form, and says whose
// token it is when asked with the client's secret.
func init() {
	Register(&Connector{
		ID: "hubspot", Name: "HubSpot", Env: "ARCHIVE_HUBSPOT", Host: "https://app.hubspot.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.hubspot.com/oauth/authorize", TokenURL: "https://api.hubspot.com/oauth/v3/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeWith: func(rv *Revocation) error {
			id, secret := rv.Client()
			return rv.Form("https://api.hubapi.com/oauth/v3/token/revoke", nil,
				url.Values{"client_id": {id}, "client_secret": {secret}, "token": {rv.Refresh}, "token_type_hint": {"refresh_token"}})
		},
		Scopes: []string{"oauth", "crm.objects.contacts.read", "crm.objects.companies.read", "crm.objects.deals.read"},
		Account: func(c *Client) (string, error) {
			id, secret := c.Credentials()
			var t struct {
				User      string `json:"user"`
				HubDomain string `json:"hub_domain"`
			}
			err := c.Do("POST", "https://api.hubspot.com/oauth/v3/token/introspect",
				url.Values{"client_id": {id}, "client_secret": {secret}, "token_type_hint": {"access_token"}, "access_token": {c.Token()}}, &t)
			return cmp.Or(t.HubDomain, strings.ToLower(t.User)), err
		},
		Actions: map[string]*Action{
			"search": Act("Find contacts, companies or deals by words in their default text properties (names, emails, domains, phone numbers), "+
				"newest first: { results: [{ id, properties, createdAt, updatedAt }], next } (pass next as page for more)",
				func(c *Client, in struct {
					Type       string   `json:"type" jsonschema:"contacts, companies or deals"`
					Query      string   `json:"query,omitempty" jsonschema:"Words to find; every record when left out"`
					Properties []string `json:"properties,omitempty" jsonschema:"The properties to return, like firstname, email, amount or dealstage; HubSpot's defaults when left out"`
					Limit      int      `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 25 when left out"`
					Page       string   `json:"page,omitempty" jsonschema:"The next of the search before, for the records after it"`
				}) (any, error) {
					if err := hubspotType(in.Type); err != nil {
						return nil, err
					}
					body := map[string]any{"limit": clamp(in.Limit, 25, 100), "sorts": []map[string]string{{"propertyName": "createdate", "direction": "DESCENDING"}}}
					if in.Query != "" {
						body["query"] = in.Query
					}
					if len(in.Properties) > 0 {
						body["properties"] = in.Properties
					}
					if in.Page != "" {
						body["after"] = in.Page
					}
					var out struct {
						Results []any `json:"results"`
						Paging  struct {
							Next struct {
								After string `json:"after"`
							} `json:"next"`
						} `json:"paging"`
					}
					out.Results = []any{}
					err := c.Do("POST", hubspotAPI+"/crm/v3/objects/"+in.Type+"/search", body, &out)
					return map[string]any{"results": out.Results, "next": out.Paging.Next.After}, err
				}),
			"get": Act("One contact, company or deal by id, with the properties asked for and the ids of the records it is associated with: "+
				"{ id, properties, associations, createdAt, updatedAt }",
				func(c *Client, in struct {
					Type         string   `json:"type" jsonschema:"contacts, companies or deals"`
					ID           string   `json:"id" jsonschema:"The record's id"`
					Properties   []string `json:"properties,omitempty" jsonschema:"The properties to return; HubSpot's defaults when left out"`
					Associations []string `json:"associations,omitempty" jsonschema:"The types of the records associated with it to list, like companies or deals"`
				}) (any, error) {
					if err := hubspotType(in.Type); err != nil {
						return nil, err
					}
					q := url.Values{}
					if len(in.Properties) > 0 {
						q.Set("properties", strings.Join(in.Properties, ","))
					}
					if len(in.Associations) > 0 {
						q.Set("associations", strings.Join(in.Associations, ","))
					}
					var out any
					err := c.Get(hubspotAPI+"/crm/v3/objects/"+in.Type+"/"+url.PathEscape(in.ID)+"?"+q.Encode(), &out)
					return out, err
				}),
		},
	})
}

const hubspotAPI = "https://api.hubapi.com"

func hubspotType(t string) error {
	if t != "contacts" && t != "companies" && t != "deals" {
		return fmt.Errorf("type is contacts, companies or deals, not %q", t)
	}
	return nil
}
