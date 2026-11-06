#pragma frame {"size":[96,96],"center":[0.0,0.0,0.0],"zoom":49,"pitch":24,"trim":false,"bounds":[[-1.2,-1.2,-1.2],[1.2,1.2,1.2]]}
float page(vec3 q, float w, float h, float cut) { return extrude(max(sdRect(q.xy, vec2(w, h), 0.0), q.x + q.y - w - h + cut), q.z, 0.05); }
vec2 bar(vec3 q, vec2 at, vec2 half_, float mat) { return M(extrude(sdRect(q.xy - at, half_, 0.02), q.z - 0.06, 0.012), mat + SOLID); }
vec2 map(vec3 p) {
  vec3 q = rotX(p, -0.12);
  vec3 s = q - vec3(0.0, 0.2, 0.0);
  vec2 r = M(extrude(sdRect(s.xy, vec2(0.86, 0.56), 0.02), s.z, 0.05), WHITE + SOLID);
  r = U(r, bar(s, vec2(-0.3, 0.3), vec2(0.42, 0.07), BLUE));
  r = U(r, bar(s, vec2(-0.42, -0.2), vec2(0.28, 0.2), LILAC));
  r = U(r, bar(s, vec2(0.4, -0.08), vec2(0.28, 0.05), STEEL));
  r = U(r, bar(s, vec2(0.4, -0.3), vec2(0.28, 0.05), STEEL));
  r = U(r, M(sdCapsule(p, vec3(-0.2, -0.3, 0.0), vec3(-0.46, -0.72, 0.0), 0.07), STEEL));
  r = U(r, M(sdCapsule(p, vec3(0.2, -0.3, 0.0), vec3(0.46, -0.72, 0.0), 0.07), STEEL));
  return r;
}
