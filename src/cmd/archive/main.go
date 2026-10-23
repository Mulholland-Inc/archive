// Command archive is the engine on its own. A product built on it, such as a family archive,
// has a main of its own that registers its module and then calls archive.Main.
package main

import "github.com/Mulholland-Inc/archive"

func main() { archive.Main() }
