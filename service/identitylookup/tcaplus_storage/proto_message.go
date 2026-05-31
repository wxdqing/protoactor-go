package tcaplus_storage

import (
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"
)

func isMessageField(msg proto.Message, fieldName string) bool {
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	return field.IsValid()
}

func getFieldValueStringUnsafe(msg proto.Message, fieldName string) string {
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	return field.Interface().(string)
}

func getFieldValueIntUnsafe[T int | uint | uint32 | int32 | uint64 | int64](msg proto.Message, fieldName string) T {
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	switch field.Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64:
		return T(field.Int())
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		return T(field.Uint())
	default:
		panic(fmt.Sprintf("unsupported field type: %v", field.Kind()))
	}
}

func setFieldValueIntUnsafe[T int | uint | uint32 | int32 | uint64 | int64](msg proto.Message, fieldName string, value T) {
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprintf("%v", value))
	case reflect.Int, reflect.Int32, reflect.Int64:
		field.SetInt(int64(value))
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		field.SetUint(uint64(value))
	default:
		panic(fmt.Sprintf("unsupported field type: %v", field.Kind()))
	}
}

func setFieldValueStringUnsafe(msg proto.Message, fieldName string, value string) {
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	field := v.FieldByName(fieldName)
	field.SetString(value)
}
