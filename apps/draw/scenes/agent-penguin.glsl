#pragma frame {"label":"Penguin"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., .9, 0.), V(.66, .9, .56), TONE);
  h = oblong(h, p, vec2(0., .68), vec2(.46, .66), .4, PALE);
  h = eyes(h, p, .22, 1.16, .16);
  vec2 r = h;
  r = U(r, M(sdCone(rotX(p - V(0., .92, .66), 1.5708), .15, .16, .01), MID));
  r = U(r, oval(rotZ(m - V(.72, .68, 0.), .45), V(0.), V(.09, .36, .17), TONE));
  r = U(r, oval(m, V(.22, .05, .26), V(.18, .05, .2), MID));
  return r;
}
