package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type viewModel struct {
	ID        types.String      `tfsdk:"id"`
	Name      types.String      `tfsdk:"name"`
	Space     types.String      `tfsdk:"space"`
	Type      types.String      `tfsdk:"type"`
	Layout    types.String      `tfsdk:"layout"`
	By        types.String      `tfsdk:"by"`
	Where     map[string]string `tfsdk:"where"`
	CreatedBy types.String      `tfsdk:"created_by"`
}

type apiView struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Space     string            `json:"space"`
	Type      string            `json:"type"`
	Layout    string            `json:"layout"`
	By        string            `json:"by"`
	Where     map[string]string `json:"where"`
	CreatedBy string            `json:"created_by"`
}

type viewResource struct{ configured }

func (r *viewResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_view"
}

func (r *viewResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "A saved view: a table, board or calendar over the objects of one type, shared with everyone in its space.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"name":       schema.StringAttribute{Required: true},
			"space":      schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: keep, Description: "Space name; defaults to your own space."},
			"type":       schema.StringAttribute{Required: true},
			"layout":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("table"), Description: "table, board or calendar."},
			"by":         schema.StringAttribute{Optional: true, Description: "Field the board groups by or the calendar places by."},
			"where":      schema.MapAttribute{Optional: true, ElementType: types.StringType, Description: "Filters, field to value."},
			"created_by": schema.StringAttribute{Computed: true, PlanModifiers: keep},
		},
	}
}

func (m *viewModel) body() map[string]any {
	return map[string]any{"name": m.Name.ValueString(), "space": m.Space.ValueString(), "type": m.Type.ValueString(),
		"layout": m.Layout.ValueString(), "by": m.By.ValueString(), "where": m.Where}
}

func (m *viewModel) fill(ctx context.Context, c *client, v apiView) diag.Diagnostics {
	var diags diag.Diagnostics
	name, err := c.spaceName(ctx, v.Space)
	if err != nil {
		diags.AddError("Reading the view's space failed", err.Error())
		return diags
	}
	m.ID, m.Name, m.Space, m.Type = types.StringValue(v.ID), types.StringValue(v.Name), types.StringValue(name), types.StringValue(v.Type)
	m.Layout, m.By, m.CreatedBy = types.StringValue(v.Layout), optString(v.By), types.StringValue(v.CreatedBy)
	if len(v.Where) > 0 || m.Where != nil {
		m.Where = v.Where
	}
	return diags
}

func (r *viewResource) save(ctx context.Context, method, p string, m *viewModel) diag.Diagnostics {
	var v apiView
	if err := r.c.do(ctx, method, p, m.body(), &v); err != nil {
		var diags diag.Diagnostics
		diags.AddError("Saving the view failed", err.Error())
		return diags
	}
	return m.fill(ctx, r.c, v)
}

func (r *viewResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m viewModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.save(ctx, "POST", "/api/views", &m)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *viewResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m viewModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var v apiView
	err := r.c.do(ctx, "GET", "/api/views/"+esc(m.ID.ValueString()), nil, &v)
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading the view failed", err.Error())
		return
	}
	resp.Diagnostics.Append(m.fill(ctx, r.c, v)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *viewResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m viewModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.save(ctx, "PUT", "/api/views/"+esc(m.ID.ValueString()), &m)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *viewResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m viewModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.do(ctx, "DELETE", "/api/views/"+esc(m.ID.ValueString()), nil, nil); err != nil && !notFound(err) {
		resp.Diagnostics.AddError("Removing the view failed", err.Error())
	}
}

func (r *viewResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
