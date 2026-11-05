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
			Tap: true,
			Sync: []*Sync{{Name: "notices", Gap: time.Second, Every: time.Hour, Keep: true,
				Mail: &Scan{From: s.from, Days: schoolDays}, Items: "messages", ID: "id",
				Doc: "Every message " + s.where + " in mail synced from your mailboxes in the last " + schoolDaysText + ", with its school, " +
					"the child it names when it says, like Student: Maya, and what kind it is, like grades, attendance, an alert or an event: read again every hour, " +
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

	schoolStudent SQL = "substring(concat_ws(' ', data->>'subject', data->>'text') from '(?:Student(?: Name)?|Child):\\s*([[:upper:]][[:alpha:]''-]+(?: [[:upper:]][[:alpha:]''-]+)?)')"
	schoolKind    SQL = "CASE WHEN data->>'subject' ~* 'urgent|emergency|alert|clos(ed|ure)|lockdown|delay' THEN 'alert' " +
		"WHEN data->>'subject' ~* 'absen|attendance|tardy|late arrival' THEN 'attendance' " +
		"WHEN data->>'subject' ~* 'grade|report card|progress report|score|assignment|missing work' THEN 'grades' " +
		"WHEN data->>'subject' ~* 'payment|fee|invoice|lunch balance' THEN 'payment' " +
		"WHEN data->>'subject' ~* 'event|reminder|calendar|rsvp|sign.?up|conference|field trip|picture day' THEN 'event' " +
		"ELSE 'message' END"
)
