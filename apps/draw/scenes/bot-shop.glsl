#pragma frame {"set":["business"],"label":"General store","group":"shop"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.46,0), V(.6,.46,.46), .02, PALE));
  r = U(r, box(p, V(0,.98,0), V(.64,.06,.5), .02, MID));
  r = U(r, box(p, V(-.27,.28,.46), V(.25,.2,.02), .01, DARK));
  r = U(r, box(p, V(.29,.24,.46), V(.14,.24,.02), .01, TONE));
  for (int i = 0; i < 4; i++) {
  r = U(r, M(sdPrism(rotX(p - V(-.45 + float(i) * .3, .78, .6), -1.5708) * V(1,1,1), .15, .12, .14), i % 2 == 0 ? TONE : PALE));
  }
  return r;
}
