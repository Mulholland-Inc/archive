#pragma frame {"size":[1920,1080],"center":[0.0,0.2,0.0],"zoom":250,"bounds":[[-5,-0.5,-3],[5,3,3]],"t":3}

float road(vec2 q, vec2 a, vec2 b, float at) {
  float s = glide(at, 0.3);
  if (s < 0.001) return 1.0;
  vec2 e = mix(a, b, s);
  float d = sdSeg(q, a, e) - 0.035;
  float l = dot(q - a, normalize(b - a));
  return max(d, 0.09 - abs(mod(l, 0.36) - 0.18));
}

const vec2 A = vec2(-2.3, -0.75), B = vec2(-0.6, 0.85), C = vec2(0.3, -0.85), D = vec2(1.75, 0.55), E = vec2(2.4, -0.95);

vec2 sheet(vec3 f) {
  vec2 r = M(sdBox(f, vec3(2.85, 0.03, 1.65)), MINT);
  if (f.y < 0.0) return r;
  float land = smin(smin(sdCircle(f.xz - A, 0.75), sdCircle(f.xz - B, 0.85), 0.8), smin(sdCircle(f.xz - C, 0.75), smin(sdCircle(f.xz - D, 0.8), sdCircle(f.xz - E, 0.65), 0.9), 0.9), 0.5);
  r = paint(r, land, SKY);
  r = seam(r, land);
  float rd = min(min(road(f.xz, A, B, 0.75), road(f.xz, B, C, 0.95)), min(road(f.xz, C, D, 1.15), min(road(f.xz, D, E, 1.35), road(f.xz, B, D, 1.5))));
  r = paint(r, rd, BLUE);
  return r;
}

vec2 person(vec3 p) {
  vec2 r = M(max(sdEllipsoid(p, vec3(0.3, 0.42, 0.22)), -p.y), LILAC + SOLID);
  return U(r, M(sdSphere(p - vec3(0.0, 0.54, 0.0), 0.16), WHITE + SOLID));
}
vec2 house(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.2, 0.0), vec3(0.28, 0.2, 0.24)), WHITE);
  vec3 q = p - vec3(0.0, 0.4, 0.0);
  float g = (0.3 * abs(q.x) + 0.3 * q.y - 0.09) / length(vec2(0.3, 0.3));
  return U(r, M(max(max(g, -q.y), abs(q.z) - 0.27), BLUE));
}
vec2 bill(vec3 p) {
  vec2 r = M(sdBox(rotX(p - vec3(0.0, 0.32, 0.0), 0.0), vec3(0.24, 0.32, 0.025)), WHITE);
  vec3 q = p - vec3(0.0, 0.32, 0.03);
  return paint(r, max(abs(q.x) - 0.15, abs(mod(q.y, 0.14) - 0.07) - 0.022), STEEL);
}
vec2 coin(vec3 p) {
  vec3 q = rotX(p - vec3(0.0, 0.32, 0.0), 1.5708);
  return U(M(sdChamferCylinder(q, 0.3, 0.07, 0.02), LILAC + ROUND), M(sdCylinder(q - vec3(0.0, 0.07, 0.0), 0.18, 0.01), BLUE + ROUND));
}
vec2 folder(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.24, 0.0), vec3(0.34, 0.24, 0.26), 0.015), LIME);
  return U(r, M(sdBox(p - vec3(0.0, 0.5, 0.0), vec3(0.36, 0.025, 0.28)), LILAC));
}

vec2 mark(vec3 p, vec2 at, float t0, float k) {
  float s = pop(t0, 0.25);
  if (s < 0.01) return M(length(p - vec3(at.x, 0.3, at.y)) - 0.1, WHITE);
  vec3 q = rotY(p - vec3(at.x, 0.03 + 0.05 * abs(sin(T * 5.0 + k)), at.y), 0.4 * sin(T * 2.0 + k)) / s;
  vec2 r = k < 0.5 ? person(q) : k < 1.5 ? house(q) : k < 2.5 ? bill(q) : k < 3.5 ? coin(q) : folder(q);
  r.x *= s;
  return r;
}

vec2 map(vec3 p) {
  float a = (1.0 - glide(0.05, 0.5)) * 2.95;
  vec2 r = sheet(p);
  r.x = max(r.x, abs(p.x) - 0.95);
  vec3 q = p - vec3(0.95, 0.0, 0.0);
  q.xy = rot(-a) * q.xy;
  vec3 f = vec3(q.x + 0.95, q.y, q.z);
  vec2 s = sheet(f);
  s.x = max(s.x, -q.x);
  r = U(r, s);
  q = p - vec3(-0.95, 0.0, 0.0);
  q.xy = rot(a) * q.xy;
  f = vec3(q.x - 0.95, q.y, q.z);
  s = sheet(f);
  s.x = max(s.x, q.x);
  r = U(r, s);
  if (T > 0.45) {
    r = U(r, mark(p, A, 0.5, 0.0));
    r = U(r, mark(p, B, 0.7, 1.0));
    r = U(r, mark(p, C, 0.9, 2.0));
    r = U(r, mark(p, D, 1.1, 3.0));
    r = U(r, mark(p, E, 1.3, 4.0));
  }
  return r;
}
