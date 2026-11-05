package connect

import "time"

// Amazon has no API for a shopper's orders, so the connector reads the order confirmations Amazon mails, from mail Archive already
// synced; the privacy export, read by the import, has every item since the account began.
func init() {
	Register(&Connector{
		ID: "amazon", Name: "Amazon", Tap: true,
		Info: Info{By: "Amazon", Description: "Read your Amazon orders from the confirmations Amazon emails you", Category: CatCommerce, Icon: IconCard,
			Scopes: []string{"Read the Amazon order confirmations in the mailboxes you synced to Archive; nothing is sent to Amazon"}},
		Sync: []*Sync{{Name: "orders", Gap: time.Second, Every: time.Hour, Keep: true,
			Mail: &Scan{From: `auto-confirm@amazon\.[a-z.]+( |$)`, Days: amazonDays}, Items: "messages", ID: "id",
			Doc: "Every order Amazon confirmed by email in mail synced from your mailboxes in the last " + amazonDaysText + ": its order number, " +
				"when it was placed, what it came to, and the item its subject names: read again every hour, and kept once the mail is older. " +
				"Every item since the account began, with prices, tax and tracking, comes from importing Amazon's Your Orders export.",
			To: Map{Table: "commerce_order",
				Cols: map[string]SQL{"number": "substring(concat_ws(' ', data->>'subject', data->>'text') from '\\m(\\d{3}-\\d{7}-\\d{7})\\M')",
					"total": "archive.sync_money(replace(" + amazonTotal + ", ',', ''), " + amazonCurrency + ")",
					"placed_at": "archive.sync_at(data->>'sent_at')", "note": "data->>'subject'",
					"tags": "jsonb_build_array(lower(substring(data->>'from' from '@(amazon\\.[a-z.]+)$')))"},
				Children: []Kids{{Table: "commerce_line", From: "(SELECT substring(data->>'subject' from '(?:Ordered|Your Amazon[^:]*order of):?\\s*\"?(.+?)\"?(?:\\s+and \\d+ more items?)?\\.?$') AS t) l WHERE l.t IS NOT NULL",
					Cols: map[string]SQL{"title": "l.t"}}}}}},
	})
}

const (
	amazonDays     = 400
	amazonDaysText = "400 days"

	amazonTotal    = "substring(data->>'text' from '(?:Order Total|Grand Total|Total)[^0-9$£€]{0,40}[$£€]?\\s?([0-9][0-9,]*\\.[0-9]{2})')"
	amazonCurrency = "CASE substring(data->>'from' from '@amazon\\.([a-z.]+)$') WHEN 'co.uk' THEN 'GBP' WHEN 'ca' THEN 'CAD' WHEN 'com.mx' THEN 'MXN' " +
		"WHEN 'de' THEN 'EUR' WHEN 'fr' THEN 'EUR' WHEN 'it' THEN 'EUR' WHEN 'es' THEN 'EUR' WHEN 'co.jp' THEN 'JPY' WHEN 'in' THEN 'INR' ELSE 'USD' END"
)
