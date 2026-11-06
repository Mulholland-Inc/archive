#pragma frame {"size":[96,96],"center":[0.0,0.0,0.0],"zoom":49,"pitch":24,"trim":false,"bounds":[[-1.2,-1.2,-1.2],[1.2,1.2,1.2]]}
vec2 map(vec3 p) {
  vec3 q = rotX(p, -0.12);
  float back = min(sdRect(q.xy - vec2(0.0, -0.06), vec2(0.86, 0.6), 0.04), sdRect(q.xy - vec2(-0.44, 0.54), vec2(0.42, 0.14), 0.05));
  vec2 r = M(extrude(back, q.z + 0.07, 0.04), BLUE + SOLID);
  r = U(r, M(extrude(sdRect(q.xy - vec2(0.0, 0.04), vec2(0.7, 0.54), 0.02), q.z, 0.02), WHITE + SOLID));
  r = U(r, M(extrude(sdRect(q.xy - vec2(0.0, -0.18), vec2(0.86, 0.48), 0.04), q.z - 0.07, 0.04), LILAC + SOLID));
  return r;
}
