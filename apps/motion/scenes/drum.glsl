#pragma frame {"size":[1920,1080],"center":[0.0,0.4,0.0],"zoom":170,"bounds":[[-7,-2,-7],[7,5,7]],"t":3}

vec2 slab(vec3 p, float i) {
  float g = glide(0.7 + i * 0.08, 0.45);
  float d = mix(sdRoundCylinder(p, 1.0, 0.34, 0.02), sdRoundBox(p, vec3(1.3, 0.1, 0.95), 0.03), g);
  vec2 r = M(d, WHITE + (g < 0.5 ? ROUND : 0.0));
  float top = mix(0.34, 0.1, g) - 0.03;
  if (p.y < top) return r;
  if (g < 0.5) return paint(r, length(p.xz) - 0.93, LIME);
  float row = floor((p.z + 0.95) / 0.38);
  float hot = floor(mod(T * 5.0 + i * 1.7, 4.0)) + 1.0;
  r = paint(r, p.z + 0.57, BLUE);
  r = paint(r, row == hot ? -1.0 : 1.0, LILAC);
  if (abs(p.x) < 1.3 && abs(p.z) < 0.95) {
    r = stripes(r, p.z + 0.95, 0.19 * 2.0);
    if (p.z > -0.57) r = stripes(r, p.x + 1.3, 0.65);
  }
  return r;
}

vec2 map(vec3 p) {
  float lift = glide(0.1, 0.5), fan = glide(0.75, 0.6);
  vec2 r = M(1e9, 0.0);
  for (int k = 0; k < 3; k++) {
    float i = float(k) - 1.0;
    float bob = fan * 0.04 * sin(T * 4.0 + i * 2.0);
    vec3 c = vec3(i * 2.45 * fan, i * mix(mix(0.8, 1.15, lift), 0.5, fan) + bob, -i * 2.45 * fan);
    r = U(r, slab(p - c, float(k)));
  }
  for (int k = 0; k < 2; k++) {
    float i = float(k) - 1.0, s = glide(1.25 + i * -0.0 + float(k) * 0.28, 0.25);
    if (s <= 0.0) continue;
    vec3 a = vec3(i * 2.45 + 0.7, i * 0.5 + 0.13, -i * 2.45 - 0.5);
    vec3 b = vec3((i + 1.0) * 2.45 - 0.7, (i + 1.0) * 0.5 + 0.13, -(i + 1.0) * 2.45 + 0.5);
    r = U(r, M(sdCapsule(p, a, mix(a, b, s), 0.05), LIME + ROUND));
  }
  return r;
}
