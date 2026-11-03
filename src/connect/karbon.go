package connect

import (
	"cmp"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Karbon, the practice-management platform of accounting firms, through its API v3. It has no sign-in for
// apps: a firm's admin registers an API application with Karbon, and once Karbon approves it, Settings,
// Connected Apps, API Applications shows its Authorization token, a GUID, and its AccessKey, a JWT, which an
// owner pastes in to connect the firm for everyone. Both go on every request.
//
// It syncs the firm's people and organizations, its clients and others, into crm_contact and crm_company, and
// its work items into work_item. Each is listed whole the first time and a week after, which deletes what is
// gone, and in between read newest change first down to the sync before. Karbon's webhooks need a subscription
// per kind, which it does not make yet: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "karbon", Name: "Karbon", Integration: true,
		Keys: []Key{
			{Name: "token", Label: "Authorization token", Secret: true,
				Help: "In Karbon, Settings, Connected Apps, API Applications, your application: the Bearer token, a code like 60a39912-382f-40a2-…"},
			{Name: "access_key", Label: "AccessKey", Secret: true, Help: "On the same page: the AccessKey, which starts with eyJ"},
		},
		Authorize: func(r *http.Request, c *Client) {
			r.Header.Set("Authorization", "Bearer "+c.Key("token"))
			r.Header.Set("AccessKey", c.Key("access_key"))
		},
		Account: func(c *Client) (string, error) {
			var t map[string]any
			if err := c.Get(karbonAPI+"/TenantSettings", &t); err != nil {
				return "", err
			}
			return cmp.Or(str(t, "TenantName"), str(t, "Name"), "Karbon"), nil
		},
		Actions: map[string]*Action{
			"get": Act("Read any of Karbon's GET endpoints, with OData's $filter, $orderby, $top and $expand, like { path: \"/WorkItems\", "+
				"query: \"$filter=WorkStatus eq 'In Progress'\" } or { path: \"/Organizations/<key>\", query: \"$expand=Contacts\" }: its answer, "+
				"{ value, @odata.nextLink } for a list (pass @odata.nextLink as page for more)",
				func(c *Client, in struct {
					Path  string `json:"path,omitempty" jsonschema:"The endpoint after /v3, like /Contacts, /Organizations, /WorkItems, /Users or /Notes"`
					Query string `json:"query,omitempty" jsonschema:"Its OData query, like $filter=contains(FullName,'Smith')&$top=50"`
					Page  string `json:"page,omitempty" jsonschema:"The @odata.nextLink of the call before"`
				}) (any, error) {
					u := in.Page
					if u == "" {
						if !strings.HasPrefix(in.Path, "/") {
							return nil, errors.New("path starts with /, like /Contacts")
						}
						u = karbonAPI + in.Path
						if in.Query != "" {
							u += "?" + in.Query
						}
					} else if !strings.HasPrefix(u, karbonAPI+"/") {
						return nil, errors.New("page is the @odata.nextLink of a call before")
					}
					var out any
					err := c.Get(u, &out)
					return out, err
				}),
		},
		Streams: []*Stream{
			karbonStream("organizations", "/Organizations", "OrganizationKey", "Every organization, client or not, with its name, website, email, phone and contact type",
				"crm_company", karbonOrgRows),
			karbonStream("contacts", "/Contacts", "ContactKey", "Every person, client or not, with their name, email, phone and contact type",
				"crm_contact", karbonContactRows),
			karbonStream("work_items", "/WorkItems", "WorkItemKey", "Every work item, with its title, type, status, client, assignee and dates",
				"work_item", karbonWorkRows),
		},
	})
}

const karbonAPI = "https://api.karbonhq.com/v3"

// karbonCursor is where a stream is: Since when the sync its records are as of began, Listed when its last full
// listing began, and while a sync is under way, Mode list or delta, Began when it began and Next its next page.
type karbonCursor struct {
	Since  string `json:"since,omitempty"`
	Listed string `json:"listed,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Began  string `json:"began,omitempty"`
	Next   string `json:"next,omitempty"`
}

// karbonStream is the stream of Karbon's list path, by the key field of its records, into table. A delta pages
// newest change first and stops at the first record no newer than the sync before; a list whose records say
// nothing of when they changed, or that will not sort by it, is listed whole each sync.
func karbonStream(name, path, key, what, table string, rows func(map[string]any) *Row) *Stream {
	return &Stream{Name: name, Gap: time.Second, Table: table, Rows: rows,
		Doc: what + ", by its " + key + ": listed whole the first time and a week after, 100 a page, which deletes those gone, and in " +
			"between those changed since the sync before, newest first.",
		Pull: func(c *Client, cursor string) (*Page, error) {
			var cur karbonCursor
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
				q := url.Values{"$top": {"100"}}
				if cur.Mode == "delta" {
					q.Set("$orderby", "LastModifiedDateTime desc")
				}
				u = karbonAPI + path + "?" + q.Encode()
			}
			var res struct {
				Value []map[string]any `json:"value"`
				Next  string           `json:"@odata.nextLink"`
			}
			if err := c.Get(u, &res); err != nil {
				if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == http.StatusBadRequest && cur.Mode == "delta" {
					cur.Mode, cur.Next = "list", ""
					return &Page{More: true, Cursor: cursorFor(cur)}, nil
				}
				return nil, err
			}
			page := &Page{Full: cur.Mode == "list"}
			done := false
			for _, m := range res.Value {
				changed := stamp(str(m, "LastModifiedDateTime"))
				if cur.Mode == "delta" {
					if changed.IsZero() {
						cur.Mode, cur.Next = "list", ""
						return &Page{More: true, Cursor: cursorFor(cur)}, nil
					}
					if !changed.After(stamp(cur.Since)) {
						done = true
						break
					}
				}
				page.Records = append(page.Records, Record{ID: str(m, key), Data: m, Updated: changed})
			}
			if res.Next != "" && !done {
				cur.Next, page.More = res.Next, true
			} else {
				if cur.Mode == "list" {
					cur.Listed = cur.Began
				}
				cur = karbonCursor{Since: cur.Began, Listed: cur.Listed}
			}
			page.Cursor = cursorFor(cur)
			return page, nil
		}}
}

// karbonOrgRows is an organization as a company, the organization of its website's domain.
func karbonOrgRows(m map[string]any) *Row {
	domain := Domain(str(m, "Website"))
	return rowOf(map[string]any{"title": str(m, "FullName"), "domain": domain, "stage": str(m, "ContactType"), "phone": str(m, "PhoneNumber"),
		"edited_at": instant(str(m, "LastModifiedDateTime"))}).org("org", domain)
}

// karbonContactRows is a person as a contact, the person of their email; Karbon names them whole, so the last
// word of their name is their last name.
func karbonContactRows(m map[string]any) *Row {
	full := strings.TrimSpace(cmp.Or(str(m, "FullName"), strings.TrimSpace(str(m, "FirstName")+" "+str(m, "LastName"))))
	first, last := full, ""
	if i := strings.LastIndex(full, " "); i > 0 {
		first, last = full[:i], full[i+1:]
	}
	email := Email(str(m, "EmailAddress"))
	return rowOf(map[string]any{"first_name": cmp.Or(str(m, "FirstName"), first), "last_name": cmp.Or(str(m, "LastName"), last), "email": email,
		"phone": str(m, "PhoneNumber"), "stage": str(m, "ContactType"), "edited_at": instant(str(m, "LastModifiedDateTime"))}).person("person", email)
}

// karbonWorkRows is a work item: its client and assignee, its status and its dates.
func karbonWorkRows(m map[string]any) *Row {
	return rowOf(map[string]any{"title": str(m, "Title"), "type": str(m, "WorkType"), "status": cmp.Or(str(m, "WorkStatus"), str(m, "PrimaryStatus")),
		"client_name": str(m, "ClientName"), "client_source": str(m, "ClientKey"), "assignee_name": str(m, "AssigneeName"),
		"starts_on": day(str(m, "StartDate")), "due_on": day(str(m, "DueDate")), "done_on": day(str(m, "CompletedDate")),
		"description": clip(str(m, "Description"), maxText), "edited_at": instant(str(m, "LastModifiedDateTime"))}).person("assignee", str(m, "AssigneeEmailAddress"))
}
