// Copyright (c) Bogware. SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func nullValue(ctx context.Context, t attr.Type) attr.Value {
	v, err := t.ValueFromTerraform(ctx, tftypes.NewValue(t.TerraformType(ctx), nil))
	if err != nil {
		panic(err)
	}
	return v
}
func readValues(ctx context.Context, raw tftypes.Value, fields []field) (map[string]attr.Value, error) {
	var rawAttrs map[string]tftypes.Value
	if err := raw.As(&rawAttrs); err != nil {
		return nil, err
	}
	out := map[string]attr.Value{}
	for _, f := range fields {
		v, ok := rawAttrs[f.Name]
		if !ok {
			out[f.Name] = nullValue(ctx, f.Type)
			continue
		}
		a, err := f.Type.ValueFromTerraform(ctx, v)
		if err != nil {
			return nil, err
		}
		out[f.Name] = a
	}
	return out, nil
}
func toJSON(ctx context.Context, v attr.Value) (any, error) {
	if v.IsUnknown() {
		return nil, fmt.Errorf("value is still unknown")
	}
	if v.IsNull() {
		return nil, nil
	}
	switch x := v.(type) {
	case types.String:
		return x.ValueString(), nil
	case types.Bool:
		return x.ValueBool(), nil
	case types.Float64:
		return x.ValueFloat64(), nil
	case types.Set:
		out := []any{}
		for _, v := range x.Elements() {
			a, e := toJSON(ctx, v)
			if e != nil {
				return nil, e
			}
			out = append(out, a)
		}
		return out, nil
	case types.List:
		out := []any{}
		for _, v := range x.Elements() {
			a, e := toJSON(ctx, v)
			if e != nil {
				return nil, e
			}
			out = append(out, a)
		}
		return out, nil
	case types.Map:
		out := map[string]any{}
		for k, v := range x.Elements() {
			a, e := toJSON(ctx, v)
			if e != nil {
				return nil, e
			}
			out[k] = a
		}
		return out, nil
	case types.Object:
		out := map[string]any{}
		for k, v := range x.Attributes() {
			if v.IsNull() {
				continue
			}
			a, e := toJSON(ctx, v)
			if e != nil {
				return nil, e
			}
			out[k] = a
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported Terraform value type")
	}
}
func fromJSON(ctx context.Context, t attr.Type, raw any) (attr.Value, error) {
	if raw == nil {
		return nullValue(ctx, t), nil
	}
	switch x := t.(type) {
	case basetypes.StringType:
		if v, ok := raw.(string); ok {
			return types.StringValue(v), nil
		}
	case basetypes.BoolType:
		if v, ok := raw.(bool); ok {
			return types.BoolValue(v), nil
		}
	case basetypes.Float64Type:
		switch v := raw.(type) {
		case json.Number:
			f, e := v.Float64()
			return types.Float64Value(f), e
		case float64:
			return types.Float64Value(v), nil
		}
	case types.SetType:
		if list, ok := raw.([]any); ok {
			out := []attr.Value{}
			for _, v := range list {
				a, e := fromJSON(ctx, x.ElemType, v)
				if e != nil {
					return nil, e
				}
				out = append(out, a)
			}
			v, d := types.SetValue(x.ElemType, out)
			if d.HasError() {
				return nil, fmt.Errorf("invalid set response")
			}
			return v, nil
		}
	case types.ListType:
		if list, ok := raw.([]any); ok {
			out := []attr.Value{}
			for _, v := range list {
				a, e := fromJSON(ctx, x.ElemType, v)
				if e != nil {
					return nil, e
				}
				out = append(out, a)
			}
			v, d := types.ListValue(x.ElemType, out)
			if d.HasError() {
				return nil, fmt.Errorf("invalid list response")
			}
			return v, nil
		}
	case types.MapType:
		if obj, ok := raw.(map[string]any); ok {
			out := map[string]attr.Value{}
			for k, v := range obj {
				a, e := fromJSON(ctx, x.ElemType, v)
				if e != nil {
					return nil, e
				}
				out[k] = a
			}
			v, d := types.MapValue(x.ElemType, out)
			if d.HasError() {
				return nil, fmt.Errorf("invalid map response")
			}
			return v, nil
		}
	case types.ObjectType:
		if obj, ok := raw.(map[string]any); ok {
			out := map[string]attr.Value{}
			for k, t := range x.AttrTypes {
				a, e := fromJSON(ctx, t, obj[k])
				if e != nil {
					return nil, e
				}
				out[k] = a
			}
			v, d := types.ObjectValue(x.AttrTypes, out)
			if d.HasError() {
				return nil, fmt.Errorf("invalid object response")
			}
			return v, nil
		}
	}
	return nil, fmt.Errorf("API value does not match expected Terraform type %s", t.String())
}
func textValue(values map[string]attr.Value, name string) string {
	if s, ok := values[name].(types.String); ok {
		return s.ValueString()
	}
	return ""
}
func endpoint(template string, values map[string]attr.Value) (string, error) {
	for strings.Contains(template, "{") {
		i := strings.Index(template, "{")
		j := strings.Index(template[i:], "}")
		if j < 0 {
			return "", fmt.Errorf("invalid endpoint template")
		}
		j += i
		name := template[i+1 : j]
		v := textValue(values, name)
		if v == "" || v == "." || v == ".." || strings.ContainsAny(v, "/\\") {
			return "", fmt.Errorf("%s must be a nonempty path identifier without slashes", name)
		}
		template = template[:i] + url.PathEscape(v) + template[j+1:]
	}
	return template, nil
}
