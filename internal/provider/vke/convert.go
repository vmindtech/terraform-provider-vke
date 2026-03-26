package vke

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func tfStringListToSlice(ctx context.Context, l types.List) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if l.IsNull() || l.IsUnknown() {
		return nil, diags
	}
	var out []string
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out, diags
}

func sliceToTFStringList(ctx context.Context, slice []string) (types.List, diag.Diagnostics) {
	if slice == nil {
		slice = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, slice)
}
