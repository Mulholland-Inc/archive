package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type spacesModel struct {
	Spaces []spaceModel `tfsdk:"spaces"`
}

type spaceModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Level types.String `tfsdk:"level"`
}

type spacesDataSource struct{ dsConfigured }

func (d *spacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spaces"
}

func (d *spacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The spaces this token can see, with its role in each.",
		Attributes: map[string]schema.Attribute{
			"spaces": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"id":    schema.StringAttribute{Computed: true},
				"name":  schema.StringAttribute{Computed: true},
				"level": schema.StringAttribute{Computed: true},
			}}},
		},
	}
}

func (d *spacesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	sp, err := d.c.spaces(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Reading spaces failed", err.Error())
		return
	}
	m := spacesModel{Spaces: []spaceModel{}}
	for _, s := range sp {
		m.Spaces = append(m.Spaces, spaceModel{ID: types.StringValue(s.ID), Name: types.StringValue(s.Name), Level: types.StringValue(s.Level)})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
