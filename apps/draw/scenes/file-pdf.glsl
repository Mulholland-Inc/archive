#pragma frame {"size":[96,96],"center":[0.0,0.0,0.0],"zoom":49,"pitch":24,"trim":false,"bounds":[[-1.2,-1.2,-1.2],[1.2,1.2,1.2]]}
float page(vec3 q, float w, float h, float cut) { return extrude(max(sdRect(q.xy, vec2(w, h), 0.0), q.x + q.y - w - h + cut), q.z, 0.05); }
vec2 bar(vec3 q, vec2 at, vec2 half_, float mat) { return M(extrude(sdRect(q.xy - at, half_, 0.02), q.z - 0.06, 0.012), mat + SOLID); }
vec2 map(vec3 p) {
  vec3 q = rotX(p, -0.12);
  vec2 r = M(page(q, 0.7, 0.74, 0.34), BLUE + SOLID);
  r = U(r, M(extrude(max(sdRect(q.xy - vec2(0.53, 0.57), vec2(0.17), 0.0), -(q.x + q.y - 1.1)), q.z - 0.06, 0.012), LILAC + SOLID));
  r = U(r, bar(q, vec2(-0.28, 0.46), vec2(0.26, 0.06), WHITE));
  for (int i = 0; i < 3; i++) r = U(r, bar(q, vec2(0.0, 0.14 - float(i) * 0.26), vec2(0.52, 0.05), SKY));
  return r;
}
