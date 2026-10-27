package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type typeModel struct {
	ID          types.String          `tfsdk:"id"`
	Name        types.String          `tfsdk:"name"`
	Description types.String          `tfsdk:"description"`
	Extends     types.String          `tfsdk:"extends"`
	Fields      map[string]fieldModel `tfsdk:"fields"`
}

type fieldModel struct {
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`
	Required    types.Bool   `tfsdk:"required"`
	List        types.Bool   `tfsdk:"list"`
	Indexed     types.Bool   `tfsdk:"indexed"`
	To          types.String `tfsdk:"to"`
	Enum        []string     `tfsdk:"enum"`
}

type apiField struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	List        bool     `json:"list,omitempty"`
	Indexed     bool     `json:"indexed,omitempty"`
	To          string   `json:"to,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

func optString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func (f fieldModel) api() apiField {
	return apiField{Type: f.Type.ValueString(), Description: f.Description.ValueString(), Required: f.Required.ValueBool(),
		List: f.List.ValueBool(), Indexed: f.Indexed.ValueBool(), To: f.To.ValueString(), Enum: f.Enum}
}

func fieldFrom(f apiField) fieldModel {
	return fieldModel{Type: types.StringValue(f.Type), Description: optString(f.Description), Required: types.BoolValue(f.Required),
		List: types.BoolValue(f.List), Indexed: types.BoolValue(f.Indexed), To: optString(f.To), Enum: f.Enum}
}

func (c *client) types(ctx context.Context) (map[string]map[string]any, error) {
	var out struct {
		Spec struct {
			Types map[string]map[string]any `json:"types"`
		} `json:"spec"`
	}
	return out.Spec.Types, c.do(ctx, "GET", "/api/types", nil, &out)
}

func typeFields(def map[string]any) (map[string]apiField, error) {
	out := map[string]apiField{}
	b, err := json.Marshal(def["fields"])
	if err != nil || def["fields"] == nil {
		return out, err
	}
	return out, json.Unmarshal(b, &out)
}

func typeLevel(def map[string]any) map[string]any {
	l := maps.Clone(def)
	delete(l, "fields")
	return l
}

func (m *typeModel) setLevel(l map[string]any) {
	for k, v := range map[string]types.String{"description": m.Description, "extends": m.Extends} {
		delete(l, k)
		if v.ValueString() != "" {
			l[k] = v.ValueString()
		}
	}
}

func (m *typeModel) def(cur map[string]any) map[string]any {
	fields := map[string]apiField{}
	for k, f := range m.Fields {
		fields[k] = f.api()
	}
	def := typeLevel(cur)
	if def == nil {
		def = map[string]any{}
	}
	m.setLevel(def)
	def["fields"] = fields
	return def
}

func (c *client) define(ctx context.Context, name string, def func(cur map[string]any) (map[string]any, error)) diag.Diagnostics {
	var diags diag.Diagnostics
	all, err := c.types(ctx)
	if err != nil {
		diags.AddError("Reading the ontology failed", err.Error())
		return diags
	}
	d, err := def(all[name])
	if err != nil {
		diags.AddError("Building the type failed", err.Error())
		return diags
	}
	if err := c.do(ctx, "PUT", "/api/types/"+esc(name), d, nil); err != nil {
		diags.AddError("Writing the type failed", fmt.Sprintf("type %s: %v", name, err))
	}
	return diags
}

type typeResource struct{ configured }

func (r *typeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_type"
}

func (r *typeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An ontology type and its fields. Changing it does not migrate existing objects.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"description": schema.StringAttribute{Optional: true},
			"extends":     schema.StringAttribute{Optional: true, Description: "Parent type whose fields this type inherits."},
			"fields": schema.MapNestedAttribute{
				Optional:    true,
				Description: "Fields by name. Renaming a field here removes the old one; stored values stay under the old name.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"type":        schema.StringAttribute{Required: true, Description: "A built-in type like text, email, money or reference, or another type."},
					"description": schema.StringAttribute{Optional: true},
					"required":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
					"list":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
					"indexed":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
					"to":          schema.StringAttribute{Optional: true, Description: "Target type of a reference field."},
					"enum":        schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "Allowed values of an enum field."},
				}},
			},
		},
	}
}

func (r *typeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m typeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.c.define(ctx, m.Name.ValueString(), func(cur map[string]any) (map[string]any, error) {
		if cur != nil {
			return nil, fmt.Errorf("type %s already exists; import it instead", m.Name.ValueString())
		}
		return m.def(nil), nil
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	m.ID = m.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *typeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m typeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := r.c.types(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Reading the ontology failed", err.Error())
		return
	}
	def, ok := all[m.ID.ValueString()]
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(m.fill(m.ID.ValueString(), def)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (m *typeModel) fill(name string, def map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics
	fields, err := typeFields(def)
	if err != nil {
		diags.AddError("Reading the type failed", err.Error())
		return diags
	}
	m.ID, m.Name = types.StringValue(name), types.StringValue(name)
	desc, _ := def["description"].(string)
	ext, _ := def["extends"].(string)
	m.Description, m.Extends = optString(desc), optString(ext)
	if len(fields) == 0 && len(m.Fields) == 0 {
		return diags
	}
	m.Fields = map[string]fieldModel{}
	for k, f := range fields {
		m.Fields[k] = fieldFrom(f)
	}
	return diags
}

func (r *typeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m typeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.c.define(ctx, m.Name.ValueString(), func(cur map[string]any) (map[string]any, error) {
		if cur == nil {
			return nil, fmt.Errorf("type %s no longer exists", m.Name.ValueString())
		}
		return m.def(cur), nil
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (r *typeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m typeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.do(ctx, "DELETE", "/api/types/"+esc(m.ID.ValueString()), nil, nil); err != nil && !notFound(err) {
		resp.Diagnostics.AddError("Removing the type failed", err.Error())
	}
}

func (r *typeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

type typeDataSource struct{ dsConfigured }

func (d *typeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_type"
}

func (d *typeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "One type of the ontology, built-in or user-defined, with its own fields.",
		Attributes: map[string]dschema.Attribute{
			"id":          dschema.StringAttribute{Computed: true},
			"name":        dschema.StringAttribute{Required: true},
			"description": dschema.StringAttribute{Computed: true},
			"extends":     dschema.StringAttribute{Computed: true},
			"fields": dschema.MapNestedAttribute{Computed: true, NestedObject: dschema.NestedAttributeObject{Attributes: map[string]dschema.Attribute{
				"type":        dschema.StringAttribute{Computed: true},
				"description": dschema.StringAttribute{Computed: true},
				"required":    dschema.BoolAttribute{Computed: true},
				"list":        dschema.BoolAttribute{Computed: true},
				"indexed":     dschema.BoolAttribute{Computed: true},
				"to":          dschema.StringAttribute{Computed: true},
				"enum":        dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			}}},
		},
	}
}

func (d *typeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m typeModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := d.c.types(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Reading the ontology failed", err.Error())
		return
	}
	def, ok := all[m.Name.ValueString()]
	if !ok {
		resp.Diagnostics.AddError("Type not found", "Archive has no type "+m.Name.ValueString())
		return
	}
	resp.Diagnostics.Append(m.fill(m.Name.ValueString(), def)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
