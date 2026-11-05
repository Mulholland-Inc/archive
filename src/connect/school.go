package connect

import "time"

func init() {
	for _, s := range []struct{ id, name, by, from, where, what string }{
		{"parentsquare", "ParentSquare", "ParentSquare", `@([^ @]+\.)?parentsquare\.com( |$)`, "from parentsquare.com",
			"Read the posts, alerts and reminders your schools send through ParentSquare"},
		{"powerschool", "PowerSchool", "PowerSchool", `powerschool`, "whose sender's address or name says PowerSchool",
			"Read the grade, attendance and report card notices PowerSchool sends you"},
		{"classdojo", "ClassDojo", "ClassDojo", `@([^ @]+\.)?classdojo\.com( |$)`, "from classdojo.com",
			"Read the messages, stories and reports your children's teachers send through ClassDojo"},
	} {
		Register(&Connector{
			ID: s.id, Name: s.name,
			Info: Info{By: s.by, Description: s.what, Category: CatHome, Icon: IconInbox,
				Scopes: []string{"Read the " + s.name + " mail in the mailboxes you synced to Archive; nothing is sent to " + s.name}},
			Keys: []Key{{Name: "students", Label: "Your children's first names",
				Help: "By commas, like Maya, Leo: each notice is matched to the child it names. " + s.name + " has no API for parents, so Archive reads its mail, which Gmail or Outlook must already sync"}},
			Sync: []*Sync{{Name: "notices", Gap: time.Second, Every: time.Hour, Keep: true,
				Mail: &Scan{From: s.from, Days: schoolDays}, Items: "messages", ID: "id",
				Set: Obj{"students": "{key/students}"},
				Doc: "Every message " + s.where + " in mail synced from your mailboxes in the last " + schoolDaysText + ", with its school, " +
					"which of your children it names, and what kind it is, like grades, attendance, an alert or an event: read again every hour, " +
					"and kept once the mail is older.",
				To: Map{Table: "school_notice",
					Cols: map[string]SQL{"service": SQL("'" + s.name + "'"), "school": "nullif(data->>'from_name', '')", "sender": "archive.sync_email(data->>'from')",
						"student": schoolStudent, "kind": schoolKind, "title": "data->>'subject'", "text": "data->>'text'",
						"sent_at": "archive.sync_at(data->>'sent_at')", "mail_message": "data->>'message'"}}}},
		})
	}
}

const (
	schoolDays     = 400
	schoolDaysText = "400 days"

	schoolStudent SQL = "(SELECT string_agg(n, ', ' ORDER BY n) FROM (SELECT DISTINCT btrim(n) AS n FROM unnest(string_to_array(data->>'students', ',')) n) x " +
		"WHERE n <> '' AND concat_ws(' ', data->>'subject', data->>'text') ~* ('\\m' || regexp_replace(n, '([^[:alnum:] ])', '\\\\\\1', 'g') || '\\M'))"
	schoolKind SQL = "CASE WHEN data->>'subject' ~* 'urgent|emergency|alert|clos(ed|ure)|lockdown|delay' THEN 'alert' " +
		"WHEN data->>'subject' ~* 'absen|attendance|tardy|late arrival' THEN 'attendance' " +
		"WHEN data->>'subject' ~* 'grade|report card|progress report|score|assignment|missing work' THEN 'grades' " +
		"WHEN data->>'subject' ~* 'payment|fee|invoice|lunch balance' THEN 'payment' " +
		"WHEN data->>'subject' ~* 'event|reminder|calendar|rsvp|sign.?up|conference|field trip|picture day' THEN 'event' " +
		"ELSE 'message' END"
)
