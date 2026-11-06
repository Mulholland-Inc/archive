#pragma frame {"size":[96,96],"center":[0.0,0.0,0.0],"zoom":49,"pitch":24,"trim":false,"bounds":[[-1.2,-1.2,-1.2],[1.2,1.2,1.2]]}
float page(vec3 q, float w, float h, float cut) { return extrude(max(sdRect(q.xy, vec2(w, h), 0.0), q.x + q.y - w - h + cut), q.z, 0.05); }
vec2 bar(vec3 q, vec2 at, vec2 half_, float mat) { return M(extrude(sdRect(q.xy - at, half_, 0.02), q.z - 0.06, 0.012), mat + SOLID); }
vec2 map(vec3 p) {
  vec3 q = rotX(p, -0.12);
  vec2 r = M(extrude(sdRect(q.xy, vec2(0.86, 0.68), 0.03), q.z, 0.05), WHITE + SOLID);
  vec3 f = q - vec3(0.0, 0.0, 0.062);
  r = U(r, M(extrude(sdRect(f.xy, vec2(0.72, 0.54), 0.0), f.z, 0.008), SKY + SOLID));
  float art = sdRect(f.xy, vec2(0.72, 0.54), 0.0);
  r = U(r, M(extrude(max(art, length(f.xy - vec2(-0.32, -0.95)) - 0.95), f.z - 0.012, 0.008), LILAC + SOLID));
  r = U(r, M(extrude(max(art, length(f.xy - vec2(0.38, -1.05)) - 1.0), f.z - 0.024, 0.008), BLUE + SOLID));
  r = U(r, M(extrude(length(f.xy - vec2(0.36, 0.24)) - 0.13, f.z - 0.012, 0.008), WHITE + SOLID));
  return r;
}
