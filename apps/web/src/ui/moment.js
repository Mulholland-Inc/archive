// The moment a story or memory is told, as a journal keeps it: the day, where you were and the weather.
// Only after you allow your location: the browser asks once, and the place leaves the device rounded to
// about a kilometre, to Open-Meteo for the weather and BigDataCloud for the town's name. Neither needs a key.

const SKY = {
  0: 'Clear', 1: 'Mostly clear', 2: 'Partly cloudy', 3: 'Overcast', 45: 'Fog', 48: 'Fog',
  51: 'Drizzle', 53: 'Drizzle', 55: 'Drizzle', 56: 'Freezing drizzle', 57: 'Freezing drizzle',
  61: 'Light rain', 63: 'Rain', 65: 'Heavy rain', 66: 'Freezing rain', 67: 'Freezing rain',
  71: 'Light snow', 73: 'Snow', 75: 'Heavy snow', 77: 'Snow', 80: 'Showers', 81: 'Showers', 82: 'Heavy showers',
  85: 'Snow showers', 86: 'Snow showers', 95: 'Thunderstorm', 96: 'Thunderstorm', 99: 'Thunderstorm',
};

/** The icon for a sky, from the WMO code Open-Meteo gives. */
export function skyIcon(code, day = true) {
  if (code === 0 || code === 1) return day ? 'sun' : 'moon';
  if (code === 2) return day ? 'cloud-sun' : 'cloud-moon';
  if (code === 45 || code === 48) return 'cloud-fog';
  if (code >= 95) return 'cloud-lightning';
  if ((code >= 71 && code <= 77) || code === 85 || code === 86) return 'cloud-snow';
  if (code >= 51) return 'cloud-rain';
  return 'cloud';
}

export function day(at = new Date()) {
  return at.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });
}

export function time(at = new Date()) {
  return at.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

const fahrenheit = () => /^en-US\b/.test(navigator.language || 'en-US');

/** True when the location was allowed before, so the place and weather can be filled in without asking. */
export async function allowed() {
  try {
    return (await navigator.permissions.query({ name: 'geolocation' })).state === 'granted';
  } catch {
    return false;
  }
}

function position() {
  return new Promise((ok, no) => {
    if (!navigator.geolocation) return no(new Error('Location is not available here'));
    navigator.geolocation.getCurrentPosition(ok, () => no(new Error('Location not allowed')), { maximumAge: 600000, timeout: 15000 });
  });
}

/** Where you are and the weather there now: { place, weather, icon }, each empty when it could not be found. */
export async function here() {
  const p = await position();
  const lat = p.coords.latitude.toFixed(2);
  const lon = p.coords.longitude.toFixed(2);
  const f = fahrenheit();
  const [town, sky] = await Promise.allSettled([
    fetch(`https://api.bigdatacloud.net/data/reverse-geocode-client?latitude=${lat}&longitude=${lon}&localityLanguage=en`).then((r) => r.json()),
    fetch(`https://api.open-meteo.com/v1/forecast?latitude=${lat}&longitude=${lon}&current=temperature_2m,weather_code,is_day${f ? '&temperature_unit=fahrenheit' : ''}`).then((r) => r.json()),
  ]);
  const t = town.status === 'fulfilled' ? town.value : {};
  const name = t.city || t.locality || '';
  const place = [name, t.principalSubdivision && t.principalSubdivision !== name ? t.principalSubdivision : t.countryName].filter(Boolean).join(', ');
  const now = sky.status === 'fulfilled' ? sky.value.current : null;
  const weather = now ? `${Math.round(now.temperature_2m)}°${f ? 'F' : 'C'}, ${SKY[now.weather_code] ?? 'Unsettled'}` : '';
  return { place, weather, icon: now ? skyIcon(now.weather_code, now.is_day !== 0) : 'cloud' };
}
