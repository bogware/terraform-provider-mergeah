// Copyright (c) Bogware. SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func resourceFields(s resourceSpec) []field {
	id := stable(computed("id", types.StringType))
	id.Description = "Terraform resource identifier. See the Import section for its format."
	return append([]field{id}, s.Fields...)
}
func resourceSchema(s resourceSpec) schema.Schema {
	return schema.Schema{MarkdownDescription: s.Description, Attributes: resourceAttributes(resourceFields(s))}
}
func resourceAttributes(fields []field) map[string]schema.Attribute {
	out := map[string]schema.Attribute{}
	for _, f := range fields {
		if f.Replace {
			f.Description += " Changes require replacement."
		}
		if f.Default != nil {
			f.Description += fmt.Sprintf(" Defaults to `%v`.", f.Default)
		}
		optional := !f.Required && !f.Computed
		isComputed := f.Computed || f.OptionalComputed || f.Default != nil
		switch t := f.Type.(type) {
		case basetypes.StringType:
			a := schema.StringAttribute{Required: f.Required, Optional: optional, Computed: isComputed, Sensitive: f.Sensitive, MarkdownDescription: f.Description}
			if f.Default != nil {
				a.Default = stringdefault.StaticString(f.Default.(string))
			}
			if f.Replace {
				a.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplace()}
			}
			if f.Stable {
				a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			}
			out[f.Name] = a
		case basetypes.BoolType:
			a := schema.BoolAttribute{Required: f.Required, Optional: optional, Computed: isComputed, Sensitive: f.Sensitive, MarkdownDescription: f.Description}
			if f.Default != nil {
				a.Default = booldefault.StaticBool(f.Default.(bool))
			}
			if f.Replace {
				a.PlanModifiers = []planmodifier.Bool{boolplanmodifier.RequiresReplace()}
			}
			if f.Stable {
				a.PlanModifiers = append(a.PlanModifiers, boolplanmodifier.UseStateForUnknown())
			}
			out[f.Name] = a
		case basetypes.Float64Type:
			a := schema.Float64Attribute{Required: f.Required, Optional: optional, Computed: isComputed, Sensitive: f.Sensitive, MarkdownDescription: f.Description}
			if f.Default != nil {
				a.Default = float64default.StaticFloat64(f.Default.(float64))
			}
			if f.Replace {
				a.PlanModifiers = []planmodifier.Float64{float64planmodifier.RequiresReplace()}
			}
			if f.Stable {
				a.PlanModifiers = append(a.PlanModifiers, float64planmodifier.UseStateForUnknown())
			}
			out[f.Name] = a
		case types.SetType:
			a := schema.SetAttribute{ElementType: t.ElemType, Required: f.Required, Optional: optional, Computed: isComputed, Sensitive: f.Sensitive, MarkdownDescription: f.Description}
			if f.Default != nil {
				v, _ := fromJSON(context.Background(), f.Type, f.Default)
				a.Default = setdefault.StaticValue(v.(types.Set))
			}
			if f.Replace {
				a.PlanModifiers = []planmodifier.Set{setplanmodifier.RequiresReplace()}
			}
			if f.Stable {
				a.PlanModifiers = append(a.PlanModifiers, setplanmodifier.UseStateForUnknown())
			}
			out[f.Name] = a
		case types.ListType:
			if len(f.Children) > 0 {
				a := schema.ListNestedAttribute{NestedObject: schema.NestedAttributeObject{Attributes: resourceAttributes(f.Children)}, Required: f.Required, Optional: optional, Computed: isComputed, Sensitive: f.Sensitive, MarkdownDescription: f.Description}
				if f.Replace {
					a.PlanModifiers = []planmodifier.List{listplanmodifier.RequiresReplace()}
				}
				if f.Stable {
					a.PlanModifiers = append(a.PlanModifiers, listplanmodifier.UseStateForUnknown())
				}
				out[f.Name] = a
			} else {
				a := schema.ListAttribute{ElementType: t.ElemType, Required: f.Required, Optional: optional, Computed: isComputed, Sensitive: f.Sensitive, MarkdownDescription: f.Description}
				if f.Replace {
					a.PlanModifiers = []planmodifier.List{listplanmodifier.RequiresReplace()}
				}
				if f.Stable {
					a.PlanModifiers = append(a.PlanModifiers, listplanmodifier.UseStateForUnknown())
				}
				out[f.Name] = a
			}
		case types.MapType:
			a := schema.MapAttribute{ElementType: t.ElemType, Required: f.Required, Optional: optional, Computed: isComputed, Sensitive: f.Sensitive, MarkdownDescription: f.Description}
			if f.Replace {
				a.PlanModifiers = []planmodifier.Map{mapplanmodifier.RequiresReplace()}
			}
			if f.Stable {
				a.PlanModifiers = append(a.PlanModifiers, mapplanmodifier.UseStateForUnknown())
			}
			out[f.Name] = a
		case types.ObjectType:
			a := schema.SingleNestedAttribute{Attributes: resourceAttributes(f.Children), Required: f.Required, Optional: optional, Computed: isComputed, Sensitive: f.Sensitive, MarkdownDescription: f.Description}
			if f.Replace {
				a.PlanModifiers = []planmodifier.Object{objectplanmodifier.RequiresReplace()}
			}
			if f.Stable {
				a.PlanModifiers = append(a.PlanModifiers, objectplanmodifier.UseStateForUnknown())
			}
			out[f.Name] = a
		}
	}
	return out
}

// objectValue keeps schema and state types identical, including optional nested attributes.
func objectValue(fields []field, values map[string]attr.Value) types.Object {
	return types.ObjectValueMust(fieldTypes(fields), values)
}
