package main

import (
	"context"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const schemaMain = "main.sql"

type schemaModel struct {
	ID        types.String      `tfsdk:"id"`
	Content   types.String      `tfsdk:"content"`
	Files     map[string]string `tfsdk:"files"`
	AllowDrop types.Bool        `tfsdk:"allow_drop"`
}

func optString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func (m *schemaModel) files() map[string]string {
	if !m.Content.IsNull() && !m.Content.IsUnknown() {
		return map[string]string{schemaMain: m.Content.ValueString()}
	}
	if m.Files == nil {
		return map[string]string{}
	}
	return m.Files
}

func (c *client) applySchema(ctx context.Context, files map[string]string, allowDrop bool) diag.Diagnostics {
	var diags diag.Diagnostics
	if err := c.do(ctx, "POST", "/api/schema/apply", map[string]any{"files": files, "allow_drop": allowDrop}, nil); err != nil {
		diags.AddError("Applying the schema failed", err.Error())
	}
	return diags
}

type schemaResource struct{ configured }

func (r *schemaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema"
}

func (r *schemaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The DDL of every user table: one CREATE TABLE per type, with RLS, policies and comments. Archive plans the difference to the database and applies it. Use one per instance.",
		Attributes: map[string]schema.Attribute{
			"id":      schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"content": schema.StringAttribute{Optional: true, Description: "All the DDL in one file, stored as " + schemaMain + ". Set this or files."},
			"files":   schema.MapAttribute{Optional: true, ElementType: types.StringType, Description: "DDL files by name, like house.sql. Set this or content."},
			"allow_drop": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Apply statements that delete data, like dropping a table or a column. Renames are not detected; rename with ALTER ... RENAME first."},
		},
	}
}

func (r *schemaResource) set(ctx context.Context, m *schemaModel, d *diag.Diagnostics) {
	if !m.Content.IsNull() && m.Files != nil {
		d.AddAttributeError(path.Root("files"), "Set content or files, not both", "")
		return
	}
	d.Append(r.c.applySchema(ctx, m.files(), m.AllowDrop.ValueBool())...)
	m.ID = types.StringValue("schema")
}

func (r *schemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m schemaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.set(ctx, &m, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *schemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m schemaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out struct {
		Files map[string]string `json:"files"`
	}
	if err := r.c.do(ctx, "GET", "/api/schema", nil, &out); err != nil {
		resp.Diagnostics.AddError("Reading the schema failed", err.Error())
		return
	}
	if body, ok := out.Files[schemaMain]; ok && len(out.Files) == 1 && m.Files == nil {
		m.Content = types.StringValue(body)
	} else if !maps.Equal(out.Files, m.files()) {
		m.Content, m.Files = types.StringNull(), out.Files
	}
	m.ID = types.StringValue("schema")
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *schemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m schemaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.set(ctx, &m, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *schemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m schemaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.c.applySchema(ctx, map[string]string{}, m.AllowDrop.ValueBool())...)
}

func (r *schemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
