#pragma frame {"size":[1920,1080],"center":[0.0,1.0,0.0],"zoom":170,"bounds":[[-6,-0.5,-5],[9,8,5]],"t":2}

vec2 straw(vec3 p) {
  float c = glide(20.0, 0.6);
  float rs = mix(0.06, 0.95, c);
  float y = 1.08;
  vec3 q = vec3(p.y - y, p.x - 0.0, p.z);
  float xb = 1.9 - mod(T * 2.4, 4.8);
  float xb2 = 1.9 - mod(T * 2.4 + 2.4, 4.8);
  float b = exp(-pow((p.x - xb) / 0.17, 2.0)) + exp(-pow((p.x - xb2) / 0.17, 2.0));
  float d = length(q.xz) - (rs + 0.085 * b * (1.0 - c));
  float xc = mix(-0.2, 0.1, c), xh = mix(3.0, 2.7, c);
  d = max(d, abs(p.x - xc) - xh);
  vec2 r = M(d * 0.8, WHITE + ROUND);
  if (c < 0.02) {
    r = stripes(r, p.x + p.y + p.z, 0.16);
    if (b > 0.25) r.y = LIME + ROUND;
    return r;
  }
  r.x = max(r.x, -(length(q.xz) - rs * 0.86));
  r.x = max(r.x, p.y - y - rs * 0.35);
  r.y = STEEL + ROUND;
  if (c > 0.6 && p.y > y - rs && p.y < y + rs && abs(p.z) < rs * 0.9) {
    float sx = p.x + T * 6.0;
    float lane = clamp(round(p.z / 0.42), -1.0, 1.0);
    float cell = floor(sx / 0.5);
    float tier = clamp(round((p.y - y + 0.5) / 0.4), 0.0, 1.0);
    vec3 b3 = vec3(mod(sx, 0.5) - 0.25, p.y - y + 0.5 - tier * 0.4 - 0.04 * sin(cell * 3.0 + lane + T * 9.0), p.z - lane * 0.42);
    float id = mod(cell + lane * 2.0 + tier, 3.0);
    r = U(r, M(max(sdRoundBox(b3, vec3(0.21, 0.17, 0.17), 0.02), abs(p.x - xc) - xh), id < 0.5 ? BLUE : id < 1.5 ? LILAC : WHITE));
  }
  return r;
}

vec2 map(vec3 p) {
  float c = glide(20.0, 0.6);
  vec3 g = p - vec3(1.5, -0.12, 0.0);
  vec2 r = M(sdRoundBox(g, vec3(6.0, 0.12, 3.2), 0.02), WHITE);
  r = seam(r, g.y + 0.05);
  r = U(r, fDrum((p - vec3(5.0, 0.0, 0.0)) / 2.4) * vec2(2.4, 1.0));
  float wh = 2.8 * (1.0 - glide(20.0, 0.5));
  if (wh > 0.02) {
    vec3 w = p - vec3(1.0, wh * 0.5, 0.0);
    vec2 wall = M(sdBox(w, vec3(0.14, wh * 0.5, 3.0)), GRAPHITE);
    wall = stripes(wall, p.z + 3.0, 0.5);
    r = U(r, wall);
  }
  vec3 b = rotY(p - vec3(-3.65, 0.0, 0.0), 1.5708 + 0.05 * sin(T * 6.0));
  b.y -= 0.03 * abs(sin(T * 6.0));
  r = U(r, fBot(b));
  r = U(r, straw(p));
  return r;
}
