import { BRANDS } from '#kit/brands.js';

let names = $state({ google: 'Google', okta: 'Okta', microsoft: 'Microsoft', entra: 'Microsoft Entra' });

export const nameBrands = (connectors) => {
  for (const c of connectors) names[c.id] = c.name;
};

export const brandFor = (id) => BRANDS[names[id]];
