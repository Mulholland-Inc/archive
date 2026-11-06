#pragma frame {"size":[1920,1080],"center":[0.0,2.6,0.0],"zoom":170,"bounds":[[-7,-0.5,-7],[7,14,7]],"t":1}

float hash(float n) { return fract(sin(n * 12.9898) * 43758.5453); }

vec2 thing(vec3 p, float k) {
  if (k < 0.5) {
    vec2 r = M(sdBox(p, vec3(0.42, 0.03, 0.56)), WHITE);
    r = paint(r, max(abs(p.x) - 0.28, abs(mod(p.z, 0.22) - 0.11) - 0.03), p.y > 0.0 ? STEEL : WHITE);
    return r;
  }
  if (k < 1.5) return U(M(sdChamferCylinder(p, 0.42, 0.1, 0.03), LILAC + ROUND), M(sdCylinder(p - vec3(0.0, 0.1, 0.0), 0.25, 0.012), BLUE + ROUND));
  if (k < 2.5) {
    vec2 r = M(max(sdEllipsoid(p, vec3(0.42, 0.42, 0.3)), -p.y), SKY + SOLID);
    return U(r, M(sdSphere(p - vec3(0.0, 0.62, 0.0), 0.21), WHITE + SOLID));
  }
  if (k < 3.5) {
    vec2 r = M(sdRoundBox(p, vec3(0.5, 0.34, 0.38), 0.02), LIME);
    return U(r, M(sdBox(p - vec3(-0.18, 0.36, 0.0), vec3(0.3, 0.04, 0.38)), LILAC));
  }
  vec2 r = M(sdBox(p, vec3(0.4, 0.3, 0.4)), WHITE);
  vec3 q = p - vec3(0.0, 0.3, 0.0);
  float gable = (0.4 * abs(q.x) + 0.5 * q.y - 0.2) / length(vec2(0.4, 0.5));
  return U(r, M(max(max(gable, -q.y), abs(q.z) - 0.44), BLUE));
}

vec2 map(vec3 p) {
  vec3 c = p - vec3(0.0, 0.0, 0.0);
  vec2 r = M(sdCylinder(c - vec3(0.0, 1.5, 0.0), 2.3, 1.5), WHITE + ROUND);
  r = stripes(r, p.y, 0.5);
  r.y = mod(r.y, 128.0);
  r.x = max(r.x, -sdCylinder(c - vec3(0.0, 3.0, 0.0), 2.1, 1.4));
  float level = 1.7 + T * 0.32;
  r = U(r, M(sdCylinder(c - vec3(0.0, level - 0.1, 0.0), 2.1, 0.1), LIME + ROUND));
  for (int i = 0; i < 14; i++) {
    float fi = float(i);
    float period = 1.15;
    float ph = fract(T / period + hash(fi * 3.1));
    float y = 14.0 - 13.0 * ph * ph - 0.0;
    float a = hash(fi + 0.7) * TAU, rad = 1.6 * sqrt(hash(fi + 2.3));
    vec3 q = p - vec3(cos(a) * rad, y, sin(a) * rad);
    if (length(q) > 1.6) { r.x = min(r.x, length(q) - 1.0); continue; }
    q = rotX(rotY(q, T * (2.0 + 3.0 * hash(fi))), T * (1.0 + 2.0 * hash(fi + 5.0)) + fi);
    r = U(r, thing(q, mod(fi, 5.0)));
  }
  return r;
}
