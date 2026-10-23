package main

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type sourceModel struct {
	ID          types.String  `tfsdk:"id"`
	Name        types.String  `tfsdk:"name"`
	Kind        types.String  `tfsdk:"kind"`
	Space       types.String  `tfsdk:"space"`
	Type        types.String  `tfsdk:"type"`
	Schedule    types.String  `tfsdk:"schedule"`
	Rate        types.Float64 `tfsdk:"rate"`
	Config      types.String  `tfsdk:"config"`
	Credentials types.String  `tfsdk:"credentials"`
	Status      types.String  `tfsdk:"status"`
	AuthURL     types.String  `tfsdk:"auth_url"`
}

type apiSource struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	Space    string          `json:"space"`
	Type     string          `json:"type"`
	Schedule string          `json:"schedule"`
	Rate     float64         `json:"rate"`
	Config   json.RawMessage `json:"config"`
	Status   string          `json:"status"`
	AuthURL  string          `json:"auth_url"`
}

func jsonObject(s types.String) (map[string]any, error) {
	out := map[string]any{}
	if s.IsNull() || s.ValueString() == "" {
		return out, nil
	}
	return out, json.Unmarshal([]byte(s.ValueString()), &out)
}

func sameJSON(s types.String, b []byte) bool {
	a, err := jsonObject(s)
	var o map[string]any
	if err != nil || json.Unmarshal(b, &o) != nil {
		return false
	}
	return len(a) == 0 && len(o) == 0 || reflect.DeepEqual(a, o)
}

type sourceResource struct{ configured }

func (r *sourceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source"
}

func (r *sourceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "A connector instance that syncs a platform into Archive's raw layer.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":     schema.StringAttribute{Required: true, PlanModifiers: replace},
			"kind":     schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "gmail, drive, imap, folder or external."},
			"space":    schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, Description: "Space name; defaults to your own space."},
			"type":     schema.StringAttribute{Optional: true, Description: "Type that intake maps new items into."},
			"schedule": schema.StringAttribute{Optional: true, Description: "Sync every duration, like 15m."},
			"rate":     schema.Float64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Float64{float64planmodifier.UseStateForUnknown()}, Description: "Requests per second to the platform."},
			"config":   schema.StringAttribute{Optional: true, PlanModifiers: replace, Description: "Connector config as JSON, for example jsonencode({path = \"/inbox\"})."},
			"credentials": schema.StringAttribute{Optional: true, Sensitive: true, PlanModifiers: replace,
				Description: "Connector credentials as JSON. Stored encrypted and never read back."},
			"status":   schema.StringAttribute{Computed: true},
			"auth_url": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Description: "Google sign-in address for gmail and drive sources without a refresh token."},
		},
	}
}

func (m *sourceModel) fill(ctx context.Context, c *client, s apiSource) diag.Diagnostics {
	var diags diag.Diagnostics
	name, err := c.spaceName(ctx, s.Space)
	if err != nil {
		diags.AddError("Reading the source's space failed", err.Error())
		return diags
	}
	m.ID, m.Name, m.Kind, m.Space = types.StringValue(s.ID), types.StringValue(s.Name), types.StringValue(s.Kind), types.StringValue(name)
	m.Type, m.Schedule, m.Rate, m.Status = optString(s.Type), optString(s.Schedule), types.Float64Value(s.Rate), types.StringValue(s.Status)
	if !sameJSON(m.Config, s.Config) {
		m.Config = types.StringValue(string(s.Config))
	}
	if m.AuthURL.IsUnknown() {
		m.AuthURL = optString(s.AuthURL)
	}
	return diags
}

func (r *sourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m sourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	config, err := jsonObject(m.Config)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "config is not a JSON object", err.Error())
		return
	}
	creds, err := jsonObject(m.Credentials)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("credentials"), "credentials is not a JSON object", "")
		return
	}
	var s apiSource
	err = r.c.do(ctx, "POST", "/api/sources", map[string]any{
		"kind": m.Kind.ValueString(), "name": m.Name.ValueString(), "space": m.Space.ValueString(), "type": m.Type.ValueString(),
		"schedule": m.Schedule.ValueString(), "rate": m.Rate.ValueFloat64(), "config": config, "credentials": creds,
	}, &s)
	if err != nil {
		resp.Diagnostics.AddError("Creating the source failed", err.Error())
		return
	}
	resp.Diagnostics.Append(m.fill(ctx, r.c, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *sourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m sourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s apiSource
	err := r.c.do(ctx, "GET", "/api/sources/"+esc(m.ID.ValueString()), nil, &s)
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading the source failed", err.Error())
		return
	}
	resp.Diagnostics.Append(m.fill(ctx, r.c, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *sourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m sourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s apiSource
	err := r.c.do(ctx, "PATCH", "/api/sources/"+esc(m.ID.ValueString()), map[string]any{
		"type": m.Type.ValueString(), "schedule": m.Schedule.ValueString(), "rate": m.Rate.ValueFloat64(),
	}, &s)
	if err != nil {
		resp.Diagnostics.AddError("Updating the source failed", err.Error())
		return
	}
	resp.Diagnostics.Append(m.fill(ctx, r.c, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *sourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m sourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.do(ctx, "DELETE", "/api/sources/"+esc(m.ID.ValueString()), nil, nil); err != nil && !notFound(err) {
		resp.Diagnostics.AddError("Removing the source failed", err.Error())
	}
}

func (r *sourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
