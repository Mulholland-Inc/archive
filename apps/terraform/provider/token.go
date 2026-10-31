package main

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type tokenModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Viewer    types.Bool   `tfsdk:"viewer"`
	Expires   types.String `tfsdk:"expires"`
	ExpiresAt types.String `tfsdk:"expires_at"`
	CreatedAt types.String `tfsdk:"created_at"`
	Token     types.String `tfsdk:"token"`
}

type apiToken struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Viewer    bool       `json:"viewer"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	Token     string     `json:"token"`
}

func (m *tokenModel) fill(t apiToken) {
	m.ID, m.Name, m.Viewer = types.StringValue(t.ID), types.StringValue(t.Name), types.BoolValue(t.Viewer)
	m.CreatedAt, m.ExpiresAt = types.StringValue(t.CreatedAt.UTC().Format(time.RFC3339)), types.StringNull()
	if t.ExpiresAt != nil {
		m.ExpiresAt = types.StringValue(t.ExpiresAt.UTC().Format(time.RFC3339))
	}
	if t.Token != "" {
		m.Token = types.StringValue(t.Token)
	}
}

type tokenResource struct{ configured }

func (r *tokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token"
}

func (r *tokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "An API or agent token. It acts as the user it belongs to, with their role and groups, or read only with viewer. Any change makes a new token.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"name": schema.StringAttribute{Required: true, PlanModifiers: replace},
			"viewer": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description:   "Read only: the token sees what you see and changes nothing."},
			"expires":    schema.StringAttribute{Optional: true, PlanModifiers: replace, Description: "Lifetime from creation, like 720h."},
			"expires_at": schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"token":      schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: keep, Description: "The secret. Only known when Terraform created the token."},
		},
	}
}

func (r *tokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m tokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var t apiToken
	err := r.c.do(ctx, "POST", "/auth/tokens", map[string]any{"name": m.Name.ValueString(),
		"expires": m.Expires.ValueString(), "viewer": m.Viewer.ValueBool()}, &t)
	if err != nil {
		resp.Diagnostics.AddError("Creating the token failed", err.Error())
		return
	}
	m.fill(t)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *tokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m tokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var ts []apiToken
	if err := r.c.do(ctx, "GET", "/auth/tokens", nil, &ts); err != nil {
		resp.Diagnostics.AddError("Reading tokens failed", err.Error())
		return
	}
	for _, t := range ts {
		if t.ID == m.ID.ValueString() {
			if m.Token.IsUnknown() {
				m.Token = types.StringNull()
			}
			m.fill(t)
			resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *tokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m tokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *tokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m tokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.do(ctx, "DELETE", "/auth/tokens/"+esc(m.ID.ValueString()), nil, nil); err != nil && !notFound(err) {
		resp.Diagnostics.AddError("Revoking the token failed", err.Error())
	}
}

func (r *tokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
