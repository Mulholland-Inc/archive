#pragma frame {"set":["business"],"label":"Bank","center":[0.0,0.72,0.0],"pitch":20,"bounds":[[-1.3,-0.1,-1.3],[1.3,2.0,1.3]],"group":"office"}
float gable(vec3 q, float h, float w) {
  return (h * abs(q.x) + w * q.y - h * w) / length(vec2(h, w));
}
vec2 roof(vec3 p, float y, float w, float h, float d) {
  vec3 g = p - vec3(0.0, y, 0.0);
  return M(max(max(gable(g, h, w), -g.y), abs(g.z) - d), WHITE);
}
float tooth(vec3 q) {
  vec2 n = normalize(vec2(-0.38, 0.5));
  return max(max(max(-q.y, q.x - 0.25), dot(q.xy - vec2(-0.25, 0.0), n)), abs(q.z) - 0.5);
}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.06, 0.0), vec3(0.78, 0.06, 0.6), 0.02), WHITE + SOLID);
  r = U(r, M(sdBox(p - vec3(0.0, 0.54, -0.08), vec3(0.62, 0.42, 0.42)), WHITE + SOLID));
  r = U(r, roof(p, 0.98, 0.84, 0.36, 0.62));
  vec3 q = vec3(abs(p.x), p.yz);
  r = U(r, M(sdCylinder(q - vec3(0.62, 0.54, 0.46), 0.075, 0.42), WHITE));
  r = U(r, M(sdCylinder(q - vec3(0.42, 0.54, 0.46), 0.0, 0.0), WHITE));
  r = U(r, canZ(p, V(0.0,1.72,0.0), 0.38,0.07, PALE));
  r = U(r, M(sdTorus((p - V(0.0,1.72,0.08)).xzy, 0.28, 0.025), TONE));
  r = U(r, box(p, V(0.0,1.72,0.08), V(0.04,0.17,0.012), 0.01, DARK));
  return r;
}
