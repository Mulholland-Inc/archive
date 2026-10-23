package main

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type policyModel struct {
	ID    types.String `tfsdk:"id"`
	Space types.String `tfsdk:"space"`
	User  types.String `tfsdk:"user"`
	Role  types.String `tfsdk:"role"`
}

type apiMember struct {
	Email string `json:"email"`
	Level string `json:"level"`
}

type policyResource struct{ configured }

func (r *policyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *policyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "A user's role in a space. Archive enforces it with Postgres role grants and row-level security.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"space": schema.StringAttribute{Required: true, PlanModifiers: replace},
			"user":  schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "The user's email address."},
			"role":  schema.StringAttribute{Required: true, Description: "owner, admin, editor or viewer."},
		},
	}
}

func (r *policyResource) grant(ctx context.Context, m *policyModel, role string) diag.Diagnostics {
	var diags diag.Diagnostics
	err := r.c.do(ctx, "POST", "/auth/spaces/"+esc(m.Space.ValueString())+"/members", map[string]string{"email": m.User.ValueString(), "role": role}, nil)
	if err != nil {
		diags.AddError("Changing the membership failed", err.Error())
	}
	m.ID = types.StringValue(m.Space.ValueString() + "/" + m.User.ValueString())
	return diags
}

func (r *policyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m policyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.grant(ctx, &m, m.Role.ValueString())...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *policyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m policyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	space, user, _ := strings.Cut(m.ID.ValueString(), "/")
	var ms []apiMember
	err := r.c.do(ctx, "GET", "/auth/spaces/"+esc(space)+"/members", nil, &ms)
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading the space members failed", err.Error())
		return
	}
	for _, mb := range ms {
		if mb.Email == user {
			m.Space, m.User, m.Role = types.StringValue(space), types.StringValue(user), types.StringValue(mb.Level)
			resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *policyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m policyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.grant(ctx, &m, m.Role.ValueString())...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}

func (r *policyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m policyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.grant(ctx, &m, "none")...)
}

func (r *policyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !strings.Contains(req.ID, "/") {
		resp.Diagnostics.AddError("Bad import id", "want space/email, like family/bob@example.com")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
