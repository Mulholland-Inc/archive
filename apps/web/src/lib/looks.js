export const TYPES = {
  personal: { label: 'Personal', icons: ['aframe', 'logcabin', 'apartment', 'beachhouse', 'bungalow', 'tent', 'caravan', 'cottage', 'shed', 'houseboat', 'hut', 'igloo', 'stilts', 'tipi', 'treehouse'] },
  family: { label: 'Family', icons: ['bungalow', 'cottage', 'castle', 'duplex', 'farmhouse', 'lighthouse', 'logcabin', 'manor', 'pagoda', 'palace', 'rowhouse', 'stilts', 'townhouse', 'treehouse', 'windmill', 'church', 'daycare', 'school', 'farm', 'greenhouse'] },
  business: { label: 'Business', icons: ['accounting', 'office', 'architect', 'bank', 'datacentre', 'insurance', 'lawfirm', 'marketing', 'realestate', 'recruiter', 'server', 'skyscraper', 'software', 'exchange', 'dental', 'hospital', 'clinic', 'pharmacy', 'vet', 'bakery', 'bar', 'brewery', 'butcher', 'cafe', 'diner', 'fishmonger', 'foodtruck', 'grocery', 'market', 'pizzeria', 'restaurant', 'winery', 'bookshop', 'boutique', 'florist', 'shop', 'hardware', 'jeweller', 'barbershop', 'gym', 'salon', 'spa', 'yoga', 'gallery', 'cinema', 'hotel', 'library', 'museum', 'music', 'photo', 'stadium', 'theatre', 'capitol', 'church', 'courthouse', 'daycare', 'nonprofit', 'school', 'townhall', 'university', 'garage', 'dealership', 'carwash', 'petrol', 'construction', 'factory', 'farm', 'silo', 'greenhouse', 'depot', 'mine', 'warehouse', 'workshop', 'hangar', 'airtower', 'bridge', 'port', 'station', 'oilwell', 'power', 'solar', 'telecom', 'watertower', 'windfarm'] }
};

export const TINTS = 32;
export const KIND_TINTS = { business: 17, family: 12, personal: 10 };

export const ICON_LABELS = {
  aframe: 'A-frame cabin',
  apartment: 'Apartment block',
  beachhouse: 'Beach house',
  bungalow: 'Bungalow',
  tent: 'Campsite',
  caravan: 'Caravan',
  castle: 'Castle',
  cottage: 'Cottage',
  duplex: 'Duplex',
  farmhouse: 'Farmhouse',
  shed: 'Garden shed',
  houseboat: 'Houseboat',
  hut: 'Hut',
  igloo: 'Igloo',
  lighthouse: 'Lighthouse',
  logcabin: 'Log cabin',
  manor: 'Manor',
  pagoda: 'Pagoda',
  palace: 'Palace',
  rowhouse: 'Row houses',
  stilts: 'Stilt house',
  tipi: 'Tipi',
  townhouse: 'Townhouse',
  treehouse: 'Treehouse',
  windmill: 'Windmill',
  accounting: 'Accounting office',
  architect: 'Architecture studio',
  bank: 'Bank',
  datacentre: 'Data centre',
  insurance: 'Insurance office',
  lawfirm: 'Law firm',
  marketing: 'Marketing agency',
  office: 'Office',
  realestate: 'Real estate agency',
  recruiter: 'Recruitment agency',
  server: 'Server building',
  skyscraper: 'Skyscraper',
  software: 'Software company',
  exchange: 'Stock exchange',
  dental: 'Dental office',
  hospital: 'Hospital',
  clinic: 'Medical clinic',
  pharmacy: 'Pharmacy',
  vet: 'Vet',
  bakery: 'Bakery',
  bar: 'Bar',
  brewery: 'Brewery',
  butcher: 'Butcher',
  cafe: 'Cafe',
  diner: 'Diner',
  fishmonger: 'Fishmonger',
  foodtruck: 'Food truck',
  grocery: 'Grocery',
  market: 'Market',
  pizzeria: 'Pizzeria',
  restaurant: 'Restaurant',
  winery: 'Winery',
  bookshop: 'Bookshop',
  boutique: 'Clothing boutique',
  florist: 'Florist',
  shop: 'General store',
  hardware: 'Hardware store',
  jeweller: 'Jeweller',
  barbershop: 'Barbershop',
  gym: 'Gym',
  salon: 'Hair salon',
  spa: 'Spa',
  yoga: 'Yoga studio',
  gallery: 'Art gallery',
  cinema: 'Cinema',
  hotel: 'Hotel',
  library: 'Library',
  museum: 'Museum',
  music: 'Music studio',
  photo: 'Photo studio',
  stadium: 'Stadium',
  theatre: 'Theatre',
  capitol: 'Capitol',
  church: 'Church',
  courthouse: 'Courthouse',
  daycare: 'Daycare',
  nonprofit: 'Nonprofit office',
  school: 'School',
  townhall: 'Town hall',
  university: 'University',
  garage: 'Auto garage',
  dealership: 'Car dealership',
  carwash: 'Car wash',
  petrol: 'Petrol station',
  construction: 'Construction yard',
  factory: 'Factory',
  farm: 'Farm',
  silo: 'Grain silo',
  greenhouse: 'Greenhouse',
  depot: 'Logistics depot',
  mine: 'Mine',
  warehouse: 'Warehouse',
  workshop: 'Workshop',
  hangar: 'Airport hangar',
  airtower: 'Airport tower',
  bridge: 'Bridge',
  port: 'Port',
  station: 'Train station',
  oilwell: 'Oil well',
  power: 'Power station',
  solar: 'Solar farm',
  telecom: 'Telecom mast',
  watertower: 'Water tower',
  windfarm: 'Wind farm'
};

// What an agent can look like: pre-rendered figures with eyes (/bots/turn-*), and the first is the default. The first
// list wakes with lit eyes and a panel; the rest have their eyes drawn on. An agent's row holds { figure }, and
// archive.agent_avatar() holds the same names.
const LIT = ['float', 'face', 'bank', 'bookcase', 'cabinet', 'cottage', 'factory', 'farmhouse', 'safe', 'townhouse', 'warehouse'];
const DRAWN = ['archivist', 'baobab', 'capitol', 'castle', 'chest', 'ledger', 'oak', 'pine', 'pinecone', 'pyramid', 'skyscraper'];
export const AVATARS = [...LIT, ...DRAWN];
export const lit = (avatar) => LIT.includes(avatar);
export const avatarOf = (avatar) => (AVATARS.includes(avatar?.figure) ? avatar.figure : AVATARS[0]);
export const AVATAR_LABELS = { float: 'Robot', face: 'Tall robot', bank: 'Bank', bookcase: 'Bookcase', cabinet: 'Cabinet', cottage: 'Cottage', factory: 'Factory', farmhouse: 'Farmhouse', safe: 'Safe', townhouse: 'Townhouse', warehouse: 'Warehouse', archivist: 'Archivist', baobab: 'Baobab', capitol: 'Capitol', castle: 'Castle', chest: 'Chest', ledger: 'Ledger', oak: 'Oak', pine: 'Pine', pinecone: 'Pinecone', pyramid: 'Pyramid', skyscraper: 'Skyscraper' };

export const PERSONALITIES = [
  { pace: 70, rest: 2600 },
  { pace: 55, rest: 1700 },
  { pace: 95, rest: 3400 },
  { pace: 65, rest: 2100 }
];

export function personality(seed) {
  return [...seed].reduce((n, c) => n + c.charCodeAt(0), 0) % PERSONALITIES.length;
}

export function defaultLook(type = 'personal') {
  return { type, icon: TYPES[type].icons[1], tint: KIND_TINTS[type] };
}

// An archive's look is its workspace's type, icon and colour, from the API; without an icon it offers or a colour it
// has its type's.
export function lookOf(w) {
  const type = TYPES[w.type] ? w.type : 'personal';
  const d = defaultLook(type);
  return { type, icon: Object.values(TYPES).some((t) => t.icons.includes(w.icon)) ? w.icon : d.icon, tint: w.color ?? d.tint };
}
