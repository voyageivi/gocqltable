// This package provides some punk-rock reflection which is not in the stdlib.
package reflect

import (
	"fmt"
	r "reflect"
	"strings"
	"sync"
)

var interfaceType = r.TypeOf((*CassandraEncDec)(nil)).Elem()

// StructToMap converts a struct to map. The object's default key string
// is the struct field name but can be specified in the struct field's
// tag value. The "cql" key in the struct field's tag value is the key
// name. Examples:
//
//	// Field appears in the resulting map as key "myName".
//	Field int `cql:"myName"`
//
//	// Field appears in the resulting as key "Field"
//	Field int
//
//	// Field appears in the resulting map as key "myName"
//	Field int "myName"
func StructToMap(val interface{}) (map[string]interface{}, bool) {
	// indirect so function works with both structs and pointers to them
	structVal := r.Indirect(r.ValueOf(val))
	kind := structVal.Kind()
	if kind != r.Struct {
		return nil, false
	}
	sinfo := getStructInfo(structVal)
	mapVal := make(map[string]interface{}, len(sinfo.FieldsList))
	for _, field := range sinfo.FieldsList {
		if structVal.Field(field.Num).CanInterface() {
			val := structVal.Field(field.Num).Interface()
			mVal := r.TypeOf(val)

			if mVal.Kind() == r.Ptr && mVal.Implements(interfaceType) {
				var strVal string
				if bs, err := (val.(CassandraEncDec)).MarshalCas(); err == nil {
					strVal = string(bs)
				}
				mapVal[field.Key] = strVal
			} else if mVal.Kind() == r.Struct && r.New(mVal).Type().Implements(interfaceType) {
				var strVal string

				rv := r.New(mVal)
				rv.Elem().Set(r.ValueOf(val))
				if bs, err := (rv.Interface().(CassandraEncDec)).MarshalCas(); err == nil {
					strVal = string(bs)
				}
				mapVal[field.Key] = strVal
			} else {
				mapVal[field.Key] = val
			}
		}
	}
	return mapVal, true
}

// MapToStruct converts a map to a struct. It is the inverse of the StructToMap
// function. For details see StructToMap.
func MapToStruct(m map[string]interface{}, struc interface{}) error {
	val := r.Indirect(r.ValueOf(struc))
	sinfo := getStructInfo(val)
	for k, v := range m {
		if info, ok := sinfo.FieldsMap[k]; ok {
			structField := val.Field(info.Num)
			if structField.Type().Name() == r.TypeOf(v).Name() {
				structField.Set(r.ValueOf(v))
			} else if r.TypeOf(v).Name() == "string" {
				interfaceType := r.TypeOf((*CassandraEncDec)(nil)).Elem()

				if structField.Kind() == r.Ptr && structField.Type().Implements(interfaceType) {
					newType := r.New(structField.Type().Elem())
					obj := newType.Interface()
					(obj.(CassandraEncDec)).UnmarshalCas([]byte(v.(string)))
					structField.Set(r.ValueOf(obj))
				} else if structField.Kind() == r.Struct {

					newType := r.New(structField.Type())
					if !newType.Type().Implements(interfaceType) {
						continue
					}
					obj := newType.Interface()
					(obj.(CassandraEncDec)).UnmarshalCas([]byte(v.(string)))
					val := r.ValueOf(obj).Elem()
					structField.Set(val)
				}

			}
		}
	}
	return nil
}

// FieldsAndValues returns a list field names and a corresponing list of values
// for the given struct. For details on how the field names are determined please
// see StructToMap.
func FieldsAndValues(val interface{}) ([]string, []interface{}, bool) {
	// indirect so function works with both structs and pointers to them
	structVal := r.Indirect(r.ValueOf(val))
	kind := structVal.Kind()
	if kind != r.Struct {
		return nil, nil, false
	}
	sinfo := getStructInfo(structVal)
	fields := make([]string, len(sinfo.FieldsList))
	values := make([]interface{}, len(sinfo.FieldsList))
	for i, info := range sinfo.FieldsList {
		field := structVal.Field(info.Num)
		fields[i] = info.Key
		values[i] = field.Interface()
	}
	return fields, values, true
}

var structMapMutex sync.RWMutex
var structMap = make(map[r.Type]*structInfo)

type fieldInfo struct {
	Key string
	Num int
}

type structInfo struct {
	// FieldsMap is used to access fields by their key
	FieldsMap map[string]fieldInfo
	// FieldsList allows iteration over the fields in their struct order.
	FieldsList []fieldInfo
}

func getStructInfo(v r.Value) *structInfo {
	st := r.Indirect(v).Type()
	structMapMutex.RLock()
	sinfo, found := structMap[st]
	structMapMutex.RUnlock()
	if found {
		return sinfo
	}

	n := st.NumField()
	fieldsMap := make(map[string]fieldInfo, n)
	fieldsList := make([]fieldInfo, 0, n)
	for i := 0; i != n; i++ {
		field := st.Field(i)
		info := fieldInfo{Num: i}
		tag := field.Tag.Get("cql")
		// If there is no cql specific tag and there are no other tags
		// set the cql tag to the whole field tag
		if tag == "" && strings.Index(string(field.Tag), ":") < 0 {
			tag = string(field.Tag)
		}
		if tag == "-" {
			continue
		} else if tag != "" {
			info.Key = tag
		} else {
			info.Key = field.Name
		}

		if _, found = fieldsMap[info.Key]; found {
			msg := fmt.Sprintf("Duplicated key '%s' in struct %s", info.Key, st.String())
			panic(msg)
		}

		fieldsList = append(fieldsList, info)
		fieldsMap[info.Key] = info
	}
	sinfo = &structInfo{
		fieldsMap,
		fieldsList,
	}
	structMapMutex.Lock()
	structMap[st] = sinfo
	structMapMutex.Unlock()
	return sinfo
}
