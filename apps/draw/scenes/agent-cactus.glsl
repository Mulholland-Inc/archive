#pragma frame {"label":"Cactus"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = M(sdCapsule(p, V(0., .54, 0.), V(0., 1.16, 0.), .5), PALE);
  h = eyes(h, p, .2, 1.18, .15);
  h = smile(h, p, vec2(0., 1.06), .08, .014, DARK);
  h = line(h, p, vec2(-.3, .9), vec2(-.3, .56), .012, TONE);
  h = line(h, p, vec2(.3, .9), vec2(.3, .56), .012, TONE);
  h = line(h, p, vec2(0., .88), vec2(0., .56), .012, TONE);
  vec2 r = h;
  r = U(r, M(min(sdCapsule(p, V(.4, .8, 0.), V(.72, .8, 0.), .13), sdCapsule(p, V(.72, .8, 0.), V(.72, 1.1, 0.), .13)), PALE));
  r = U(r, M(min(sdCapsule(p, V(-.4, .68, 0.), V(-.72, .68, 0.), .13), sdCapsule(p, V(-.72, .68, 0.), V(-.72, .94, 0.), .13)), PALE));
  r = U(r, cone(p, V(0., .2, 0.), .2, .4, .54, TONE));
  r = U(r, ball(polarY(rotY(p, .3), 5.), V(.1, 1.7, 0.), .075, TONE));
  return r;
}
