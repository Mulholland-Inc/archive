#pragma frame {"size":[1920,1080],"center":[0.0,1.6,0.0],"zoom":170,"yaw":25,"pitch":24,"bounds":[[-8,-1,-4],[18,12,4]],"t":2.4}

float gableD(vec3 q, float h, float w) { return (h * abs(q.x) + w * q.y - h * w) / length(vec2(h, w)); }

vec2 house(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.1, 0.0), vec3(2.2, 0.1, 2.0), 0.02), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 1.0, 0.0), vec3(1.3, 0.8, 1.1)), WHITE));
  vec3 q = p - vec3(0.0, 1.8, 0.0);
  r = U(r, M(max(max(gableD(q, 1.0, 1.5), -q.y), abs(q.z) - 1.2), LIME));
  r = paint(r, max(sdRect(vec2(p.x + 0.6, p.y - 0.95), vec2(0.28, 0.28), 0.0), -p.z + 1.09), SKY);
  r = paint(r, max(sdRect(vec2(p.x - 0.5, p.y - 0.65), vec2(0.26, 0.45), 0.0), -p.z + 1.09), GRAPHITE);
  return r;
}

vec2 tower(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.1, 0.0), vec3(2.0, 0.1, 2.0), 0.02), WHITE);
  vec2 t = M(sdBox(p - vec3(0.0, 2.7, 0.0), vec3(1.15, 2.5, 1.15)), WHITE);
  vec2 cell = vec2(repC(p.x, 0.5, 4.0), repC(p.y - 2.7, 0.6, 8.0));
  if (p.z > 1.14) t = paint(t, sdRect(cell, vec2(0.15, 0.2), 0.0), mod(floor(p.x * 2.0) + floor(p.y / 0.6 + T * 2.0), 3.0) < 1.0 ? LILAC : SKY);
  vec2 cz = vec2(repC(p.z, 0.5, 4.0), repC(p.y - 2.7, 0.6, 8.0));
  if (p.x > 1.14) t = paint(t, sdRect(cz, vec2(0.15, 0.2), 0.0), SKY);
  return U(r, t);
}

vec2 cloud(vec3 p) {
  p.y += 0.12 * sin(T * 3.0);
  float d = sdEllipsoid(p - vec3(0.0, 3.7, 0.0), vec3(2.0, 0.7, 1.5));
  d = smin(d, sdSphere(p - vec3(-1.0, 3.7, 0.3), 0.8), 0.25);
  d = smin(d, sdSphere(p - vec3(1.1, 3.6, 0.1), 0.9), 0.25);
  d = smin(d, sdSphere(p - vec3(0.1, 4.0, -0.2), 0.9), 0.25);
  return M(max(d, p.y - 4.4), WHITE + SOLID);
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  r = U(r, house(p));
  r = U(r, tower(p - vec3(7.0, 0.0, 0.0)));
  r = U(r, cloud(p - vec3(14.0, 0.0, 0.0)));
  float h1 = glide(0.28, 0.55), h2 = glide(1.2, 0.55);
  vec3 a = vec3(0.0, 2.8, 0.0), b = vec3(7.0, 5.2, 0.0), c = vec3(14.0, 4.42, 0.0);
  vec3 at = mix(a, b, h1);
  at.y += 1.2 * sin(PI * h1);
  at = mix(at, c, h2);
  at.y += 1.2 * sin(PI * h2);
  float sq = 1.0 + 0.12 * hop(0.8, 0.2) + 0.12 * hop(1.72, 0.2);
  vec3 d = (p - at) / vec3(0.72 * sq, 0.72 / sq, 0.72 * sq);
  vec2 dr = fDrum(d);
  dr.x *= 0.62;
  r = U(r, dr);
  return r;
}
