package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type RegisterOption struct {
	Name        string
	Description string
}

func (t *Toolkit) RegisterFunc(fn interface{}, opts ...RegisterOption) error {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return fmt.Errorf("RegisterFunc requires a function, got %T", fn)
	}

	ft := v.Type()
	if ft.NumIn() != 2 {
		return fmt.Errorf("function must have 2 inputs (context.Context, struct), got %d", ft.NumIn())
	}
	if ft.NumOut() != 2 {
		return fmt.Errorf("function must have 2 outputs (*ToolResponse, error), got %d", ft.NumOut())
	}

	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if !ft.In(0).Implements(ctxType) {
		return fmt.Errorf("first argument must be context.Context")
	}

	retErrType := reflect.TypeOf((*error)(nil)).Elem()
	if !ft.Out(1).Implements(retErrType) {
		return fmt.Errorf("second return must be error")
	}

	name := ""
	description := ""
	for _, opt := range opts {
		if opt.Name != "" {
			name = opt.Name
		}
		if opt.Description != "" {
			description = opt.Description
		}
	}

	if name == "" {
		name = extractFuncName(ft)
	}

	argType := ft.In(1)
	params := buildSchemaFromStruct(argType)

	wrapper := func(ctx context.Context, args map[string]interface{}) (*ToolResponse, error) {
		argVal := reflect.New(argType).Elem()
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("marshal args: %w", err)
		}
		if err := json.Unmarshal(raw, argVal.Addr().Interface()); err != nil {
			return nil, fmt.Errorf("unmarshal args to struct: %w", err)
		}

		out := v.Call([]reflect.Value{reflect.ValueOf(ctx), argVal})
		if !out[1].IsNil() {
			errVal := out[1].Interface().(error)
			return nil, errVal
		}
		return out[0].Interface().(*ToolResponse), nil
	}

	return t.Register(name, description, params, wrapper)
}

func extractFuncName(ft reflect.Type) string {
	name := ft.Name()
	if name == "" {
		name = "anonymous_func"
	}
	name = strings.TrimSuffix(name, "Func")
	name = strings.TrimSuffix(name, "Tool")
	return toSnakeCase(name)
}

func buildSchemaFromStruct(t reflect.Type) map[string]interface{} {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return map[string]interface{}{
			"type": "object",
		}
	}

	properties := make(map[string]interface{})
	required := make([]string, 0)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		fieldName := field.Name
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		desc := field.Tag.Get("description")
		prop := map[string]interface{}{
			"type": goTypeToJSONType(field.Type),
		}
		if desc != "" {
			prop["description"] = desc
		}
		properties[fieldName] = prop

		omitempty := strings.Contains(jsonTag, ",omitempty")
		if !omitempty {
			required = append(required, fieldName)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func goTypeToJSONType(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map:
		return "object"
	default:
		return "string"
	}
}

func toSnakeCase(s string) string {
	var result []byte
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(r+32))
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}
