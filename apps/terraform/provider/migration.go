package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type migrationModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	SQL       types.String `tfsdk:"sql"`
	AllowDrop types.Bool   `tfsdk:"allow_drop"`
}

type migrationResource struct{ configured }

func (r *migrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_migration"
}

func (r *migrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "DDL that changes the tables and views, run once by name. Archive adds to each table the id, access, created_at, updated_at, search and row-level security it leaves out. Order migrations with depends_on. Removing one does not undo it; write a new one.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Description: "The revision it made."},
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "Unique, like 2026_11_house; a migration with a name that already ran is skipped."},
			"sql":  schema.StringAttribute{Required: true, Description: "CREATE, ALTER, DROP and COMMENT ON statements, run in one transaction."},
			"allow_drop": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Let it drop tables and columns, which deletes their data."},
		},
	}
}

func (r *migrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m migrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out struct {
		Revision string `json:"revision"`
	}
	if err := r.c.do(ctx, "POST", "/api/schema/migrate", map[string]any{"sql": m.SQL.ValueString(), "name": m.Name.ValueString(), "allow_drop": m.AllowDrop.ValueBool()}, &out); err != nil {
		resp.Diagnostics.AddError("The migration failed", err.Error())
		return
	}
	m.ID = types.StringValue(out.Revision)
	if out.Revision == "" {
		m.ID = types.StringValue(m.Name.ValueString())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *migrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m migrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *migrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m migrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	var plan migrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.SQL.ValueString() != m.SQL.ValueString() {
		resp.Diagnostics.AddAttributeError(path.Root("sql"), "A migration runs once", "Its SQL already ran; put the change in a new archive_migration.")
		return
	}
	m.AllowDrop = plan.AllowDrop
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *migrationResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {}

func (r *migrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}
