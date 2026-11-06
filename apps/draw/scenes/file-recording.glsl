#pragma frame {"size":[96,96],"center":[0.0,0.0,0.0],"zoom":49,"pitch":24,"trim":false,"bounds":[[-1.2,-1.2,-1.2],[1.2,1.2,1.2]]}
float page(vec3 q, float w, float h, float cut) { return extrude(max(sdRect(q.xy, vec2(w, h), 0.0), q.x + q.y - w - h + cut), q.z, 0.05); }
vec2 bar(vec3 q, vec2 at, vec2 half_, float mat) { return M(extrude(sdRect(q.xy - at, half_, 0.02), q.z - 0.06, 0.012), mat + SOLID); }
vec2 map(vec3 p) {
  vec3 q = rotX(p, -0.12);
  vec2 r = M(extrude(sdRect(q.xy, vec2(0.86, 0.62), 0.14), q.z, 0.05), LIME + SOLID);
  for (int i = 0; i < 5; i++) {
    float h = i == 2 ? 0.42 : (i == 1 || i == 3) ? 0.28 : 0.14;
    r = U(r, bar(q, vec2(-0.5 + float(i) * 0.25, 0.0), vec2(0.07, h), WHITE));
  }
  return r;
}
