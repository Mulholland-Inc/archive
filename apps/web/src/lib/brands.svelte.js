import { BRANDS } from '#kit/brands.js';

// A logo goes by a connector's name, as the API lists it, or a sign-in provider's, which the API lists by id alone.
let names = $state({ google: 'Google', okta: 'Okta', microsoft: 'Microsoft', entra: 'Microsoft Entra' });

export const nameBrands = (connectors) => {
  for (const c of connectors) names[c.id] = c.name;
};

export const brandFor = (id) => BRANDS[names[id]];
