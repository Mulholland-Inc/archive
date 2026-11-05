package connect

import (
	"archive/zip"
	"io"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Amazon's privacy export, "Request your data" → Your Orders: one CSV row per item, in Retail.OrderHistory.N/ or, in newer
// exports, Your Orders/Order History.csv, with refunds in Retail.OrdersReturned. Headers drift between versions, so they are
// matched without case, spaces or punctuation. The first order file reads them all and groups rows into orders by website and
// order id; the other files then give nothing.

var (
	importAmazonState = regexp.MustCompile(`\b([A-Z]{2})\s+\d{5}(?:-\d{4})?\b`)
	importAmazonMoney = regexp.MustCompile(`-?\d[\d,]*(?:\.\d+)?`)
)

func importAmazonKind(base string) string {
	b := strings.ToLower(base)
	switch {
	case !strings.HasSuffix(b, ".csv"):
		return ""
	case strings.HasPrefix(b, "retail.orderhistory") || b == "order history.csv":
		return "orders"
	case strings.HasPrefix(b, "retail.ordersreturned") || strings.HasPrefix(b, "retail.customerreturns") || b == "returns.csv" || b == "refunds.csv":
		return "returns"
	}
	return ""
}

func importAmazonMatch(p, base string, head func() []byte) bool {
	h := importAmazonKey(string(head()))
	switch importAmazonKind(base) {
	case "orders":
		return strings.Contains(h, "orderid") && (strings.Contains(h, "asin") || strings.Contains(h, "productname"))
	case "returns":
		return strings.Contains(h, "orderid")
	}
	return false
}

func importAmazonKey(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

// importAmazonCol reads the first of names a row has, by its header without case, spaces or punctuation.
func importAmazonCol(row map[string]string, names ...string) string {
	for _, n := range names {
		if v := strings.Trim(row[n], `'" `); v != "" && !strings.EqualFold(v, "Not Available") && !strings.EqualFold(v, "Not Applicable") {
			return v
		}
	}
	return ""
}

func importAmazonNum(s string) (float64, bool) {
	m := importAmazonMoney.FindString(s)
	if m == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
	return f, err == nil
}

type importAmazonOrder struct {
	data                                  map[string]any
	items                                 []any
	total, shipping, discounts, refunded  float64
	hasTotal, hasShip, hasDisc, hasRefund bool
	payments, tracking, statuses          []string
}

func importAmazon(x *importRun, m *importMember, r io.Reader, _ int64) error {
	var sibs []*zip.File
	if m.zip != nil {
		for _, f := range m.zip.File {
			if importAmazonKind(path.Base(f.Name)) != "" {
				sibs = append(sibs, f)
			}
		}
		for _, f := range sibs {
			if importAmazonKind(path.Base(f.Name)) == "orders" {
				if f != m.f {
					return nil
				}
				break
			}
		}
	} else if importAmazonKind(m.baseName()) != "orders" {
		return nil
	}
	orders := map[string]*importAmazonOrder{}
	var order []string
	read := func(kind string, r io.Reader) error {
		return importCSV(r, func(raw map[string]string) bool {
			row := map[string]string{}
			for k, v := range raw {
				row[importAmazonKey(k)] = v
			}
			id := importAmazonCol(row, "orderid")
			if id == "" {
				return true
			}
			site := strings.ToLower(importAmazonCol(row, "website"))
			if site == "" {
				site = "amazon.com"
			}
			key := site + ":" + id
			o := orders[key]
			if o == nil {
				if kind != "orders" {
					return true
				}
				o = &importAmazonOrder{data: map[string]any{"website": site, "number": id}}
				orders[key] = o
				order = append(order, key)
			}
			add := func(list *[]string, v string) {
				if v != "" && !strings.Contains(strings.Join(*list, "\n"), v) {
					*list = append(*list, v)
				}
			}
			cur := importAmazonCol(row, "currency")
			if cur != "" && o.data["currency"] == nil {
				o.data["currency"] = cur
			}
			if kind == "returns" {
				if f, ok := importAmazonNum(importAmazonCol(row, "amountrefunded", "refundamount", "totalrefund", "refundedamount")); ok {
					o.refunded, o.hasRefund = o.refunded+f, true
				}
				return true
			}
			if t := importWhen(importAmazonCol(row, "orderdate", "orderdateutc")); t != "" && o.data["placed"] == nil {
				o.data["placed"] = t
			}
			owed := importAmazonCol(row, "totalowed", "itemtotal", "total")
			if f, ok := importAmazonNum(owed); ok {
				o.total, o.hasTotal = o.total+f, true
			}
			if f, ok := importAmazonNum(importAmazonCol(row, "shippingcharge")); ok {
				o.shipping, o.hasShip = o.shipping+f, true
			}
			if f, ok := importAmazonNum(importAmazonCol(row, "totaldiscounts", "discounts")); ok {
				o.discounts, o.hasDisc = o.discounts+math.Abs(f), true
			}
			add(&o.payments, importAmazonCol(row, "paymentinstrumenttype", "paymentmethod"))
			add(&o.tracking, importAmazonCol(row, "carriernametrackingnumber", "trackingnumber"))
			add(&o.statuses, importAmazonCol(row, "orderstatus", "shipmentstatus"))
			if a := importAmazonCol(row, "shippingaddress"); a != "" && o.data["address"] == nil {
				o.data["address"] = a
			}
			item := map[string]any{"title": importAmazonCol(row, "productname", "title", "itemname"), "asin": importAmazonCol(row, "asin", "asinisbn"),
				"quantity": importAmazonCol(row, "quantity", "originalquantity"), "price": importAmazonCol(row, "unitprice", "purchasepriceperunit", "itempriceperunit"),
				"tax": importAmazonCol(row, "unitpricetax", "itemsubtotaltax", "shipmentitemsubtotaltax"), "total": owed,
				"condition": importAmazonCol(row, "productcondition", "condition"), "shipped": importDay(importAmazonCol(row, "shipdate"))}
			for _, k := range []string{"price", "tax", "total"} {
				if v, ok := importAmazonNum(item[k].(string)); ok {
					item[k] = strconv.FormatFloat(v, 'f', -1, 64)
				}
			}
			o.items = append(o.items, item)
			return true
		})
	}
	if len(sibs) == 0 {
		if err := read("orders", r); err != nil {
			return err
		}
	}
	for _, kind := range []string{"orders", "returns"} {
		for _, f := range sibs {
			if importAmazonKind(path.Base(f.Name)) != kind {
				continue
			}
			fr, err := f.Open()
			if err != nil {
				return err
			}
			err = read(kind, fr)
			fr.Close()
			if err != nil {
				return err
			}
		}
	}
	money := func(f float64, ok bool) any {
		if !ok {
			return nil
		}
		return strconv.FormatFloat(f, 'f', 2, 64)
	}
	for _, k := range order {
		o := orders[k]
		d := o.data
		d["items"], d["total"], d["shipping"], d["discounts"], d["refunded"] = o.items, money(o.total, o.hasTotal),
			money(o.shipping, o.hasShip), money(o.discounts, o.hasDisc), money(o.refunded, o.hasRefund)
		d["payment"], d["tracking"], d["status"] = strings.Join(o.payments, ", "), o.tracking, strings.Join(o.statuses, ", ")
		if a, _ := d["address"].(string); a != "" {
			if s := importAmazonState.FindAllStringSubmatch(a, -1); len(s) > 0 {
				d["state"] = s[len(s)-1][1]
			}
			for _, c := range []string{"United States", "United Kingdom", "Canada", "Germany", "France", "Japan", "Australia", "Mexico", "India", "Italy", "Spain"} {
				if strings.HasSuffix(strings.TrimSpace(a), c) {
					d["country"] = c
				}
			}
			delete(d, "address")
		}
		if !x.put("amazon:"+k, d, 0) {
			return nil
		}
	}
	return nil
}

func importMoney(field string) SQL {
	return SQL("archive.sync_money(data->>'" + field + "', coalesce(data->>'currency', 'USD'))")
}
