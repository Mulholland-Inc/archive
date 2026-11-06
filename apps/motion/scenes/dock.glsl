#pragma frame {"size":[1920,1080],"center":[0.0,0.9,0.0],"zoom":150,"bounds":[[-8,-0.5,-8],[8,5,8]],"t":2}

vec2 map(vec3 p) {
  vec2 r = fDrum(p / 1.15) * vec2(1.15, 1.0);
  vec3 g = p - vec3(0.0, -0.08, 0.0);
  float disc = extrude(length(p.xz) - 1.7, g.y, 0.08);
  r = U(r, M(disc, WHITE + ROUND));
  float a = atan(p.z, p.x);
  float n = 8.0;
  float k = floor((a + PI / n) / (TAU / n));
  float a0 = k * TAU / n;
  float s = glide(0.15 + mod(k, n) * 0.04, 0.3);
  if (s > 0.01) {
    vec2 dir = vec2(cos(a0), sin(a0));
    float l = dot(p.xz, dir);
    float side = abs(dot(p.xz, vec2(-dir.y, dir.x)));
    float reach = 1.2 + 3.3 * s;
    float flow = reach - mod(T * 3.2 + k * 0.7, 1.3) * 4.0;
    float flow2 = reach - mod(T * 3.2 + k * 0.7 + 0.65, 1.3) * 4.0;
    float bulge = exp(-pow((l - flow) / 0.17, 2.0)) + exp(-pow((l - flow2) / 0.17, 2.0));
    float d = max(length(vec2(side, p.y - 0.28)) - 0.08 - 0.06 * bulge, max(l - reach, 1.1 - l));
    vec2 q = M(d * 0.8, bulge > 0.3 ? LIME + ROUND : WHITE + ROUND);
    r = U(r, q);
  }
  return r;
}
