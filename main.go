package main

import (
	"context"
	"flag"
	"log"

	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	portvmindprovider "github.com/vmindtech/terraform-provider-portvmind/internal/provider"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Enable Terraform provider debug logging")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/vmindtech/portvmind",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), func() tfprovider.Provider {
		return portvmindprovider.New()
	}, opts)
	if err != nil {
		log.Fatal(err)
	}
}
