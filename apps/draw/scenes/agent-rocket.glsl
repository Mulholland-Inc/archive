#pragma frame {"label":"Rocket robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., 1.02, 0.), V(.54, 1.0, .54), PALE);
  h = p.y > 1.56 ? vec2(h.x, TONE) : h;
  h = oblong(h, p, vec2(0., 1.12), vec2(.42, .22), .17, DARK);
  h = dab(h, m, vec2(.19, 1.12), .115, EYE);
  vec2 r = h;
  vec3 f = m - V(.54, .36, 0.);
  r = U(r, M(max(sdRoundBox(f, V(.24, .3, .06), .04), dot(f.xy, normalize(vec2(.72, .4))) - .04), TONE));
  r = U(r, cone(p, V(0., .05, 0.), .1, .28, .18, MID));
  r = U(r, stick(m, V(.5, .86, 0.), V(.68, .76, .04), .05, MID));
  return r;
}
