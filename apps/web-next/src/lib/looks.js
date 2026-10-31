export const TYPES = {
  personal: { label: 'Personal', icons: ['robot', 'chest', 'safe', 'bookcase', 'pinecone'] },
  family: { label: 'Family', icons: ['robot', 'cottage', 'castle', 'townhouse', 'farmhouse', 'oak', 'pine', 'baobab', 'pinecone'] },
  business: { label: 'Business', icons: ['robot', 'skyscraper', 'capitol', 'pyramid', 'ledger', 'cabinet', 'bank', 'warehouse', 'factory'] }
};

export const TINTS = 8;
export const KIND_TINTS = { business: 3, family: 0, personal: 1 };

export const ICON_LABELS = {
  robot: 'Robot', chest: 'Chest', safe: 'Safe', bookcase: 'Bookcase', pinecone: 'Pinecone', cottage: 'Cottage', castle: 'Castle',
  townhouse: 'Townhouse', farmhouse: 'Farmhouse', oak: 'Oak', pine: 'Pine', baobab: 'Baobab', skyscraper: 'Skyscraper',
  capitol: 'Capitol', pyramid: 'Pyramid', ledger: 'Ledger', cabinet: 'Cabinet', bank: 'Bank', warehouse: 'Warehouse', factory: 'Factory'
};

export const PLAIN = ['archivist', 'oak', 'pine', 'baobab', 'pinecone', 'skyscraper', 'capitol', 'ledger', 'castle', 'chest', 'pyramid'];

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
