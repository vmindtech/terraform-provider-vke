package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/vmindtech/terraform-provider-portvmind/internal/provider/core"
)

// New is kept for backwards compatibility with older import paths.
func New() provider.Provider {
	return core.New()
}
