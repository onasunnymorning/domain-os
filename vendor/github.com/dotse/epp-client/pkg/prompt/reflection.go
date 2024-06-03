// Copyright (c) 2022 The Swedish Internet Foundation
//
// Distributed under the MIT License. (See accompanying LICENSE file or copy at
// <https://opensource.org/licenses/MIT>.)

package prompt

import (
	"fmt"
	"reflect"
	"strconv"
)

// FieldValue the values of a field in a struct.
type FieldValue struct {
	Name     string
	Value    reflect.Value
	Type     reflect.Type
	Kind     reflect.Kind
	ListKind reflect.Kind
	ListType reflect.Type
}

// ValueAsString returns the value as string.
func (f *FieldValue) ValueAsString() string {
	switch f.Kind {
	case reflect.Int:
		return strconv.Itoa(int(f.Value.Int()))

	case reflect.Slice:
		if f.ListKind == reflect.Pointer {
			var ret []string

			for i := 0; i < f.Value.Len(); i++ {
				// dereference to make the output prettier
				ret = append(ret, fmt.Sprintf("%+v", f.Value.Index(i).Elem()))
			}

			return fmt.Sprintf("%+v", ret)
		}

		fallthrough

	case reflect.Struct, reflect.Bool:
		return fmt.Sprintf("%+v", f.Value)

	case reflect.Ptr:
		if f.Value.IsNil() {
			return "<empty>"
		}
		// dereference to make the output prettier
		return fmt.Sprintf("%+v", f.Value.Elem())

	default:
	}

	return f.Value.String()
}

// ListValuesAsString returns all values in a list as strings.
func (f *FieldValue) ListValuesAsString() []string {
	if f.Value.Len() == 0 {
		return []string{}
	}

	ret := []string{}

	switch f.ListKind {
	case reflect.Int:
		for i := 0; i < f.Value.Len(); i++ {
			ret = append(ret, strconv.Itoa(int(f.Value.Index(i).Int())))
		}

	case reflect.Struct:
		for i := 0; i < f.Value.Len(); i++ {
			ret = append(ret, fmt.Sprintf("%+v", f.Value.Index(i)))
		}

	case reflect.Ptr:
		for i := 0; i < f.Value.Len(); i++ {
			// dereference to make the output prettier
			ret = append(ret, fmt.Sprintf("%+v", f.Value.Index(i).Elem()))
		}

	case reflect.Bool:
		for i := 0; i < f.Value.Len(); i++ {
			// dereference to make the output prettier
			ret = append(ret, fmt.Sprintf("%t", f.Value.Index(i).Bool()))
		}

	default:
		for i := 0; i < f.Value.Len(); i++ {
			ret = append(ret, f.Value.Index(i).String())
		}
	}

	return ret
}

// RemoveAtIndex removes the value at the given index in a list.
func (f *FieldValue) RemoveAtIndex(index int) {
	if f.Value.Len() == 0 {
		return
	}

	// make a new slice of the correct size
	dst := reflect.MakeSlice(f.Value.Type(), f.Value.Len()-1, f.Value.Len()-1)
	idx := 0

	// copy over all values that we still want
	for i := 0; i < f.Value.Len(); i++ {
		if i == index {
			continue
		}

		dst.Index(idx).Set(f.Value.Index(i))
		idx++
	}

	f.Value.Set(dst)
}

// AddToStructList add new data to the current list.
func (f *FieldValue) AddToStructList(newData any) {
	// make a new slice of the correct size
	dst := reflect.MakeSlice(f.Value.Type(), f.Value.Len()+1, f.Value.Len()+1)
	reflect.Copy(dst, f.Value)

	dst.Index(f.Value.Len()).Set(reflect.ValueOf(newData))

	f.Value.Set(dst)
}

// NewOfListType returns a pointer to an empty item of the type in the current list.
func (f *FieldValue) NewOfListType() any {
	if f.ListKind == reflect.Struct {
		panic("only pointers in lists allowed")
	}

	return reflect.New(f.ListType.Elem()).Interface()
}

// DereferencePointer return the FieldValue for the underlying value of the pointer.
func DereferencePointer(parent any, fv *FieldValue) *FieldValue {
	dataValue := reflect.ValueOf(parent)
	dataType := reflect.TypeOf(parent)

	if dataType.Kind() == reflect.Ptr {
		// dereference to handle the data and not the pointer
		dataType = dataType.Elem()
		dataValue = dataValue.Elem()
	}

	for i := 0; i < dataType.NumField(); i++ {
		// find correct field
		if dataType.Field(i).Tag.Get("xml") != fv.Name {
			continue
		}

		if dataValue.Field(i).Kind() != reflect.Pointer {
			panic("not pointer")
		}

		if dataValue.Field(i).IsNil() {
			// initialize if zero value
			dataValue.Field(i).Set(reflect.New(fv.Type.Elem()))
		}

		fieldValue := dataValue.Field(i).Elem()
		ret := &FieldValue{
			Name:  fv.Name,
			Value: fieldValue,
			Type:  fieldValue.Type(),
			Kind:  fieldValue.Kind(),
		}

		if dataValue.Kind() == reflect.Slice {
			fv.ListKind = fieldValue.Type().Elem().Kind()
			fv.ListType = fieldValue.Type().Elem()
		}

		return ret
	}

	panic("not found")
}

// GetFieldFromList return the field at the given index in the given list.
func GetFieldFromList(idx int, list any) any {
	dataValue := reflect.ValueOf(list)

	return dataValue.Index(idx).Interface()
}

// InitializeIfNil initialize the given field in the given data if it's nil.
func InitializeIfNil(parent any, fv *FieldValue) {
	dataValue := reflect.ValueOf(parent)
	dataType := reflect.TypeOf(parent)

	switch dataType.Kind() {
	case reflect.Slice:
		// no nil slices
		return

	case reflect.Ptr:
		// dereference to handle the data and not the pointer
		dataType = dataType.Elem()
		dataValue = dataValue.Elem()

	default:
	}

	for i := 0; i < dataType.NumField(); i++ {
		if dataType.Field(i).Tag.Get("xml") != fv.Name {
			continue
		}

		if dataValue.Field(i).Kind() != reflect.Pointer {
			return
		}

		if dataValue.Field(i).IsNil() {
			dataValue.Field(i).Set(reflect.New(fv.Type.Elem()))
		}

		return
	}
}

// GetFromTag get a field from a struct given the xml tag.
func GetFromTag(parent any, tag string) any {
	dataValue := reflect.ValueOf(parent)
	dataType := reflect.TypeOf(parent)

	if dataType.Kind() == reflect.Pointer {
		// dereference to handle the data and not the pointer
		dataType = dataType.Elem()
		dataValue = dataValue.Elem()
	}

	for i := 0; i < dataType.NumField(); i++ {
		if dataType.Field(i).Tag.Get("xml") == tag {
			return dataValue.Field(i).Interface()
		}
	}

	panic("not found")
}

// SetValue set the given value at the given field in the given data.
func SetValue(parent any, tag string, newValue any) {
	dataValue := reflect.ValueOf(parent)
	dataType := reflect.TypeOf(parent)

	if dataType.Kind() == reflect.Pointer {
		// dereference to handle the data and not the pointer
		dataType = dataType.Elem()
		dataValue = dataValue.Elem()
	}

	for i := 0; i < dataType.NumField(); i++ {
		fieldValue := dataValue.Field(i)
		fieldType := dataType.Field(i)

		if fieldType.Tag.Get("xml") != tag {
			continue
		}

		if !fieldValue.CanSet() {
			panic("can't set data")
		}

		fieldKind := fieldValue.Kind()

		if fieldValue.Kind() == reflect.Pointer {
			if dataValue.Field(i).IsNil() {
				dataValue.Field(i).Set(reflect.New(fieldType.Type.Elem()))
			}

			fieldValue = fieldValue.Elem()
			fieldKind = fieldType.Type.Elem().Kind()
		}

		newValueValue := reflect.ValueOf(newValue)
		if newValueValue.Kind() != fieldKind {
			panic(fmt.Sprintf("not the same type %v, %v", newValueValue.Kind(), fieldValue.Kind()))
		}

		switch fieldKind {
		case reflect.String:
			fieldValue.SetString(newValue.(string))

		case reflect.Int:
			fieldValue.SetInt(int64(newValue.(int)))

		case reflect.Bool:
			fieldValue.SetBool(newValue.(bool))

		case reflect.Slice:
			if fieldType.Type != reflect.TypeOf(newValue) {
				panic("not same type of slice")
			}

			// make a new slice with the correct length
			dst := reflect.MakeSlice(fieldType.Type, newValueValue.Len(), newValueValue.Len())
			reflect.Copy(dst, newValueValue)
			fieldValue.Set(dst)

		default:
			panic(fmt.Sprintf("currently unhandled kind, %v", fieldValue.Kind()))
		}

		return
	}
}

// GetFieldValues get all fields in the given data as FieldValue.
func GetFieldValues(data any) []*FieldValue {
	ret := []*FieldValue{}

	dataValue := reflect.ValueOf(data)
	dataType := reflect.TypeOf(data)

	switch dataType.Kind() {
	case reflect.Ptr:
		// dereference to handle the data and not the pointer
		dataType = dataType.Elem()
		dataValue = dataValue.Elem()

	case reflect.Slice:
		// slices are special case since they don't have tags for their elements
		for i := 0; i < dataValue.Len(); i++ {
			ret = append(ret, &FieldValue{
				Value: dataValue.Index(i),
				Type:  dataType,
				Kind:  dataType.Elem().Kind(),
			})
		}

		return ret

	default:
	}

	// iterate all fields in the struct
	for i := 0; i < dataType.NumField(); i++ {
		fieldType := dataType.Field(i)
		fieldValue := dataValue.Field(i)

		fv := &FieldValue{
			Name:  fieldType.Tag.Get("xml"),
			Value: fieldValue,
			Type:  fieldType.Type,
			Kind:  fieldValue.Kind(),
		}

		if fieldValue.Kind() == reflect.Slice {
			fv.ListKind = fieldType.Type.Elem().Kind()
			fv.ListType = fieldType.Type.Elem()
		}

		ret = append(ret, fv)
	}

	return ret
}
