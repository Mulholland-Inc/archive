const F: Record<string, string> = Object.fromEntries((await archive.files({ limit: 1000 })).map(f => [f.name, f.id]));
let seed = 42;
const rnd = () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; };
const pick = <T,>(a: T[]): T => a[Math.floor(rnd() * a.length)];
const eur = (n: number) => ({ amount: Math.round(n * 100) / 100, currency: "EUR" });
const day = (d: Date) => d.toISOString().slice(0, 10);
const addDays = (s: string, n: number) => { const d = new Date(s + "T12:00:00Z"); d.setUTCDate(d.getUTCDate() + n); return day(d); };
const fam = { space: "family" };
const ams = (street: string, postcode: string) => ({ lines: [street], locality: "Amsterdam", postcode, country: "NL" });
const c: Record<string, number> = {};
const mk = async (t: string, d: any, o?: any) => { c[t] = (c[t] || 0) + 1; return (await archive.create(t as any, d, o)).id; };

const P: Record<string, string> = {};
const people: [string, string, any][] = [
  ["me", "Marijn Brussel", { relation: "other", birthday: "1993-05-18", email: "marijn@mulholland.ai", phone: "+31 6 2841 7730", address: ams("Lindengracht 112-2", "1015 KN"), notes: "Me. Blood type A+. Allergic to penicillin." }],
  ["sanne", "Sanne de Vries", { relation: "partner", birthday: "1994-11-09", email: "sanne@example.com", phone: "+31 6 1188 2049", address: ams("Lindengracht 112-2", "1015 KN"), notes: "Together since 2021. Loves ceramics, hates coriander. Ring size 52." }],
  ["mum", "Ingrid Brussel", { relation: "family", birthday: "1964-03-27", email: "ingrid.brussel@ziggo.nl", phone: "+31 6 5520 1187", address: { lines: ["Kerkstraat 8"], locality: "Haarlem", postcode: "2011 HP", country: "NL" }, notes: "Mum. Printer is always offline. Likes Ishiguro." }],
  ["dad", "Henk Brussel", { relation: "family", birthday: "1962-11-02", email: "henk.brussel@ziggo.nl", phone: "+31 6 5520 1190", address: { lines: ["Kerkstraat 8"], locality: "Haarlem", postcode: "2011 HP", country: "NL" }, notes: "Dad. Turns 64 on 2 Nov 2026. Into cycling and old Volvos." }],
  ["lotte", "Lotte Brussel", { relation: "family", birthday: "1997-07-14", email: "lotte.b@gmail.com", phone: "+31 6 3012 4478", address: { lines: ["Oudegracht 201"], locality: "Utrecht", postcode: "3511 NK", country: "NL" }, notes: "Sister. Vet student at Utrecht. Has a dog called Bram." }],
  ["els", "Els van Dijk", { relation: "family", birthday: "1959-01-30", notes: "Tante Els, mum's sister. Lives in Zandvoort." }],
  ["daan", "Daan Visser", { relation: "friend", birthday: "1992-02-21", email: "daan.visser@gmail.com", phone: "+31 6 4419 2210", notes: "Climbing buddy, Wednesdays at Monk. Reads sci-fi. Works at Adyen." }],
  ["noor", "Noor El Amrani", { relation: "friend", birthday: "1993-09-05", email: "noor.elamrani@proton.me", phone: "+31 6 2299 0815", notes: "From uni (TU Delft). Architect in Rotterdam. Godmother of nobody yet." }],
  ["joris", "Joris Kok", { relation: "friend", birthday: "1991-12-12", email: "joris@kok.studio", notes: "Designer, did the Archive logo. Owes me a beer." }],
  ["femke", "Femke Jansen", { relation: "neighbour", phone: "+31 6 2781 3304", notes: "3rd floor. Waters the plants and feeds Mosterd when we travel. Bring her stroopwafels." }],
  ["pieter", "Pieter Bakker", { relation: "service", email: "onderhoud@jordaanvastgoed.nl", phone: "+31 6 4412 8890", notes: "Maintenance contact at Jordaan Vastgoed. Slow to reply, call instead." }],
  ["huisarts", "Dr. Anouk Smit", { relation: "doctor", phone: "+31 20 624 1180", address: ams("Westerstraat 74", "1015 MN"), notes: "Huisarts (GP), Huisartsenpraktijk Jordaan." }],
  ["tandarts", "Dr. Ruben Mulder", { relation: "doctor", phone: "+31 20 682 3390", address: ams("Spaarndammerstraat 41", "1013 SV"), notes: "Dentist. Next check-up April 2027." }],
  ["fysio", "Mark de Groot", { relation: "doctor", phone: "+31 20 330 1245", notes: "Physio for my left knee, 9 sessions covered per year." }],
  ["jesse", "Jesse Hoekstra", { relation: "colleague", email: "jesse@mulholland.ai", notes: "Co-founder at Mulholland." }],
  ["mila", "Mila Verbeek", { relation: "colleague", email: "mila@mulholland.ai", birthday: "1996-06-03", notes: "Designer at Mulholland. Vegetarian." }],
  ["tom", "Tom Bos", { relation: "friend", birthday: "1993-04-17", email: "tombos@hey.com", notes: "Sanne's best friend's husband. Brews beer." }],
  ["kim", "Kim de Wit", { relation: "friend", birthday: "1994-08-22", email: "kim.dewit@gmail.com", notes: "Sanne's best friend since primary school. Wedding in June 2027 in Ghent." }],
];
for (const [k, name, d] of people) P[k] = await mk("person", { name, ...d }, ["me", "huisarts", "tandarts", "fysio", "jesse", "mila"].includes(k) ? undefined : fam);

const PL: Record<string, string> = {};
const places: [string, string, any][] = [
  ["home", "Home, Lindengracht 112-2", { kind: "home", address: ams("Lindengracht 112-2", "1015 KN"), location: { lat: 52.3812, lon: 4.8846 }, notes: "2nd floor. Bike shed key is the small blue one." }],
  ["parents", "Mum and dad's", { kind: "home", address: { lines: ["Kerkstraat 8"], locality: "Haarlem", postcode: "2011 HP", country: "NL" }, location: { lat: 52.3817, lon: 4.6366 } }],
  ["office", "Mulholland office", { kind: "office", address: ams("Keizersgracht 520", "1017 EK"), location: { lat: 52.3646, lon: 4.8889 } }],
  ["monk", "Monk Bouldergym", { kind: "gym", address: ams("Ankerweg 3", "1031 BB"), location: { lat: 52.3664, lon: 4.9259 }, rating: 5, notes: "Wednesdays with Daan. 10-ride card at the desk." }],
  ["wildezwijnen", "Restaurant Wilde Zwijnen", { kind: "restaurant", address: ams("Javaplein 23", "1095 CJ"), rating: 5, notes: "Anniversary place. Get the chef's menu." }],
  ["botanique", "Bar Botanique", { kind: "bar", address: ams("Eerste van Swindenstraat 581", "1093 LC"), rating: 4 }],
  ["lot61", "Lot Sixty One", { kind: "cafe", address: ams("Kinkerstraat 112", "1053 ED"), rating: 5, notes: "Best flat white in town." }],
  ["ah", "Albert Heijn Lindengracht", { kind: "shop", address: ams("Lindengracht 18", "1015 KE") }],
  ["noordermarkt", "Noordermarkt", { kind: "shop", address: ams("Noordermarkt", "1015 MV"), notes: "Saturday organic market. Mushroom stall and the cheese guy." }],
  ["huisarts", "Huisartsenpraktijk Jordaan", { kind: "doctor", address: ams("Westerstraat 74", "1015 MN") }],
  ["tandarts", "Tandartspraktijk Westerpark", { kind: "doctor", address: ams("Spaarndammerstraat 41", "1013 SV") }],
  ["westerpark", "Westerpark", { kind: "park", location: { lat: 52.3866, lon: 4.8743 }, notes: "Usual 6 km loop." }],
  ["vondelpark", "Vondelpark", { kind: "park", location: { lat: 52.3580, lon: 4.8686 } }],
  ["stedelijk", "Stedelijk Museum", { kind: "museum", address: ams("Museumplein 10", "1071 DJ"), website: "https://www.stedelijk.nl", rating: 4, notes: "Museumkaart works here." }],
  ["stadsloket", "Stadsloket Centrum", { kind: "office", address: ams("Amstel 1", "1011 PN"), notes: "Passport and ID. Appointment only." }],
  ["casagraca", "Casa da Graça", { kind: "hotel", address: { lines: ["Rua da Senhora do Monte 14"], locality: "Lisboa", postcode: "1170-361", country: "PT" }, location: { lat: 38.7194, lon: -9.1322 }, rating: 4 }],
  ["cafedeklos", "Café de Klos", { kind: "restaurant", address: ams("Kerkstraat 41", "1017 GB"), rating: 4, notes: "Spare ribs. Cash only, no reservations." }],
  ["pllek", "Pllek", { kind: "bar", address: ams("TT Neveritaweg 59", "1033 WB"), rating: 4, notes: "Sunset drinks in summer, take the ferry from Centraal." }],
];
for (const [k, name, d] of places) PL[k] = await mk("place", { name, ...d }, ["office", "huisarts", "tandarts"].includes(k) ? undefined : fam);

const A: Record<string, string> = {};
A.ing = await mk("bank_account", { name: "ING Betaalrekening", institution: "ING", kind: "checking", iban: "NL83 INGB 0004 3866 12", bic: "INGBNL2A", balance: eur(6412.18), opened: "2011-09-01" });
A.joint = await mk("bank_account", { name: "Joint account (rent and groceries)", institution: "ABN AMRO", kind: "checking", iban: "NL02ABNA0123456789", bic: "ABNANL2A", balance: eur(2210.55), opened: "2024-01-15" }, fam);
A.savings = await mk("bank_account", { name: "Spaarrekening", institution: "ING", kind: "savings", iban: "NL69INGB0123456789", balance: eur(18750), opened: "2015-03-02" });
A.meesman = await mk("bank_account", { name: "Meesman Aandelen Wereldwijd", institution: "Meesman", kind: "investment", balance: eur(31420.9), opened: "2019-06-10" });
A.pension = await mk("bank_account", { name: "Pension (Brand New Day)", institution: "Brand New Day", kind: "pension", balance: eur(14880), opened: "2022-01-01" });

await mk("payment_card", { name: "ING debit card", number: "5555 5555 5555 4444", expires: "09/29", account: A.ing, holder: P.me });
await mk("payment_card", { name: "Visa credit card", number: "4111 1111 1111 1111", expires: "03/28", account: A.ing, holder: P.me });
await mk("payment_card", { name: "Joint ABN debit card", number: "4000 0566 5566 5556", expires: "01/30", account: A.joint, holder: P.sanne }, fam);

const groceries = ["Albert Heijn 1427", "Albert Heijn 1427", "Jumbo Westerstraat", "Noordermarkt kaas", "Ekoplaza Jordaan", "Picnic B.V."];
const eating = ["Lot Sixty One", "Bar Botanique", "Café de Klos", "Restaurant Wilde Zwijnen", "Pllek", "Thuisbezorgd.nl", "De Pizzabakkers", "Foodhallen"];
const shops = ["Bol.com", "Coolblue B.V.", "HEMA", "Uniqlo Kalverstraat", "Decathlon", "IKEA Amsterdam", "Zalando SE", "Rituals"];
const months = ["2026-05", "2026-06", "2026-07", "2026-08", "2026-09", "2026-10"];
for (const m of months) {
  await mk("payment", { booked: `${m}-01`, amount: eur(-1970), counterparty: "Jordaan Vastgoed B.V.", description: `Huur ${m} Lindengracht 112-2`, category: "rent", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-02`, amount: eur(985), counterparty: "Sanne de Vries", description: "Haar helft huur", category: "income", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-02`, amount: eur(985), counterparty: "Marijn Brussel", description: "Mijn helft huur", category: "income", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-25`, amount: eur(4325.83), counterparty: "Mulholland B.V.", description: `Salaris ${m}`, category: "income", account: A.ing, receipt: m === "2026-09" ? F["Loonstrook september 2026.pdf"] : undefined });
  await mk("payment", { booked: `${m}-25`, amount: eur(-985), counterparty: "Joint account", description: "Huur helft", category: "rent", account: A.ing });
  await mk("payment", { booked: `${m}-26`, amount: eur(-500), counterparty: "Spaarrekening", description: "Monthly savings", category: "savings", account: A.ing });
  await mk("payment", { booked: `${m}-27`, amount: eur(-250), counterparty: "Meesman", description: "Inleg Aandelen Wereldwijd", category: "savings", account: A.ing });
  await mk("payment", { booked: `${m}-14`, amount: eur(-172), counterparty: "Vattenfall Klantenservice N.V.", description: "Termijnbedrag", category: "utilities", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-20`, amount: eur(-41.5), counterparty: "Ziggo Services B.V.", description: "Internet en TV", category: "utilities", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-25`, amount: eur(-21.37), counterparty: "Centraal Beheer", description: "Woonverzekering CB-77120459", category: "insurance", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-26`, amount: eur(-180.85), counterparty: "Zilveren Kruis Zorgverzekeringen", description: "Premie zorgverzekering", category: "insurance", account: A.ing });
  for (const [n, p, dd] of [["Netflix International B.V.", 13.99, 5], ["Spotify AB", 17.99, 7], ["Basic-Fit Nederland B.V.", 29.99, 15], ["NS Groep IZ NS Reizigers", 60 + rnd() * 40, 3], ["iCloud+ Apple", 2.99, 9], ["NRC Media", 19.5, 11]] as [string, number, number][])
    await mk("payment", { booked: `${m}-${String(dd).padStart(2, "0")}`, amount: eur(-p), counterparty: n, description: "Incasso", category: n.startsWith("NS") ? "transport" : "subscriptions", account: A.ing });
  for (let i = 0; i < 9; i++) await mk("payment", { booked: `${m}-${String(1 + Math.floor(rnd() * 28)).padStart(2, "0")}`, amount: eur(-(15 + rnd() * 80)), counterparty: pick(groceries), description: "Betaalautomaat", category: "groceries", account: A.joint }, fam);
  for (let i = 0; i < 6; i++) await mk("payment", { booked: `${m}-${String(1 + Math.floor(rnd() * 28)).padStart(2, "0")}`, amount: eur(-(8 + rnd() * 110)), counterparty: pick(eating), description: "Betaalautomaat", category: "eating_out", account: A.ing });
  for (let i = 0; i < 3; i++) await mk("payment", { booked: `${m}-${String(1 + Math.floor(rnd() * 28)).padStart(2, "0")}`, amount: eur(-(12 + rnd() * 140)), counterparty: pick(shops), description: "Online aankoop", category: "shopping", account: A.ing });
  for (let i = 0; i < 4; i++) await mk("payment", { booked: `${m}-${String(1 + Math.floor(rnd() * 28)).padStart(2, "0")}`, amount: eur(-(2.5 + rnd() * 12)), counterparty: pick(["GVB", "Swapfiets", "OV-chipkaart opladen", "Q-Park"]), description: "Reizen", category: "transport", account: A.ing });
}
await mk("payment", { booked: "2026-10-02", amount: eur(-448), counterparty: "Coolblue B.V.", description: "Order 46612980 Sony WH-1000XM6", category: "shopping", account: A.ing, receipt: F["2026-10-02 Coolblue invoice 7731042.pdf"] });
await mk("payment", { booked: "2026-10-06", amount: eur(-99.24), counterparty: "Tandartspraktijk Westerpark", description: "Factuur 2026-5512", category: "health", account: A.ing, receipt: F["Tandarts Westerpark factuur.pdf"] });
await mk("payment", { booked: "2026-10-21", amount: eur(-63.85), counterparty: "Albert Heijn 1427", description: "Betaalautomaat", category: "groceries", account: A.joint, receipt: F["Receipt Albert Heijn 2026-10-21.jpg"] }, fam);
await mk("payment", { booked: "2026-09-15", amount: eur(-486.4), counterparty: "KLM Royal Dutch Airlines", description: "Booking QX7R2M AMS-LIS", category: "travel", account: A.joint }, fam);
await mk("payment", { booked: "2026-08-08", amount: eur(612), counterparty: "Belastingdienst", description: "Teruggave IB 2025", category: "taxes", account: A.ing, receipt: F["Belastingdienst aanslag IB 2025.pdf"] });
await mk("payment", { booked: "2026-03-14", amount: eur(-3148), counterparty: "Apple Distribution International", description: "MacBook Pro + AppleCare", category: "shopping", account: A.ing, receipt: F["2026-03-14 Apple invoice MacBook Pro.pdf"] });
await mk("payment", { booked: "2026-07-18", amount: eur(-1240), counterparty: "Chamonix trip (various)", description: "Hotel, lift passes, food", category: "travel", account: A.ing });
await mk("payment", { booked: "2026-10-18", amount: eur(-125), counterparty: "Ace & Tate", description: "New glasses", category: "health", account: A.ing });

const subs: [string, number, string, string, string, any?][] = [
  ["Netflix Standard", 13.99, "FREQ=MONTHLY", "2026-11-05", "https://www.netflix.com/account"],
  ["Spotify Duo", 17.99, "FREQ=MONTHLY", "2026-11-07", "https://www.spotify.com/account"],
  ["Basic-Fit Comfort", 29.99, "FREQ=MONTHLY", "2026-11-15", "https://www.basic-fit.com", false],
  ["NS Flex Dal Voordeel", 5.6, "FREQ=MONTHLY", "2026-11-03", "https://www.ns.nl/mijnns"],
  ["iCloud+ 200 GB", 2.99, "FREQ=MONTHLY", "2026-11-09", "https://appleid.apple.com"],
  ["NRC digitaal", 19.5, "FREQ=MONTHLY", "2026-11-11", "https://www.nrc.nl/account"],
  ["Museumkaart", 75.9, "FREQ=YEARLY", "2027-02-14", "https://www.museum.nl"],
  ["1Password Families", 59.88, "FREQ=YEARLY", "2027-04-01", "https://my.1password.com"],
  ["Monk 10-ride card", 135, "FREQ=YEARLY", "2027-01-20", "https://monk.nl"],
  ["ANWB Wegenwacht", 89, "FREQ=YEARLY", "2027-05-01", "https://www.anwb.nl"],
];
for (const [name, p, billing, renews, website, active] of subs) await mk("subscription", { name, price: eur(p), billing, renews, website, account: A.ing, active: active ?? true });
await mk("subscription", { name: "Ziggo Internet 1 Gbit + TV", price: eur(41.5), billing: "FREQ=MONTHLY", renews: "2026-11-20", website: "https://www.ziggo.nl/mijn-ziggo", account: A.joint }, fam);
await mk("subscription", { name: "Swapfiets Deluxe 7 (Sanne)", price: eur(23.9), billing: "FREQ=MONTHLY", renews: "2026-11-01", website: "https://swapfiets.nl", account: A.joint }, fam);

await mk("identity_document", { kind: "passport", number: { country: "NL", number: "NXC4L8R27" }, holder: P.me, country: "NL", issued: "2017-02-08", expires: "2027-02-08", scan: F["Passport scan SPECIMEN.jpg"] });
await mk("identity_document", { kind: "id_card", number: { country: "NL", number: "IR9K22TP4" }, holder: P.me, country: "NL", issued: "2021-06-30", expires: "2031-06-30" });
await mk("identity_document", { kind: "drivers_license", number: { country: "NL", number: "521044839" }, holder: P.me, country: "NL", issued: "2012-09-14", expires: "2032-09-14" });
await mk("identity_document", { kind: "passport", number: { country: "NL", number: "NW7HJ1Q83" }, holder: P.sanne, country: "NL", issued: "2022-04-11", expires: "2032-04-11" }, fam);

await mk("insurance", { name: "Health insurance", provider: "Zilveren Kruis", kind: "health", policy_number: "204-881-3392", premium: eur(180.85), deductible: eur(385), renews: "2027-01-01", policy: F["Zilveren Kruis polisblad 2026.pdf"] });
await mk("insurance", { name: "Home contents and liability", provider: "Centraal Beheer", kind: "contents", policy_number: "CB-77120459", premium: eur(21.37), deductible: eur(100), renews: "2027-04-01", policy: F["Centraal Beheer inboedel en aansprakelijkheid.pdf"] }, fam);
await mk("insurance", { name: "Continuous travel insurance", provider: "Allianz Direct", kind: "travel", policy_number: "AD-TR-558120", premium: eur(6.2), renews: "2027-03-01" }, fam);
await mk("insurance", { name: "Legal aid", provider: "DAS", kind: "legal", policy_number: "DAS-90012284", premium: eur(16.8), deductible: eur(0), renews: "2027-06-01" });

const ev = (title: string, starts: string, hours: number, d: any = {}, o?: any) => mk("event", { title, starts, ends: new Date(new Date(starts).getTime() + hours * 3600e3).toISOString(), ...d }, o);
await ev("Sunday dinner at mum and dad's, dad turns 64", "2026-11-02T17:00:00Z", 3, { place: PL.parents, people: [P.mum, P.dad, P.sanne, P.els, P.lotte], notes: "Bring the Lisbon photo book and fix the printer." }, fam);
await ev("Carpenter for the bedroom window frame", "2026-11-03T07:00:00Z", 4, { place: PL.home, people: [P.pieter], notes: "Timmerbedrijf Vos, 8:00-12:00. Someone must be home." }, fam);
await ev("Passport renewal appointment", "2026-11-05T08:30:00Z", 0.25, { place: PL.stadsloket, notes: "Bring old passport, a passport photo and EUR 84." });
await ev("Bouldering with Daan", "2026-10-28T18:00:00Z", 2, { place: PL.monk, people: [P.daan], recurrence: "FREQ=WEEKLY;BYDAY=WE" });
await ev("Flight KL1691 to Lisbon", "2026-11-12T06:05:00Z", 2.9, { people: [P.sanne], notes: "Booking code QX7R2M. Schiphol 07:05." }, fam);
await ev("Flight KL1696 back to Amsterdam", "2026-11-16T18:10:00Z", 2.9, { people: [P.sanne], notes: "Lisbon 18:10, lands 22:05." }, fam);
await ev("Physio, left knee", "2026-10-30T15:00:00Z", 0.5, { people: [P.fysio] });
await ev("Dentist check-up", "2027-04-08T08:00:00Z", 0.5, { place: PL.tandarts, people: [P.tandarts] });
await ev("GP: blood test results", "2026-11-10T09:10:00Z", 0.2, { place: PL.huisarts, people: [P.huisarts] });
await ev("Dinner with Noor in Rotterdam", "2026-11-20T18:30:00Z", 3, { people: [P.noor] });
await ev("Mulholland offsite", "2026-12-03T08:00:00Z", 32, { people: [P.jesse, P.mila], notes: "Planning 2027. Bring the Archive roadmap." });
await ev("Sinterklaas at Lotte's", "2026-12-05T17:00:00Z", 4, { people: [P.lotte, P.mum, P.dad, P.sanne], notes: "Lootjes: I drew Lotte. Poem needed!" }, fam);
await ev("Kim and Tom's housewarming", "2026-11-28T19:00:00Z", 4, { people: [P.kim, P.tom, P.sanne] }, fam);
await ev("Anniversary dinner", "2026-12-18T19:00:00Z", 3, { place: PL.wildezwijnen, people: [P.sanne], notes: "5 years. Book by end of November." }, fam);
await ev("Sanne's birthday", "2026-11-09T00:00:00Z", 24, { people: [P.sanne], recurrence: "FREQ=YEARLY" }, fam);
await ev("Dad's birthday", "2026-11-02T00:00:00Z", 24, { people: [P.dad], recurrence: "FREQ=YEARLY" }, fam);
await ev("Stedelijk: new exhibition", "2026-10-11T12:00:00Z", 2, { place: PL.stedelijk, people: [P.sanne], notes: "Was great, the light installation room." }, fam);
await ev("Weekly 1:1 with Jesse", "2026-10-27T09:00:00Z", 1, { place: PL.office, people: [P.jesse], recurrence: "FREQ=WEEKLY;BYDAY=TU" });

const tasks: [string, string, string, string, string | null, any?][] = [
  ["Renew passport", "doing", "high", "admin", "2026-11-05", { notes: "Appointment booked at Stadsloket. Needed for... not Lisbon (ID card is fine) but for the US trip next year." }],
  ["Buy a birthday present for dad", "todo", "high", "social", "2026-11-01", { person: "dad" }],
  ["Buy a birthday present for Sanne", "todo", "urgent", "social", "2026-11-08", { person: "sanne", notes: "See gift ideas." }],
  ["Fix mum's printer", "todo", "normal", "social", "2026-11-02", { person: "mum" }],
  ["Book anniversary dinner at Wilde Zwijnen", "todo", "high", "social", "2026-11-30", { person: "sanne" }],
  ["Submit dentist invoice to Zilveren Kruis", "todo", "normal", "money", "2026-11-15", { notes: "Invoice 2026-5512, EUR 99,24." }],
  ["Decide on switching health insurer", "todo", "normal", "money", "2026-12-31", { notes: "Compare premiums before 31 Dec. Maybe drop dental." }],
  ["Cancel Basic-Fit", "done", "low", "money", null, { notes: "Going to Monk instead." }],
  ["File 2026 income tax return", "todo", "normal", "money", "2027-05-01"],
  ["Order the Lisbon photo book", "done", "normal", "home", "2026-10-15"],
  ["Ask Femke to feed Mosterd 12-16 Nov", "todo", "high", "home", "2026-11-08", { person: "femke" }],
  ["Chase landlord about window draft", "waiting", "normal", "home", null, { person: "pieter", notes: "Carpenter coming 3 Nov." }],
  ["Buy a reading chair", "todo", "low", "home", null, { notes: "Budget about EUR 600." }],
  ["Physio exercises every other day", "doing", "normal", "health", null],
  ["Get the blood test results", "todo", "normal", "health", "2026-11-10", { person: "huisarts" }],
  ["Run a sub-50 10k", "doing", "normal", "health", "2026-12-31"],
  ["Write Sinterklaas poem for Lotte", "todo", "normal", "social", "2026-12-04", { person: "lotte" }],
  ["Reply to Daan about book recommendation", "todo", "low", "social", null, { person: "daan" }],
  ["Prepare Archive roadmap for offsite", "doing", "high", "work", "2026-12-01", { person: "jesse" }],
  ["Read 'Thinking in Systems' notes again", "todo", "low", "learning", null],
  ["Learn basic Portuguese phrases", "doing", "low", "travel", "2026-11-12"],
  ["Check travel insurance covers hiking above 3000 m", "todo", "low", "travel", null],
  ["Back up photos to the NAS", "todo", "normal", "home", null],
  ["Return the Zalando jacket", "done", "low", "admin", "2026-10-10"],
];
for (const [title, status, priority, area, due, x] of tasks) {
  const o = x || {};
  await mk("task", { title, status, priority, area, due, notes: o.notes, person: o.person ? P[o.person] : undefined }, ["social", "home", "travel"].includes(area) ? fam : undefined);
}

const notes: [string, string, string[], boolean?][] = [
  ["Wi-Fi and house info", "## House\n- Meter cupboard is behind the coats\n- Stopcock under the kitchen sink\n- Bin day: Tuesday and Friday morning\n- Guest Wi-Fi is on the card on the fridge", ["home"], true],
  ["Lisbon plan", "## Lisbon 12-16 Nov\n- Thu: arrive, Graça, sunset at Senhora do Monte\n- Fri: Belém, pastéis, MAAT\n- Sat: day trip to Sintra (book Pena Palace tickets!)\n- Sun: LX Factory, Time Out market\n- Restaurants: Taberna da Rua das Flores, A Cevicheria", ["travel", "lisbon"], true],
  ["Gift ideas for Sanne", "- Ceramics course at Studio Klei\n- The Lisbon azulejo book\n- A good linen apron\n- Concert tickets: Fontaines D.C. in Paradiso", ["gifts", "sanne"]],
  ["Archive roadmap thoughts", "- Code mode is the agent interface\n- Real tables, declarative DDL\n- Next: views, sharing, mobile capture\n- Ask Jesse about pricing for teams", ["work", "archive"]],
  ["Knee rehab exercises", "1. Wall sits 3x45s\n2. Single-leg glute bridge 3x12\n3. Step-downs 3x10\n4. Bike 15 min easy\nIce after runs over 10 km.", ["health"]],
  ["Books to recommend to Daan", "- The Martian (if he somehow hasn't)\n- Children of Time\n- A Memory Called Empire\nNot too sad, he said.", ["books", "daan"]],
  ["Questions for the GP", "- Blood test: iron and vitamin D\n- Knee: referral to sports doctor?\n- Flu shot this year?", ["health"]],
  ["Insurance overview 2026", "| Policy | Monthly |\n|---|---|\n| Zilveren Kruis | 180,85 |\n| Centraal Beheer | 21,37 |\n| Allianz travel | 6,20 |\n| DAS legal | 16,80 |", ["money"]],
  ["Sourdough schedule", "- 9:00 feed starter\n- 13:00 mix, autolyse 1h\n- 14:00-17:00 stretch and fold every 30 min\n- Overnight fridge\n- Bake 250 °C, 20 min lid on, 25 off", ["cooking"]],
  ["Chamonix trip recap", "Lac Blanc hike was the highlight. The Aiguille du Midi cable car was too foggy. Next time: Tour du Mont Blanc in 2028?", ["travel", "chamonix"]],
  ["Moving checklist (just in case)", "- Notice to landlord: 1 month\n- Deposit back within 14 days\n- Transfer Ziggo, Vattenfall\n- Register at the new gemeente within 5 days", ["home"]],
  ["Car or no car?", "We use Greenwheels about twice a month, ~EUR 90. A second-hand car would be ~EUR 350/month all-in. Stay car-free for now.", ["money", "decisions"]],
];
for (const [title, body, tags, pinned] of notes) await mk("note", { title, body, tags, pinned: !!pinned }, tags.some(t => ["home", "travel", "gifts", "cooking"].includes(t)) ? fam : undefined);

const moods = [3, 4, 4, 5, 3, 2, 4, 5, 4, 3];
const bits = ["Long day at work, the schema diff finally works.", "Ran the Westerpark loop, knee felt fine.", "Dinner with Sanne at home, made the shakshuka.", "Bouldering with Daan, finally topped the purple one.", "Rainy. Stayed in and read.", "Bad night of sleep, grumpy all day.", "Called mum, dad is excited about his birthday.", "Shipped code mode. Haiku can drive the whole archive with one tool.", "Walked along the IJ, the light was ridiculous.", "Sanne's ceramics are getting really good.", "Admin day: insurance, passport appointment, invoices.", "Coffee at Lot Sixty One, sketched the Archive roadmap."];
for (let i = 0; i < 45; i++) await mk("journal", { day: addDays("2026-09-12", i), mood: pick(moods), entry: `${pick(bits)} ${pick(bits)}` });

const books: [string, string, string | null, string, number | null, string | null, string?][] = [
  ["The Overstory", "Richard Powers", "9780393635522", "read", 5, "2026-02-10", "Chapter structure like tree rings."],
  ["Project Hail Mary", "Andy Weir", "9780593135204", "read", 5, "2026-08-30", "Rocky! Recommended to Daan."],
  ["Thinking in Systems", "Donella Meadows", "9781603580557", "read", 4, "2026-06-14", "Leverage points."],
  ["Klara and the Sun", "Kazuo Ishiguro", "9780571364879", "read", 4, "2026-04-02", "Lent to mum."],
  ["Children of Time", "Adrian Tchaikovsky", "9781447273301", "reading", null, null],
  ["The Pragmatic Programmer", "David Thomas, Andrew Hunt", "9780135957059", "read", 4, "2025-11-20"],
  ["Designing Data-Intensive Applications", "Martin Kleppmann", "9781449373320", "reading", null, null, "Rereading the replication chapters."],
  ["A Philosophy of Software Design", "John Ousterhout", "9781732102217", "read", 5, "2025-09-01", "Deep modules."],
  ["Het smelt", "Lize Spit", null, "want", null, null],
  ["Tomorrow, and Tomorrow, and Tomorrow", "Gabrielle Zevin", "9780593321201", "read", 4, "2025-12-28"],
  ["The Midnight Library", "Matt Haig", "9781786892737", "abandoned", 2, null, "Not for me."],
  ["Piranesi", "Susanna Clarke", "9781526622426", "read", 5, "2025-08-15"],
  ["Sapiens", "Yuval Noah Harari", "9780099590088", "read", 3, "2024-07-01"],
  ["Four Thousand Weeks", "Oliver Burkeman", "9780374159122", "read", 4, "2026-01-05"],
  ["A Memory Called Empire", "Arkady Martine", "9781250186430", "want", null, null],
  ["The Dispossessed", "Ursula K. Le Guin", "9780060512750", "want", null, null],
  ["Educated", "Tara Westover", "9780399590504", "read", 4, "2024-10-12"],
  ["Shoe Dog", "Phil Knight", "9781501135910", "read", 4, "2025-03-03"],
  ["The Mom Test", "Rob Fitzpatrick", "9781492180746", "read", 5, "2026-05-20", "Required reading for the team."],
  ["Lisbon: Pocket Guide", "Lonely Planet", null, "reading", null, null],
];
for (const [title, author, isbn, status, rating, finished, n] of books) await mk("book", { title, author, isbn, status, rating, finished, notes: n });

const recipes: [string, string, number, number, string[], string, string[], string?][] = [
  ["Shakshuka with feta", "Middle Eastern", 30, 2, ["1 onion", "1 red pepper", "2 cloves garlic", "1 tsp cumin", "1 tsp smoked paprika", "400 g chopped tomatoes", "4 eggs", "75 g feta"], "Soften onion and pepper, add spices and tomatoes, simmer, crack in eggs, cover until set, top with feta.", ["vegetarian", "quick", "weeknight"], "https://example.com/shakshuka"],
  ["Dad's lamb stew", "Dutch", 180, 6, ["1.2 kg lamb shoulder", "3 onions", "4 carrots", "2 bay leaves", "500 ml stock", "1 tbsp tomato paste"], "Brown the lamb, soften onions, add everything, braise 2.5 hours at 160 °C.", ["family", "slow"]],
  ["Sourdough loaf", "Bread", 1440, 8, ["500 g bread flour", "350 g water", "100 g starter", "10 g salt"], "See the sourdough schedule note.", ["baking"]],
  ["Miso aubergine", "Japanese", 35, 2, ["2 aubergines", "2 tbsp white miso", "1 tbsp mirin", "1 tsp sugar", "sesame seeds", "spring onion"], "Score and roast aubergines 25 min, brush with miso glaze, grill 5 min.", ["vegetarian", "weeknight"]],
  ["Pasta alla vodka", "Italian", 25, 2, ["200 g rigatoni", "1 shallot", "2 tbsp tomato paste", "50 ml vodka", "100 ml cream", "parmesan", "chilli flakes"], "Fry shallot and paste, deglaze with vodka, add cream, toss with pasta and pasta water.", ["quick", "comfort"]],
  ["Erwtensoep", "Dutch", 150, 6, ["500 g split peas", "1 smoked sausage", "2 carrots", "1 leek", "1 celeriac", "2 potatoes", "1.5 l stock"], "Simmer peas in stock 1 hour, add vegetables, cook 45 min, slice in the sausage.", ["winter", "family"]],
  ["Green curry", "Thai", 40, 4, ["3 tbsp green curry paste", "400 ml coconut milk", "300 g chicken thigh", "1 courgette", "Thai basil", "fish sauce", "lime"], "Fry paste, add coconut milk and chicken, simmer, add veg, season with fish sauce and lime.", ["spicy"]],
  ["Overnight oats", "Breakfast", 5, 1, ["50 g oats", "150 ml milk", "1 tbsp chia", "berries", "honey"], "Mix, fridge overnight, top with berries.", ["breakfast", "quick"]],
  ["Pastéis de nata (attempt)", "Portuguese", 90, 12, ["1 roll puff pastry", "250 ml milk", "150 g sugar", "4 egg yolks", "1 cinnamon stick", "lemon peel"], "Make syrup and custard, line muffin tin with rolled pastry, bake at 275 °C for 10 min.", ["baking", "lisbon"]],
  ["Lentil dal", "Indian", 35, 4, ["250 g red lentils", "1 onion", "3 cloves garlic", "thumb of ginger", "1 tsp turmeric", "400 ml coconut milk", "spinach"], "Fry aromatics, add lentils, turmeric and liquid, simmer 20 min, stir in spinach.", ["vegetarian", "weeknight", "meal prep"]],
];
for (const [name, cuisine, minutes, servings, ingredients, steps, tags, source] of recipes) await mk("recipe", { name, cuisine, minutes, servings, ingredients, steps, tags, source }, fam);

await mk("trip", { name: "Lisbon long weekend", destination: "Lisbon", country: "PT", starts: "2026-11-12", ends: "2026-11-16", budget: eur(1600), people: [P.sanne], notes: "KLM QX7R2M, Casa da Graça (Booking 4410.882.915, free cancellation until 5 Nov). Sintra on Saturday." }, fam);
await mk("trip", { name: "Chamonix hiking", destination: "Chamonix", country: "FR", starts: "2026-07-18", ends: "2026-07-25", budget: eur(1500), people: [P.sanne], notes: "Lac Blanc! Cable car fogged out." }, fam);
await mk("trip", { name: "Lisbon and Porto", destination: "Lisbon", country: "PT", starts: "2025-05-01", ends: "2025-05-08", budget: eur(1400), people: [P.sanne], notes: "First time. Loved Graça, so going back." }, fam);
await mk("trip", { name: "Kim and Tom's wedding", destination: "Ghent", country: "BE", starts: "2027-06-11", ends: "2027-06-13", people: [P.sanne, P.kim, P.tom], notes: "Train, book hotel early." }, fam);
await mk("trip", { name: "US road trip (idea)", destination: "Utah and Arizona", country: "US", starts: "2027-09-10", ends: "2027-09-30", budget: eur(6000), people: [P.sanne], notes: "Needs new passport and ESTA." }, fam);
await mk("trip", { name: "Christmas in Haarlem", destination: "Haarlem", country: "NL", starts: "2026-12-24", ends: "2026-12-26", people: [P.mum, P.dad, P.lotte, P.sanne] }, fam);

let w = 76.8;
for (let i = 0; i < 120; i++) {
  const d = addDays("2026-06-29", i);
  if (i % 3 === 0) { w += (rnd() - 0.55) * 0.5; await mk("measurement", { taken: d, kind: "weight", value: Math.round(w * 10) / 10, unit: "kg" }); }
  await mk("measurement", { taken: d, kind: "sleep", value: Math.round((6 + rnd() * 2.5) * 10) / 10, unit: "h" });
  await mk("measurement", { taken: d, kind: "steps", value: Math.round(4000 + rnd() * 12000), unit: "steps" });
  if (i % 7 === 0) await mk("measurement", { taken: d, kind: "resting_heart_rate", value: Math.round(52 + rnd() * 8), unit: "bpm" });
  if (i % 2 === 0) await mk("measurement", { taken: d, kind: "run_distance", value: Math.round((4.5 + rnd() * 9.5) * 100) / 100, unit: "km", notes: pick(["Westerpark loop", "Amstel and back", "Vondelpark x2", "IJ waterfront", "Amsterdamse Bos long run"]) });
}
await mk("measurement", { taken: "2026-09-20", kind: "blood_pressure_systolic", value: 122, unit: "mmHg", notes: "At the pharmacy" });
await mk("measurement", { taken: "2026-09-20", kind: "blood_pressure_diastolic", value: 78, unit: "mmHg", notes: "At the pharmacy" });

const things: [string, string, string, string, number, string | null, string | null, string?][] = [
  ["Headphones", "Sony WH-1000XM6", "electronics", "2026-10-02", 448, "2029-10-05", "5521804", "2026-10-02 Coolblue invoice 7731042.pdf"],
  ["Laptop", "MacBook Pro 14 M5 Pro", "electronics", "2026-03-14", 3148, "2029-03-14", "C02ZK4M1Q6LT", "2026-03-14 Apple invoice MacBook Pro.pdf"],
  ["Camera", "Sony A7C II + 24-105 G", "electronics", "2025-04-02", 2899, "2027-04-02", "4419827"],
  ["Phone", "iPhone 17 Pro 256 GB", "electronics", "2026-09-19", 1329, "2027-09-19", "F2LZ81XKQ1"],
  ["Bike", "VanMoof S5", "bike", "2024-05-11", 2998, "2026-05-11", "VM-ASY5-88120"],
  ["Road bike", "Canyon Endurace CF 7", "bike", "2023-03-20", 2299, null, "WCA2210487"],
  ["Sofa", "Hay Mags 3-seater", "furniture", "2024-02-20", 2450, null, null],
  ["Coffee machine", "Lelit Mara X", "kitchen", "2025-01-10", 1399, "2027-01-10", "MX-552019"],
  ["Monitor", "LG UltraFine 27\" 5K", "electronics", "2026-04-02", 1299, "2028-04-02", "211NTXR9K882"],
  ["Ceramics wheel (Sanne)", "Shimpo VL-Whisper", "hobby", "2025-12-18", 1190, "2027-12-18", null],
  ["Vinyl turntable", "Rega Planar 3", "electronics", "2022-11-25", 849, null, "P3-990121"],
  ["Dishwasher", "Bosch Serie 6", "appliance", "2024-02-01", 749, "2026-02-01", "SMV6ZCX49E"],
];
for (const [name, brand, category, purchased, price, warranty_until, serial, receipt] of things)
  await mk("possession", { name, brand, category, purchased, price: eur(price), warranty_until, serial, receipt: receipt ? F[receipt] : undefined }, ["furniture", "kitchen", "appliance", "hobby"].includes(category) ? fam : undefined);

const gifts: [string, string, string, number | null, string, string?][] = [
  ["Ceramics course at Studio Klei", "sanne", "Birthday 2026", 245, "idea", "https://example.com/studio-klei"],
  ["Fontaines D.C. tickets, Paradiso", "sanne", "Birthday 2026", 98, "bought"],
  ["Linen apron", "sanne", "Sinterklaas", 45, "idea"],
  ["Vintage Volvo 240 model car", "dad", "Birthday 2026", 39, "bought"],
  ["Cycling jacket", "dad", "Christmas", 120, "idea"],
  ["Klara and the Sun (lent, maybe gift a copy)", "mum", "Christmas", 22, "idea"],
  ["Dog toy for Bram + vet book", "lotte", "Sinterklaas", 35, "idea"],
  ["Homebrew kit", "tom", "Housewarming", 60, "idea"],
  ["Stroopwafels", "femke", "Thank you for cat sitting", 8, "given"],
  ["Chalk bag", "daan", "Birthday 2027", 30, "idea"],
  ["Hand-made mug", "sanne", "Received for our 4th anniversary", null, "received"],
];
for (const [idea, who, occasion, price, status, link] of gifts) await mk("gift", { idea, person: P[who], occasion, price: price == null ? undefined : eur(price), status, link }, who === "sanne" ? undefined : fam);

return c;
