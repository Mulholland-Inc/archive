package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "run with support for debuggers like delve")
	flag.Parse()
	err := providerserver.Serve(context.Background(), func() provider.Provider { return &archiveProvider{version: version} }, providerserver.ServeOpts{
		Address: "registry.terraform.io/mulholland-inc/archive",
		Debug:   *debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
