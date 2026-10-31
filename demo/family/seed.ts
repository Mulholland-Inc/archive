// The Quists of Gloamshire, a made-up family five generations deep. No real people or places.
const people: [string, string, string, string | null, number | null, string | null, number | null, string | null][] = [
  ["I1","Mordant Quist","m","1832",1832,"1899",1899,"Gloamshire, Erewhon"],
  ["I2","Hesper Lanthorn","f","1836",1836,"1910",1910,"Gloamshire, Erewhon"],
  ["I3","Corwin Fennimore","m","1840",1840,"1901",1901,"Gloamshire, Erewhon"],
  ["I4","Isadora Wexcombe","f","1844",1844,"1920",1920,"Gloamshire, Erewhon"],
  ["I5","Absalom Quist","m","1855",1855,"1920",1920,"Gloamshire, Erewhon"],
  ["I6","Osric Quist","m","1858",1858,"1931",1931,"Gloamshire, Erewhon"],
  ["I7","Perpetua Quist","f","1860",1860,"1940",1940,"Gloamshire, Erewhon"],
  ["I8","Ione Quist","f","1866",1866,"1950",1950,"Gloamshire, Erewhon"],
  ["I10","Thaddeus Quist","m","1863",1863,"1938",1938,"Gloamshire, Erewhon"],
  ["I11","Marigold Fennimore","f","1867",1867,"1949",1949,"Gloamshire, Erewhon"],
  ["I12","Ansel Brackwater","m","1870",1870,"1944",1944,"Gloamshire, Erewhon"],
  ["I13","Linnet Pimbury","f","1874",1874,"1958",1958,"Gloamshire, Erewhon"],
  ["I14","Tobiah Tallowmere","m","1868",1868,"1930",1930,"Gloamshire, Erewhon"],
  ["I15","Elowen Gorrel","f","1872",1872,"1961",1961,"Gloamshire, Erewhon"],
  ["I16","Caspian Orrin","m","1875",1875,"1950",1950,"Gloamshire, Erewhon"],
  ["I17","Sorrel Wexcombe","f","1879",1879,"1966",1966,"Gloamshire, Erewhon"],
  ["I20","Alder Quist","m","1899",1899,"1975",1975,"Gloamshire, Erewhon"],
  ["I21","Fenna Brackwater","f","1902",1902,"1990",1990,"Gloamshire, Erewhon"],
  ["I22","Rook Quist","m","1896",1896,"1960",1960,"Gloamshire, Erewhon"],
  ["I23","Tamsin Vell","f","1898",1898,"1971",1971,"Gloamshire, Erewhon"],
  ["I24","Juno Quist","f","1901",1901,"1988",1988,"Gloamshire, Erewhon"],
  ["I25","Bramble Quist","m","1905",1905,"1912",1912,"Gloamshire, Erewhon"],
  ["I26","Hollis Brackwater","m","1896",1896,"1970",1970,"Gloamshire, Erewhon"],
  ["I27","Verity Brackwater","f","1898",1898,"1980",1980,"Gloamshire, Erewhon"],
  ["I28","Ambrose Brackwater","m","1904",1904,"1979",1979,"Gloamshire, Erewhon"],
  ["I29","Celandine Brackwater","f","1906",1906,"1990",1990,"Gloamshire, Erewhon"],
  ["I30","Lark Brackwater","f","1908",1908,"1912",1912,"Gloamshire, Erewhon"],
  ["I31","Piers Brackwater","m","1910",1910,"1985",1985,"Gloamshire, Erewhon"],
  ["I32","Saffron Brackwater","f","1913",1913,"2000",2000,"Gloamshire, Erewhon"],
  ["I33","Edwy Brackwater","m","1916",1916,"1944",1944,"Gloamshire, Erewhon"],
  ["I34","Wystan Tallowmere","m","1904",1904,"1982",1982,"Gloamshire, Erewhon"],
  ["I35","Clementine Orrin","f","1907",1907,"2001",2001,"Gloamshire, Erewhon"],
  ["I40","Odo Quist","m","1929",1929,"2012",2012,"Gloamshire, Erewhon"],
  ["I41","Hazel Tallowmere","f","1932",1932,null,null,"Gloamshire, Erewhon"],
  ["I42","Pell Quist","m","1926",1926,"1999",1999,"Gloamshire, Erewhon"],
  ["I43","Ivy Marrable","f","1928",1928,"2015",2015,"Gloamshire, Erewhon"],
  ["I44","Quill Quist","m","1934",1934,"1944",1944,"Gloamshire, Erewhon"],
  ["I45","Ione Tallowmere","f","1930",1930,null,null,"Gloamshire, Erewhon"],
  ["I46","Rowan Tallowmere","m","1936",1936,"2001",2001,"Gloamshire, Erewhon"],
  ["I47","Mabyn Kettle","f","1938",1938,"1970",1970,"Gloamshire, Erewhon"],
  ["I48","Thea Pascoe","f","1945",1945,null,null,"Gloamshire, Erewhon"],
  ["I50","Bryony Quist","f","1952",1952,null,null,"Gloamshire, Erewhon"],
  ["I51","Lowen Quist","m","1954",1954,null,null,"Gloamshire, Erewhon"],
  ["I52","Gethin Quist","m","1957",1957,null,null,"Gloamshire, Erewhon"],
  ["I53","Morwenna Tallowmere","f","1962",1962,null,null,"Gloamshire, Erewhon"],
  ["I54","Kit Tallowmere","m","1965",1965,null,null,"Gloamshire, Erewhon"],
  ["I55","Senna Tallowmere","f","1975",1975,null,null,"Gloamshire, Erewhon"],
  ["I60","Juniper Quist","f","1958",1958,null,null,"Gloamshire, Erewhon"],
  ["I61","Arlo Penhallow","m","1956",1956,null,null,"Gloamshire, Erewhon"],
  ["I62","Tobin Quist","m","1955",1955,null,null,"Gloamshire, Erewhon"],
  ["I63","Nell Vosper","f","1957",1957,null,null,"Gloamshire, Erewhon"],
  ["I64","Merritt Quist","m","1961",1961,null,null,"Gloamshire, Erewhon"],
  ["I65","Cato Penhallow","m","1984",1984,null,null,"Gloamshire, Erewhon"],
  ["I66","Ilsa Penhallow","f","1987",1987,null,null,"Gloamshire, Erewhon"],
  ["I67","Gideon Penhallow","m","1925",1925,"2003",2003,"Gloamshire, Erewhon"],
  ["I68","Rosalind Kember","f","1928",1928,"2019",2019,"Gloamshire, Erewhon"],
  ["I69","Briar Quist","f","1982",1982,null,null,"Gloamshire, Erewhon"]
];
const families: [string, string | null, string | null, string[]][] = [
  ["F1","I1","I2",["I6","I7","I10","I8"]],
  ["F3","I3","I4",["I11"]],
  ["F4","I10","I11",["I22","I20","I24","I25"]],
  ["F5","I12","I13",["I26","I27","I21","I28","I29","I30","I31","I32","I33"]],
  ["F6","I14","I15",["I34"]],
  ["F7","I16","I17",["I35"]],
  ["F8","I20","I21",["I42","I40","I44"]],
  ["F9","I34","I35",["I45","I41","I46"]],
  ["F10","I40","I41",["I62","I60","I64"]],
  ["F11","I22","I23",[]],
  ["F12","I42","I43",["I50","I51","I52"]],
  ["F13","I46","I47",["I53","I54"]],
  ["F14","I46","I48",["I55"]],
  ["F15","I61","I60",["I65","I66"]],
  ["F16","I67","I68",["I61"]],
  ["F17","I62","I63",["I69"]],
  ["F18","I1",null,["I5"]]
];

const P: Record<string, string> = {};
for (const [key, name, sex, born, born_year, died, died_year, birthplace] of people) {
  P[key] = (await archive.create("person", { name, sex, born, born_year, died, died_year, birthplace, home: key === "I60" })).id;
}
for (const [, a, b, kids] of families) {
  const partnership = (await archive.create("partnership", { a: a && P[a], b: b && P[b] })).id;
  for (const k of kids) await archive.create("parent_link", { child: P[k], partnership });
}
return { people: people.length, families: families.length };
