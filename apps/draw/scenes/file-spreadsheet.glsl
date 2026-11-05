#pragma frame {"size":[96,96],"center":[0.0,0.0,0.0],"zoom":49,"pitch":24,"trim":false,"bounds":[[-1.2,-1.2,-1.2],[1.2,1.2,1.2]]}
float page(vec3 q, float w, float h, float cut) { return extrude(max(sdRect(q.xy, vec2(w, h), 0.0), q.x + q.y - w - h + cut), q.z, 0.05); }
vec2 bar(vec3 q, vec2 at, vec2 half_, float mat) { return M(extrude(sdRect(q.xy - at, half_, 0.02), q.z - 0.06, 0.012), mat + SOLID); }
vec2 map(vec3 p) {
  vec3 q = rotX(p, -0.12);
  vec2 r = M(extrude(sdRect(q.xy, vec2(0.76, 0.76), 0.03), q.z, 0.05), WHITE + SOLID);
  r = U(r, bar(q, vec2(0.0, 0.5), vec2(0.62, 0.12), LIME));
  for (int y = 0; y < 3; y++) for (int x = 0; x < 3; x++)
    r = U(r, bar(q, vec2(-0.43 + float(x) * 0.43, 0.14 - float(y) * 0.3), vec2(0.18, 0.1), y == 0 && x == 1 ? LILAC : SKY));
  return r;
}
