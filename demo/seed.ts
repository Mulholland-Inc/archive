const F: Record<string, string> = Object.fromEntries((await archive.files({ limit: 1000 })).map(f => [f.name, f.id]));
let seed = 42;
const rnd = () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; };
const pick = <T,>(a: T[]): T => a[Math.floor(rnd() * a.length)];
const usd = (n: number) => ({ amount: Math.round(n * 100) / 100, currency: "USD" });
const day = (d: Date) => d.toISOString().slice(0, 10);
const addDays = (s: string, n: number) => { const d = new Date(s + "T12:00:00Z"); d.setUTCDate(d.getUTCDate() + n); return day(d); };
const fam = { space: "family" };
const at = (street: string, locality: string, state: string, postcode: string) => ({ lines: [street], locality, region: `US-${state}`, postcode, country: "US" });
const bk = (street: string, postcode: string) => at(street, "Brooklyn", "NY", postcode);
const ny = (street: string, postcode: string) => at(street, "New York", "NY", postcode);
const c: Record<string, number> = {};
const mk = async (t: string, d: any, o?: any) => { c[t] = (c[t] || 0) + 1; return (await archive.create(t as any, d, o)).id; };

const home = bk("412 Union Street, Apt 3R", "11231");
const parents = at("27 Valley Road", "Montclair", "NJ", "07042");
const P: Record<string, string> = {};
const people: [string, string, any][] = [
  ["me", "Jordan Ellis", { relation: "other", birthday: "1993-05-18", email: "jordan.ellis@example.com", phone: "+1 718 555 0142", address: home, notes: "Me. Blood type A+. Allergic to penicillin." }],
  ["sam", "Sam Rivera", { relation: "partner", birthday: "1994-11-09", email: "sam@example.com", phone: "+1 718 555 0167", address: home, notes: "Together since 2021. Loves ceramics, hates cilantro. Ring size 6." }],
  ["mom", "Linda Ellis", { relation: "family", birthday: "1964-03-27", email: "linda.ellis@example.com", phone: "+1 973 555 0118", address: parents, notes: "Mom. Printer is always offline. Likes Ishiguro." }],
  ["dad", "Greg Ellis", { relation: "family", birthday: "1962-11-02", email: "greg.ellis@example.com", phone: "+1 973 555 0121", address: parents, notes: "Dad. Turns 64 on Nov 2, 2026. Into cycling and old Volvos." }],
  ["emma", "Emma Ellis", { relation: "family", birthday: "1997-07-14", email: "emma.ellis@gmail.com", phone: "+1 607 555 0139", address: at("118 Eddy Street", "Ithaca", "NY", "14850"), notes: "Sister. Vet student at Cornell. Has a dog called Biscuit." }],
  ["carol", "Carol Brennan", { relation: "family", birthday: "1959-01-30", notes: "Aunt Carol, mom's sister. Lives in Cape May." }],
  ["ben", "Ben Carter", { relation: "friend", birthday: "1992-02-21", email: "ben.carter@gmail.com", phone: "+1 347 555 0110", notes: "Climbing buddy, Wednesdays at Gowanus Bouldering. Reads sci-fi. Works at a payments startup." }],
  ["priya", "Priya Shah", { relation: "friend", birthday: "1993-09-05", email: "priya.shah@proton.me", phone: "+1 215 555 0181", notes: "From college (Cornell). Architect in Philadelphia. Godmother of nobody yet." }],
  ["owen", "Owen Park", { relation: "friend", birthday: "1991-12-12", email: "owen@park.studio", notes: "Designer, did the Archive logo. Owes me a beer." }],
  ["maria", "Maria Lopez", { relation: "neighbor", phone: "+1 718 555 0193", notes: "4th floor. Waters the plants and feeds Pickles when we travel. Bring her bagels." }],
  ["tony", "Tony Russo", { relation: "service", email: "repairs@unionstreetproperties.example", phone: "+1 347 555 0118", notes: "Super at Union Street Properties. Slow to reply, call instead." }],
  ["gp", "Dr. Rachel Kim", { relation: "doctor", phone: "+1 718 555 0124", address: bk("298 Court Street", "11231"), notes: "Primary care doctor, Cobble Hill Primary Care." }],
  ["dentist", "Dr. David Chen", { relation: "doctor", phone: "+1 718 555 0157", address: bk("211 Seventh Avenue", "11215"), notes: "Dentist. Next check-up April 2027." }],
  ["pt", "Kevin Walsh", { relation: "doctor", phone: "+1 718 555 0176", notes: "Physical therapist for my left knee, 20 visits covered per year." }],
  ["jesse", "Jesse Howard", { relation: "colleague", email: "jesse@mulholland.ai", notes: "Co-founder at Mulholland." }],
  ["mila", "Mila Brooks", { relation: "colleague", email: "mila@mulholland.ai", birthday: "1996-06-03", notes: "Designer at Mulholland. Vegetarian." }],
  ["tom", "Tom Becker", { relation: "friend", birthday: "1993-04-17", email: "tombecker@hey.com", notes: "Sam's best friend's husband. Brews beer." }],
  ["kim", "Kim Nguyen", { relation: "friend", birthday: "1994-08-22", email: "kim.nguyen@gmail.com", notes: "Sam's best friend since grade school. Wedding in June 2027 in Austin." }],
];
for (const [k, name, d] of people) P[k] = await mk("person", { name, ...d }, ["me", "gp", "dentist", "pt", "jesse", "mila"].includes(k) ? undefined : fam);

const PL: Record<string, string> = {};
const places: [string, string, any][] = [
  ["home", "Home, 412 Union Street", { kind: "home", address: home, location: { lat: 40.6796, lon: -73.9934 }, notes: "3rd floor. The bike room key is the small blue one." }],
  ["parents", "Mom and dad's", { kind: "home", address: parents, location: { lat: 40.8176, lon: -74.2121 } }],
  ["office", "Mulholland office", { kind: "office", address: ny("88 Spring Street", "10012"), location: { lat: 40.7228, lon: -73.9982 } }],
  ["gym", "Gowanus Bouldering", { kind: "gym", address: bk("210 3rd Avenue", "11217"), location: { lat: 40.6786, lon: -73.9881 }, rating: 5, notes: "Wednesdays with Ben. 10-visit punch card at the desk." }],
  ["rosemarys", "Rosemary's Table", { kind: "restaurant", address: bk("415 Court Street", "11231"), rating: 5, notes: "Anniversary place. Get the chef's tasting menu." }],
  ["linden", "Bar Linden", { kind: "bar", address: bk("181 Smith Street", "11201"), rating: 4 }],
  ["juniper", "Cafe Juniper", { kind: "cafe", address: bk("96 Union Street", "11231"), rating: 5, notes: "Best flat white in town." }],
  ["tj", "Trader Joe's Court Street", { kind: "shop", address: bk("130 Court Street", "11201") }],
  ["greenmarket", "Grand Army Plaza Greenmarket", { kind: "shop", address: bk("Prospect Park West and Flatbush Avenue", "11238"), notes: "Saturday farmers market. Mushroom stand and the cheese guy." }],
  ["gp", "Cobble Hill Primary Care", { kind: "doctor", address: bk("298 Court Street", "11231") }],
  ["dentist", "Park Slope Dental", { kind: "doctor", address: bk("211 Seventh Avenue", "11215") }],
  ["prospect", "Prospect Park", { kind: "park", location: { lat: 40.6602, lon: -73.9690 }, notes: "Usual 3.35 mile loop." }],
  ["bbp", "Brooklyn Bridge Park", { kind: "park", location: { lat: 40.7003, lon: -73.9967 } }],
  ["moma", "MoMA", { kind: "museum", address: ny("11 West 53rd Street", "10019"), website: "https://www.moma.org", rating: 4, notes: "Membership works here." }],
  ["postoffice", "Brooklyn Main Post Office", { kind: "office", address: bk("271 Cadman Plaza East", "11201"), notes: "Passport applications. Appointment only." }],
  ["casagraca", "Casa da Graça", { kind: "hotel", address: { lines: ["Rua da Senhora do Monte 14"], locality: "Lisboa", postcode: "1170-361", country: "PT" }, location: { lat: 38.7194, lon: -9.1322 }, rating: 4 }],
  ["smoke", "Smoke Joint", { kind: "restaurant", address: bk("87 South Elliott Place", "11217"), rating: 4, notes: "Ribs. Cash only, no reservations." }],
  ["piersix", "Pier Six Beer Garden", { kind: "bar", address: bk("Pier 6, Brooklyn Bridge Park", "11201"), rating: 4, notes: "Sunset drinks in summer, take the ferry from Wall Street." }],
];
for (const [k, name, d] of places) PL[k] = await mk("place", { name, ...d }, ["office", "gp", "dentist"].includes(k) ? undefined : fam);

const A: Record<string, string> = {};
A.chase = await mk("bank_account", { name: "Chase Total Checking", institution: "Chase", kind: "checking", routing: "021000021", last4: "6612", balance: usd(6412.18), opened: "2011-09-01" });
A.joint = await mk("bank_account", { name: "Joint account (rent and groceries)", institution: "Capital One", kind: "checking", routing: "031176110", last4: "3891", balance: usd(2210.55), opened: "2024-01-15" }, fam);
A.savings = await mk("bank_account", { name: "High-yield savings", institution: "Ally", kind: "savings", routing: "124003116", last4: "0457", balance: usd(18750), opened: "2015-03-02" });
A.vanguard = await mk("bank_account", { name: "Vanguard brokerage", institution: "Vanguard", kind: "investment", balance: usd(31420.9), opened: "2019-06-10" });
A.retirement = await mk("bank_account", { name: "401(k) (Fidelity)", institution: "Fidelity", kind: "retirement", balance: usd(48880), opened: "2022-01-01" });

await mk("payment_card", { name: "Chase debit card", number: "5555 5555 5555 4444", expires: "09/29", account: A.chase, holder: P.me });
await mk("payment_card", { name: "Visa credit card", number: "4111 1111 1111 1111", expires: "03/28", account: A.chase, holder: P.me });
await mk("payment_card", { name: "Joint Capital One debit card", number: "4000 0566 5566 5556", expires: "01/30", account: A.joint, holder: P.sam }, fam);

const groceries = ["Trader Joe's #558", "Trader Joe's #558", "Whole Foods Market", "Grand Army Greenmarket", "Key Food", "FreshDirect"];
const eating = ["Cafe Juniper", "Bar Linden", "Smoke Joint", "Rosemary's Table", "Pier Six Beer Garden", "DoorDash", "Joe's Pizza", "Time Out Market"];
const shops = ["Amazon.com", "Best Buy", "Target", "Uniqlo Fifth Avenue", "REI", "IKEA Brooklyn", "Zara", "Sephora"];
const months = ["2026-05", "2026-06", "2026-07", "2026-08", "2026-09", "2026-10"];
for (const m of months) {
  await mk("payment", { booked: `${m}-01`, amount: usd(-3200), counterparty: "Union Street Properties LLC", description: `Rent ${m} 412 Union St 3R`, category: "rent", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-02`, amount: usd(1600), counterparty: "Sam Rivera", description: "Sam's half of the rent", category: "income", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-02`, amount: usd(1600), counterparty: "Jordan Ellis", description: "My half of the rent", category: "income", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-25`, amount: usd(7466.42), counterparty: "Mulholland Inc.", description: `Payroll ${m}`, category: "income", account: A.chase, receipt: m === "2026-09" ? F["Pay stub September 2026.pdf"] : undefined });
  await mk("payment", { booked: `${m}-25`, amount: usd(-1600), counterparty: "Joint account", description: "Half of the rent", category: "rent", account: A.chase });
  await mk("payment", { booked: `${m}-26`, amount: usd(-800), counterparty: "Ally savings", description: "Monthly savings", category: "savings", account: A.chase });
  await mk("payment", { booked: `${m}-27`, amount: usd(-400), counterparty: "Vanguard", description: "Vanguard Total World Stock (VT)", category: "savings", account: A.chase });
  await mk("payment", { booked: `${m}-14`, amount: usd(-112), counterparty: "Con Edison", description: "Level payment", category: "utilities", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-20`, amount: usd(-69.99), counterparty: "Spectrum", description: "Internet", category: "utilities", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-25`, amount: usd(-14.25), counterparty: "Lemonade Insurance", description: "Renters policy LM-77120459", category: "insurance", account: A.joint }, fam);
  await mk("payment", { booked: `${m}-26`, amount: usd(-230.6), counterparty: "Aetna", description: "Health plan premium", category: "insurance", account: A.chase });
  for (const [n, p, dd] of [["Netflix.com", 15.49, 5], ["Spotify USA", 16.99, 7], ["Planet Fitness", 24.99, 15], ["MTA OMNY", 34 + rnd() * 30, 3], ["Apple iCloud+", 2.99, 9], ["The New York Times", 25, 11]] as [string, number, number][])
    await mk("payment", { booked: `${m}-${String(dd).padStart(2, "0")}`, amount: usd(-p), counterparty: n, description: "Autopay", category: n.startsWith("MTA") ? "transport" : "subscriptions", account: A.chase });
  for (let i = 0; i < 9; i++) await mk("payment", { booked: `${m}-${String(1 + Math.floor(rnd() * 28)).padStart(2, "0")}`, amount: usd(-(20 + rnd() * 110)), counterparty: pick(groceries), description: "Card purchase", category: "groceries", account: A.joint }, fam);
  for (let i = 0; i < 6; i++) await mk("payment", { booked: `${m}-${String(1 + Math.floor(rnd() * 28)).padStart(2, "0")}`, amount: usd(-(10 + rnd() * 140)), counterparty: pick(eating), description: "Card purchase", category: "eating_out", account: A.chase });
  for (let i = 0; i < 3; i++) await mk("payment", { booked: `${m}-${String(1 + Math.floor(rnd() * 28)).padStart(2, "0")}`, amount: usd(-(15 + rnd() * 170)), counterparty: pick(shops), description: "Online order", category: "shopping", account: A.chase });
  for (let i = 0; i < 4; i++) await mk("payment", { booked: `${m}-${String(1 + Math.floor(rnd() * 28)).padStart(2, "0")}`, amount: usd(-(2.9 + rnd() * 25)), counterparty: pick(["Citi Bike", "Uber", "MTA OMNY", "NYC Ferry"]), description: "Travel", category: "transport", account: A.chase });
}
await mk("payment", { booked: "2026-10-02", amount: usd(-500.8), counterparty: "Best Buy", description: "Order 7731042 Sony WH-1000XM6", category: "shopping", account: A.chase, receipt: F["2026-10-02 Best Buy receipt 7731042.pdf"] });
await mk("payment", { booked: "2026-10-06", amount: usd(-320), counterparty: "Park Slope Dental", description: "Invoice 2026-5512", category: "health", account: A.chase, receipt: F["Park Slope Dental invoice.pdf"] });
await mk("payment", { booked: "2026-10-21", amount: usd(-63.85), counterparty: "Trader Joe's #558", description: "Card purchase", category: "groceries", account: A.joint, receipt: F["Receipt Trader Joe's 2026-10-21.jpg"] }, fam);
await mk("payment", { booked: "2026-09-15", amount: usd(-1286.4), counterparty: "TAP Air Portugal", description: "Booking QX7R2M JFK-LIS", category: "travel", account: A.joint }, fam);
await mk("payment", { booked: "2026-08-08", amount: usd(612), counterparty: "IRS Treasury", description: "2025 federal tax refund", category: "taxes", account: A.chase, receipt: F["IRS refund notice 2025.pdf"] });
await mk("payment", { booked: "2026-03-14", amount: usd(-3568.92), counterparty: "Apple", description: "MacBook Pro + AppleCare", category: "shopping", account: A.chase, receipt: F["2026-03-14 Apple invoice MacBook Pro.pdf"] });
await mk("payment", { booked: "2026-07-18", amount: usd(-1340), counterparty: "Chamonix trip (various)", description: "Hotel, lift passes, food", category: "travel", account: A.chase });
await mk("payment", { booked: "2026-10-18", amount: usd(-145), counterparty: "Warby Parker", description: "New glasses", category: "health", account: A.chase });

const subs: [string, number, string, string, string, any?][] = [
  ["Netflix Standard", 15.49, "FREQ=MONTHLY", "2026-11-05", "https://www.netflix.com/account"],
  ["Spotify Duo", 16.99, "FREQ=MONTHLY", "2026-11-07", "https://www.spotify.com/account"],
  ["Planet Fitness Classic", 24.99, "FREQ=MONTHLY", "2026-11-15", "https://www.planetfitness.com", false],
  ["OMNY 30-day unlimited", 34, "FREQ=MONTHLY", "2026-11-03", "https://omny.info"],
  ["iCloud+ 200 GB", 2.99, "FREQ=MONTHLY", "2026-11-09", "https://appleid.apple.com"],
  ["The New York Times", 25, "FREQ=MONTHLY", "2026-11-11", "https://www.nytimes.com/account"],
  ["MoMA membership", 120, "FREQ=YEARLY", "2027-02-14", "https://www.moma.org"],
  ["1Password Families", 59.88, "FREQ=YEARLY", "2027-04-01", "https://my.1password.com"],
  ["Gowanus Bouldering 10-visit card", 180, "FREQ=YEARLY", "2027-01-20", "https://example.com/gowanus-bouldering"],
  ["AAA Plus", 119, "FREQ=YEARLY", "2027-05-01", "https://www.aaa.com"],
];
for (const [name, p, billing, renews, website, active] of subs) await mk("subscription", { name, price: usd(p), billing, renews, website, account: A.chase, active: active ?? true });
await mk("subscription", { name: "Spectrum Internet 1 Gig", price: usd(69.99), billing: "FREQ=MONTHLY", renews: "2026-11-20", website: "https://www.spectrum.net", account: A.joint }, fam);
await mk("subscription", { name: "Citi Bike annual (Sam)", price: usd(219.99), billing: "FREQ=YEARLY", renews: "2027-04-01", website: "https://citibikenyc.com", account: A.joint }, fam);

await mk("identity_document", { kind: "passport", number: { country: "US", number: "A38214907" }, holder: P.me, country: "US", issued: "2017-02-08", expires: "2027-02-08", scan: F["Passport scan SPECIMEN.jpg"] });
await mk("identity_document", { kind: "drivers_license", number: { country: "US", number: "521044839" }, holder: P.me, country: "US", issued: "2024-05-18", expires: "2032-05-18" });
await mk("identity_document", { kind: "passport", number: { country: "US", number: "A57720931" }, holder: P.sam, country: "US", issued: "2022-04-11", expires: "2032-04-11" }, fam);
await mk("identity_document", { kind: "drivers_license", number: { country: "US", number: "731552908" }, holder: P.sam, country: "US", issued: "2023-11-09", expires: "2031-11-09" }, fam);

await mk("insurance", { name: "Health insurance", provider: "Aetna", kind: "health", policy_number: "W204881339", premium: usd(230.6), deductible: usd(1500), renews: "2027-01-01", policy: F["Aetna plan summary 2026.pdf"] });
await mk("insurance", { name: "Renters insurance", provider: "Lemonade", kind: "contents", policy_number: "LM-77120459", premium: usd(14.25), deductible: usd(500), renews: "2027-04-01", policy: F["Lemonade renters insurance policy.pdf"] }, fam);
await mk("insurance", { name: "Annual travel insurance", provider: "Allianz Travel", kind: "travel", policy_number: "AZ-TR-558120", premium: usd(21), renews: "2027-03-01" }, fam);
await mk("insurance", { name: "Legal plan", provider: "ARAG", kind: "legal", policy_number: "ARAG-90012284", premium: usd(16.8), deductible: usd(0), renews: "2027-06-01" });

const ev = (title: string, starts: string, hours: number, d: any = {}, o?: any) => mk("event", { title, starts, ends: new Date(new Date(starts).getTime() + hours * 3600e3).toISOString(), ...d }, o);
await ev("Sunday dinner at mom and dad's, dad turns 64", "2026-11-01T22:00:00Z", 3, { place: PL.parents, people: [P.mom, P.dad, P.sam, P.carol, P.emma], notes: "Bring the Lisbon photo book and fix the printer." }, fam);
await ev("Carpenter for the bedroom window frame", "2026-11-03T13:00:00Z", 4, { place: PL.home, people: [P.tony], notes: "Brooklyn Carpentry Co., 8 AM to noon. Someone must be home." }, fam);
await ev("Passport appointment", "2026-11-05T14:30:00Z", 0.5, { place: PL.postoffice, notes: "Bring the old passport, a passport photo, a check for $130 and $35 for the fee." });
await ev("Bouldering with Ben", "2026-10-28T23:00:00Z", 2, { place: PL.gym, people: [P.ben], recurrence: "FREQ=WEEKLY;BYDAY=WE" });
await ev("Flight TP208 to Lisbon", "2026-11-12T01:55:00Z", 7, { people: [P.sam], notes: "Booking code QX7R2M. JFK Terminal 1, 8:55 PM." }, fam);
await ev("Flight TP209 back to New York", "2026-11-16T11:30:00Z", 7.75, { people: [P.sam], notes: "Lisbon 11:30 AM, lands at JFK 2:15 PM." }, fam);
await ev("Physical therapy, left knee", "2026-10-30T20:00:00Z", 0.75, { people: [P.pt] });
await ev("Dentist check-up", "2027-04-08T13:00:00Z", 0.75, { place: PL.dentist, people: [P.dentist] });
await ev("Doctor: blood test results", "2026-11-10T14:10:00Z", 0.25, { place: PL.gp, people: [P.gp] });
await ev("Dinner with Priya in Philadelphia", "2026-11-20T23:30:00Z", 3, { people: [P.priya] });
await ev("Mulholland offsite", "2026-12-03T14:00:00Z", 32, { people: [P.jesse, P.mila], notes: "Planning 2027. Bring the Archive roadmap." });
await ev("Thanksgiving at mom and dad's", "2026-11-26T19:00:00Z", 6, { place: PL.parents, people: [P.emma, P.mom, P.dad, P.sam, P.carol], notes: "I'm bringing the pie. Secret Santa draw after dinner." }, fam);
await ev("Kim and Tom's housewarming", "2026-11-29T00:00:00Z", 4, { people: [P.kim, P.tom, P.sam] }, fam);
await ev("Anniversary dinner", "2026-12-19T00:00:00Z", 3, { place: PL.rosemarys, people: [P.sam], notes: "5 years. Book by the end of November." }, fam);
await ev("Sam's birthday", "2026-11-09T05:00:00Z", 24, { people: [P.sam], recurrence: "FREQ=YEARLY" }, fam);
await ev("Dad's birthday", "2026-11-02T05:00:00Z", 24, { people: [P.dad], recurrence: "FREQ=YEARLY" }, fam);
await ev("MoMA: new exhibition", "2026-10-11T16:00:00Z", 2, { place: PL.moma, people: [P.sam], notes: "Was great, the light installation room." }, fam);
await ev("Weekly 1:1 with Jesse", "2026-10-27T14:00:00Z", 1, { place: PL.office, people: [P.jesse], recurrence: "FREQ=WEEKLY;BYDAY=TU" });

const tasks: [string, string, string, string, string | null, any?][] = [
  ["Renew passport", "doing", "high", "admin", "2026-11-05", { notes: "Appointment booked at the Brooklyn post office. Still valid for Lisbon, but needed for the Japan trip next year." }],
  ["Buy a birthday present for dad", "todo", "high", "social", "2026-10-31", { person: "dad" }],
  ["Buy a birthday present for Sam", "todo", "urgent", "social", "2026-11-08", { person: "sam", notes: "See gift ideas." }],
  ["Fix mom's printer", "todo", "normal", "social", "2026-11-01", { person: "mom" }],
  ["Book anniversary dinner at Rosemary's Table", "todo", "high", "social", "2026-11-30", { person: "sam" }],
  ["Submit the dentist invoice to Aetna", "todo", "normal", "money", "2026-11-15", { notes: "Invoice 2026-5512, $320.00." }],
  ["Pick a health plan in open enrollment", "todo", "normal", "money", "2026-12-15", { notes: "Compare premiums before Dec 15. Maybe drop vision." }],
  ["Cancel Planet Fitness", "done", "low", "money", null, { notes: "Going to Gowanus Bouldering instead." }],
  ["File 2026 income tax return", "todo", "normal", "money", "2027-04-15"],
  ["Order the Lisbon photo book", "done", "normal", "home", "2026-10-15"],
  ["Ask Maria to feed Pickles Nov 11-16", "todo", "high", "home", "2026-11-08", { person: "maria" }],
  ["Chase the landlord about the window draft", "waiting", "normal", "home", null, { person: "tony", notes: "Carpenter coming Nov 3." }],
  ["Buy a reading chair", "todo", "low", "home", null, { notes: "Budget about $600." }],
  ["Physical therapy exercises every other day", "doing", "normal", "health", null],
  ["Get the blood test results", "todo", "normal", "health", "2026-11-10", { person: "gp" }],
  ["Run a sub-50 10K", "doing", "normal", "health", "2026-12-31"],
  ["Buy a Secret Santa gift for Emma", "todo", "normal", "social", "2026-12-20", { person: "emma" }],
  ["Reply to Ben about a book recommendation", "todo", "low", "social", null, { person: "ben" }],
  ["Prepare Archive roadmap for offsite", "doing", "high", "work", "2026-12-01", { person: "jesse" }],
  ["Read 'Thinking in Systems' notes again", "todo", "low", "learning", null],
  ["Learn basic Portuguese phrases", "doing", "low", "travel", "2026-11-11"],
  ["Check travel insurance covers hiking above 10,000 ft", "todo", "low", "travel", null],
  ["Back up photos to the NAS", "todo", "normal", "home", null],
  ["Return the Zara jacket", "done", "low", "admin", "2026-10-10"],
];
for (const [title, status, priority, area, due, x] of tasks) {
  const o = x || {};
  await mk("task", { title, status, priority, area, due, notes: o.notes, person: o.person ? P[o.person] : undefined }, ["social", "home", "travel"].includes(area) ? fam : undefined);
}

const notes: [string, string, string[], boolean?][] = [
  ["Wi-Fi and house info", "## House\n- Breaker panel is behind the coats\n- Water shutoff under the kitchen sink\n- Trash and recycling: Tuesday and Friday morning\n- Guest Wi-Fi is on the card on the fridge", ["home"], true],
  ["Lisbon plan", "## Lisbon Nov 12-16\n- Thu: arrive, Graça, sunset at Senhora do Monte\n- Fri: Belém, pastéis, MAAT\n- Sat: day trip to Sintra (book Pena Palace tickets!)\n- Sun: LX Factory, Time Out Market\n- Restaurants: Taberna da Rua das Flores, A Cevicheria", ["travel", "lisbon"], true],
  ["Gift ideas for Sam", "- Pottery class at a Brooklyn studio\n- The Lisbon azulejo book\n- A good linen apron\n- Concert tickets: Fontaines D.C. at Brooklyn Steel", ["gifts", "sam"]],
  ["Archive roadmap thoughts", "- Code mode is the agent interface\n- Real tables, declarative DDL\n- Next: views, sharing, mobile capture\n- Ask Jesse about pricing for teams", ["work", "archive"]],
  ["Knee rehab exercises", "1. Wall sits 3x45s\n2. Single-leg glute bridge 3x12\n3. Step-downs 3x10\n4. Bike 15 min easy\nIce after runs over 6 miles.", ["health"]],
  ["Books to recommend to Ben", "- The Martian (if he somehow hasn't)\n- Children of Time\n- A Memory Called Empire\nNot too sad, he said.", ["books", "ben"]],
  ["Questions for the doctor", "- Blood test: iron and vitamin D\n- Knee: referral to a sports medicine doctor?\n- Flu shot this year?", ["health"]],
  ["Insurance overview 2026", "| Policy | Monthly |\n|---|---|\n| Aetna health | $230.60 |\n| Lemonade renters | $14.25 |\n| Allianz travel | $21.00 |\n| ARAG legal | $16.80 |", ["money"]],
  ["Sourdough schedule", "- 9:00 feed starter\n- 13:00 mix, autolyse 1h\n- 14:00-17:00 stretch and fold every 30 min\n- Overnight fridge\n- Bake at 480 °F, 20 min lid on, 25 off", ["cooking"]],
  ["Chamonix trip recap", "Lac Blanc hike was the highlight. The Aiguille du Midi cable car was too foggy. Next time: Tour du Mont Blanc in 2028?", ["travel", "chamonix"]],
  ["Moving checklist (just in case)", "- Notice to the landlord: 60 days\n- Deposit back within 14 days\n- Transfer Spectrum and Con Edison\n- Change the address with USPS and the DMV within 10 days", ["home"]],
  ["Car or no car?", "We use Zipcar about twice a month, about $120. A used car with parking in Brooklyn would be about $900 a month all-in. Stay car-free for now.", ["money", "decisions"]],
];
for (const [title, body, tags, pinned] of notes) await mk("note", { title, body, tags, pinned: !!pinned }, tags.some(t => ["home", "travel", "gifts", "cooking"].includes(t)) ? fam : undefined);

const moods = [3, 4, 4, 5, 3, 2, 4, 5, 4, 3];
const bits = ["Long day at work, the schema diff finally works.", "Ran the Prospect Park loop, knee felt fine.", "Dinner with Sam at home, made the shakshuka.", "Bouldering with Ben, finally topped the purple one.", "Rainy. Stayed in and read.", "Bad night of sleep, grumpy all day.", "Called mom, dad is excited about his birthday.", "Shipped code mode. Haiku can drive the whole archive with one tool.", "Walked along the East River, the light was ridiculous.", "Sam's ceramics are getting really good.", "Admin day: insurance, passport appointment, invoices.", "Coffee at Cafe Juniper, sketched the Archive roadmap."];
for (let i = 0; i < 45; i++) await mk("journal", { day: addDays("2026-09-12", i), mood: pick(moods), entry: `${pick(bits)} ${pick(bits)}` });

const books: [string, string, string | null, string, number | null, string | null, string?][] = [
  ["The Overstory", "Richard Powers", "9780393635522", "read", 5, "2026-02-10", "Chapter structure like tree rings."],
  ["Project Hail Mary", "Andy Weir", "9780593135204", "read", 5, "2026-08-30", "Rocky! Recommended to Ben."],
  ["Thinking in Systems", "Donella Meadows", "9781603580557", "read", 4, "2026-06-14", "Leverage points."],
  ["Klara and the Sun", "Kazuo Ishiguro", "9780571364879", "read", 4, "2026-04-02", "Lent to mom."],
  ["Children of Time", "Adrian Tchaikovsky", "9781447273301", "reading", null, null],
  ["The Pragmatic Programmer", "David Thomas, Andrew Hunt", "9780135957059", "read", 4, "2025-11-20"],
  ["Designing Data-Intensive Applications", "Martin Kleppmann", "9781449373320", "reading", null, null, "Rereading the replication chapters."],
  ["A Philosophy of Software Design", "John Ousterhout", "9781732102217", "read", 5, "2025-09-01", "Deep modules."],
  ["The Heaven & Earth Grocery Store", "James McBride", null, "want", null, null],
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
  ["Shakshuka with feta", "Middle Eastern", 30, 2, ["1 onion", "1 red bell pepper", "2 cloves garlic", "1 tsp cumin", "1 tsp smoked paprika", "1 can crushed tomatoes (14 oz)", "4 eggs", "3 oz feta"], "Soften onion and pepper, add spices and tomatoes, simmer, crack in eggs, cover until set, top with feta.", ["vegetarian", "quick", "weeknight"], "https://example.com/shakshuka"],
  ["Dad's lamb stew", "American", 180, 6, ["2.5 lb lamb shoulder", "3 onions", "4 carrots", "2 bay leaves", "2 cups stock", "1 tbsp tomato paste"], "Brown the lamb, soften onions, add everything, braise 2.5 hours at 325 °F.", ["family", "slow"]],
  ["Sourdough loaf", "Bread", 1440, 8, ["4 cups bread flour", "1.5 cups water", "1/2 cup starter", "2 tsp salt"], "See the sourdough schedule note.", ["baking"]],
  ["Miso eggplant", "Japanese", 35, 2, ["2 eggplants", "2 tbsp white miso", "1 tbsp mirin", "1 tsp sugar", "sesame seeds", "scallion"], "Score and roast the eggplants 25 min, brush with miso glaze, broil 5 min.", ["vegetarian", "weeknight"]],
  ["Pasta alla vodka", "Italian", 25, 2, ["8 oz rigatoni", "1 shallot", "2 tbsp tomato paste", "1/4 cup vodka", "1/2 cup heavy cream", "parmesan", "chili flakes"], "Fry shallot and paste, deglaze with vodka, add cream, toss with pasta and pasta water.", ["quick", "comfort"]],
  ["Split pea soup", "American", 150, 6, ["1 lb split peas", "1 ham hock", "2 carrots", "1 leek", "2 celery stalks", "2 potatoes", "6 cups stock"], "Simmer peas and ham hock in stock 1 hour, add vegetables, cook 45 min, pull the ham off the bone.", ["winter", "family"]],
  ["Green curry", "Thai", 40, 4, ["3 tbsp green curry paste", "1 can coconut milk", "10 oz chicken thigh", "1 zucchini", "Thai basil", "fish sauce", "lime"], "Fry paste, add coconut milk and chicken, simmer, add veg, season with fish sauce and lime.", ["spicy"]],
  ["Overnight oats", "Breakfast", 5, 1, ["1/2 cup oats", "2/3 cup milk", "1 tbsp chia", "berries", "honey"], "Mix, fridge overnight, top with berries.", ["breakfast", "quick"]],
  ["Pastéis de nata (attempt)", "Portuguese", 90, 12, ["1 sheet puff pastry", "1 cup milk", "3/4 cup sugar", "4 egg yolks", "1 cinnamon stick", "lemon peel"], "Make syrup and custard, line a muffin tin with rolled pastry, bake at 525 °F for 10 min.", ["baking", "lisbon"]],
  ["Lentil dal", "Indian", 35, 4, ["1 1/4 cups red lentils", "1 onion", "3 cloves garlic", "thumb of ginger", "1 tsp turmeric", "1 can coconut milk", "spinach"], "Fry aromatics, add lentils, turmeric and liquid, simmer 20 min, stir in spinach.", ["vegetarian", "weeknight", "meal prep"]],
];
for (const [name, cuisine, minutes, servings, ingredients, steps, tags, source] of recipes) await mk("recipe", { name, cuisine, minutes, servings, ingredients, steps, tags, source }, fam);

await mk("trip", { name: "Lisbon long weekend", destination: "Lisbon", country: "PT", starts: "2026-11-11", ends: "2026-11-16", budget: usd(2400), people: [P.sam], notes: "TAP QX7R2M, Casa da Graça (Booking 4410.882.915, free cancellation until Nov 5). Sintra on Saturday." }, fam);
await mk("trip", { name: "Chamonix hiking", destination: "Chamonix", country: "FR", starts: "2026-07-18", ends: "2026-07-25", budget: usd(1800), people: [P.sam], notes: "Lac Blanc! Cable car fogged out." }, fam);
await mk("trip", { name: "Lisbon and Porto", destination: "Lisbon", country: "PT", starts: "2025-05-01", ends: "2025-05-08", budget: usd(1700), people: [P.sam], notes: "First time. Loved Graça, so going back." }, fam);
await mk("trip", { name: "Kim and Tom's wedding", destination: "Austin", country: "US", starts: "2027-06-11", ends: "2027-06-13", people: [P.sam, P.kim, P.tom], notes: "Book flights and the hotel early." }, fam);
await mk("trip", { name: "Japan (idea)", destination: "Tokyo and Kyoto", country: "JP", starts: "2027-09-10", ends: "2027-09-30", budget: usd(7000), people: [P.sam], notes: "Needs a new passport." }, fam);
await mk("trip", { name: "Christmas in Montclair", destination: "Montclair", country: "US", starts: "2026-12-24", ends: "2026-12-26", people: [P.mom, P.dad, P.emma, P.sam] }, fam);

let w = 169.3;
for (let i = 0; i < 120; i++) {
  const d = addDays("2026-06-29", i);
  if (i % 3 === 0) { w += (rnd() - 0.55) * 1.1; await mk("measurement", { taken: d, kind: "weight", value: Math.round(w * 10) / 10, unit: "lb" }); }
  await mk("measurement", { taken: d, kind: "sleep", value: Math.round((6 + rnd() * 2.5) * 10) / 10, unit: "h" });
  await mk("measurement", { taken: d, kind: "steps", value: Math.round(4000 + rnd() * 12000), unit: "steps" });
  if (i % 7 === 0) await mk("measurement", { taken: d, kind: "resting_heart_rate", value: Math.round(52 + rnd() * 8), unit: "bpm" });
  if (i % 2 === 0) await mk("measurement", { taken: d, kind: "run_distance", value: Math.round((2.8 + rnd() * 5.9) * 100) / 100, unit: "mi", notes: pick(["Prospect Park loop", "Brooklyn Bridge and back", "Prospect Park x2", "East River esplanade", "Hudson River Greenway long run"]) });
}
await mk("measurement", { taken: "2026-09-20", kind: "blood_pressure_systolic", value: 122, unit: "mmHg", notes: "At the pharmacy" });
await mk("measurement", { taken: "2026-09-20", kind: "blood_pressure_diastolic", value: 78, unit: "mmHg", notes: "At the pharmacy" });

const things: [string, string, string, string, number, string | null, string | null, string?][] = [
  ["Headphones", "Sony WH-1000XM6", "electronics", "2026-10-02", 500.8, "2029-10-02", "5521804", "2026-10-02 Best Buy receipt 7731042.pdf"],
  ["Laptop", "MacBook Pro 14 M5 Pro", "electronics", "2026-03-14", 3568.92, "2029-03-14", "C02ZK4M1Q6LT", "2026-03-14 Apple invoice MacBook Pro.pdf"],
  ["Camera", "Sony A7C II + 24-105 G", "electronics", "2025-04-02", 2899, "2027-04-02", "4419827"],
  ["Phone", "iPhone 17 Pro 256 GB", "electronics", "2026-09-19", 1199, "2027-09-19", "F2LZ81XKQ1"],
  ["Bike", "Trek FX 3 Disc", "bike", "2024-05-11", 1149, "2026-05-11", "WTU288C1477"],
  ["Road bike", "Canyon Endurace CF 7", "bike", "2023-03-20", 2299, null, "WCA2210487"],
  ["Sofa", "West Elm Harmony 3-seater", "furniture", "2024-02-20", 2450, null, null],
  ["Coffee machine", "Lelit Mara X", "kitchen", "2025-01-10", 1599, "2027-01-10", "MX-552019"],
  ["Monitor", "LG UltraFine 27\" 5K", "electronics", "2026-04-02", 1299, "2028-04-02", "211NTXR9K882"],
  ["Pottery wheel (Sam)", "Shimpo VL-Whisper", "hobby", "2025-12-18", 1190, "2027-12-18", null],
  ["Turntable", "Rega Planar 3", "electronics", "2022-11-25", 849, null, "P3-990121"],
  ["Air purifier", "Coway Airmega 400", "appliance", "2024-02-01", 549, "2026-02-01", "AP-2121HH-0042"],
];
for (const [name, brand, category, purchased, price, warranty_until, serial, receipt] of things)
  await mk("possession", { name, brand, category, purchased, price: usd(price), warranty_until, serial, receipt: receipt ? F[receipt] : undefined }, ["furniture", "kitchen", "appliance", "hobby"].includes(category) ? fam : undefined);

const gifts: [string, string, string, number | null, string, string?][] = [
  ["Pottery class at a Brooklyn studio", "sam", "Birthday 2026", 245, "idea", "https://example.com/pottery-class"],
  ["Fontaines D.C. tickets, Brooklyn Steel", "sam", "Birthday 2026", 98, "bought"],
  ["Linen apron", "sam", "Christmas", 45, "idea"],
  ["Vintage Volvo 240 model car", "dad", "Birthday 2026", 39, "bought"],
  ["Cycling jacket", "dad", "Christmas", 120, "idea"],
  ["Klara and the Sun (lent, maybe gift a copy)", "mom", "Christmas", 22, "idea"],
  ["Dog toy for Biscuit + vet book", "emma", "Secret Santa", 35, "idea"],
  ["Homebrew kit", "tom", "Housewarming", 60, "idea"],
  ["Bagels and coffee", "maria", "Thank you for cat sitting", 18, "given"],
  ["Chalk bag", "ben", "Birthday 2027", 30, "idea"],
  ["Hand-made mug", "sam", "Received for our 4th anniversary", null, "received"],
];
for (const [idea, who, occasion, price, status, link] of gifts) await mk("gift", { idea, person: P[who], occasion, price: price == null ? undefined : usd(price), status, link }, who === "sam" ? undefined : fam);

return c;
