package struct2csv

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"
)

// WriteCSV writes a csv response file and sets headers
//
// Content-Type: text/csv
//
// Content-Disposition: attachment; filename=yourfilename
//
// for now it handles direct properties of struct and 1 level more Example:
//
// to ignore fields give it csv tag "-"
//
//	type Model struct {
//		ID               uuid.UUID    `csv:"-"`
//		Type             TypeValue    `csv:"النوع"`
//		Amount           float64      `csv:"القيمة"`
//		Reference        *string      `csv:"المرجع"`
//		Notes            *string      `csv:"ملاحظات"`
//		CreatedAt        time.Time    `csv:"وقت الانشاء"`
//		UpdatedAt        time.Time    `csv:"وقت التحديث"`
//
//		User        WalletUser `csv:"المستخدم"`
//		RechargedBy WalletUser `csv:"شحن بواسطة"`
//	}
//
//	type WalletUser struct {
//		Name  *string    `csv:"الاسم"`
//		Phone *string    `csv:"الهاتف"`
//		Email *string    `csv:"الايميل"`
//	}
func WriteCSV(
	h http.Header,
	w http.ResponseWriter,
	filename string,
	data any,
) error {
	h.Set("Content-Type", "text/csv")
	h.Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	value := reflect.ValueOf(data)
	if value.Kind() != reflect.Slice {
		return errors.New("data is not a slice")
	}

	elemType := value.Type().Elem()
	isPointer := elemType.Kind() == reflect.Ptr
	if isPointer {
		elemType = elemType.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		return errors.New("slice elements are not structs")
	}

	var headers []string
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		if tag := field.Tag.Get("csv"); tag != "-" {
			if field.Type.Kind() == reflect.Struct &&
				field.Type != reflect.TypeOf(time.Time{}) &&
				!field.Anonymous {
				for j := 0; j < field.Type.NumField(); j++ {
					subField := field.Type.Field(j)
					if subTag := subField.Tag.Get("csv"); subTag != "-" {
						addedTag := fmt.Sprintf("%s.%s", tag, subTag)
						headers = append(headers, addedTag)
					}
				}
			} else {
				headers = append(headers, tag)
			}
		}
	}

	if err := writer.Write(headers); err != nil {
		return errors.New("failed to write CSV headers")
	}

	for i := 0; i < value.Len(); i++ {
		row := []string{}
		elem := value.Index(i)
		if isPointer {
			elem = elem.Elem()
		}
		for j := 0; j < elemType.NumField(); j++ {
			field := elem.Field(j)
			fieldType := elemType.Field(j)
			csvTag := fieldType.Tag.Get("csv")
			if csvTag == "-" {
				continue
			}
			if fieldType.Type.Kind() == reflect.Struct &&
				fieldType.Type != reflect.TypeOf(time.Time{}) &&
				!fieldType.Anonymous {
				// Handle sub-structs
				for k := 0; k < field.NumField(); k++ {
					subField := field.Field(k)
					subFieldType := fieldType.Type.Field(k)

					csvTag := subFieldType.Tag.Get("csv")
					if csvTag == "-" {
						continue
					}
					row = append(row, formatValue(subField))
				}
			} else {
				// Handle regular fields
				row = append(row, formatValue(field))
			}
		}
		if err := writer.Write(row); err != nil {
			return errors.New("failed to write CSV row")
		}
	}
	return nil
}

func formatValue(value reflect.Value) string {
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.String:
		return value.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'f', -1, 64)
	case reflect.Bool:
		return strconv.FormatBool(value.Bool())
	case reflect.Struct:
		if value.Type() == reflect.TypeOf(time.Time{}) {
			return value.Interface().(time.Time).Format(
				"2006-01-02 15:04",
			)
		}
		return ""
	default:
		return ""
	}
}
