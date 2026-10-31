package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type memberModel struct {
	ID     types.String `tfsdk:"id"`
	Email  types.String `tfsdk:"email"`
	Role   types.String `tfsdk:"role"`
	Groups types.Set    `tfsdk:"groups"`
}

type apiMember struct {
	ID     string   `json:"id"`
	Email  string   `json:"email"`
	Role   string   `json:"role"`
	Groups []string `json:"groups"`
}

func (m *memberModel) body(ctx context.Context) (map[string]any, diag.Diagnostics) {
	groups := []string{}
	diags := m.Groups.ElementsAs(ctx, &groups, false)
	return map[string]any{"email": m.Email.ValueString(), "role": m.Role.ValueString(), "groups": groups}, diags
}

func (m *memberModel) fill(a apiMember) diag.Diagnostics {
	vals := []attr.Value{}
	for _, g := range a.Groups {
		vals = append(vals, types.StringValue(g))
	}
	set, diags := types.SetValue(types.StringType, vals)
	m.ID, m.Email, m.Role, m.Groups = types.StringValue(a.ID), types.StringValue(a.Email), types.StringValue(a.Role), set
	return diags
}

type memberResource struct{ configured }

func (r *memberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_member"
}

func (r *memberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A person in the workspace: a row of the member table. Adding one invites them; removing it takes them out with their tokens. Needs an owner's token.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"email": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "The address they sign in with, in lowercase. It cannot change."},
			"role":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("viewer"), Description: "viewer, editor or owner."},
			"groups": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})),
				Description: "Groups they are in, like hr. A row whose access is hr is seen only by the hr group and the owners."},
		},
	}
}

func (r *memberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m memberModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	body, diags := m.body(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var a apiMember
	if err := r.c.do(ctx, "POST", "/api/member", body, &a); err != nil {
		resp.Diagnostics.AddError("Adding the member failed", err.Error())
		return
	}
	resp.Diagnostics.Append(m.fill(a)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *memberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m memberModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var a apiMember
	err := r.c.do(ctx, "GET", "/api/member/"+esc(m.ID.ValueString()), nil, &a)
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading the member failed", err.Error())
		return
	}
	resp.Diagnostics.Append(m.fill(a)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *memberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m memberModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	body, diags := m.body(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	delete(body, "email")
	var a apiMember
	if err := r.c.do(ctx, "PATCH", "/api/member/"+esc(m.ID.ValueString()), body, &a); err != nil {
		resp.Diagnostics.AddError("Changing the member failed", err.Error())
		return
	}
	resp.Diagnostics.Append(m.fill(a)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *memberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m memberModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.do(ctx, "DELETE", "/api/member/"+esc(m.ID.ValueString()), nil, nil); err != nil && !notFound(err) {
		resp.Diagnostics.AddError("Removing the member failed", err.Error())
	}
}

func (r *memberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
