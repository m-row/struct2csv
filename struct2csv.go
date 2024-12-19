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
	// Set headers for CSV download
	h.Set("Content-Type", "text/csv")
	h.Set(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, filename),
	)

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

	// Generate headers
	headers, err := extractHeaders(elemType)
	if err != nil {
		return fmt.Errorf("failed to extract headers: %w", err)
	}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write headers: %w", err)
	}

	// Write rows
	for i := 0; i < value.Len(); i++ {
		elem := value.Index(i)
		if isPointer {
			elem = elem.Elem()
		}

		row, err := extractRow(elem, elemType)
		if err != nil {
			return fmt.Errorf("failed to extract row %d: %w", i, err)
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row %d: %w", i, err)
		}
	}

	return nil
}

// extractHeaders generates CSV headers from struct tags
func extractHeaders(elemType reflect.Type) ([]string, error) {
	var headers []string
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		if isIgnoredField(field) {
			continue
		}

		csvTag := field.Tag.Get("csv")
		if isSubStruct(field) {
			subHeaders, err := extractHeaders(field.Type)
			if err != nil {
				return nil, err
			}
			for _, subHeader := range subHeaders {
				headers = append(
					headers,
					fmt.Sprintf("%s.%s", csvTag, subHeader),
				)
			}
		} else {
			headers = append(headers, csvTag)
		}
	}
	return headers, nil
}

// extractRow generates a CSV row from a struct value
func extractRow(value reflect.Value, elemType reflect.Type) ([]string, error) {
	var row []string
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		if isIgnoredField(field) {
			continue
		}

		fieldValue := value.Field(i)
		if isSubStruct(field) {
			subRow, err := extractRow(fieldValue, field.Type)
			if err != nil {
				return nil, err
			}
			row = append(row, subRow...)
		} else {
			row = append(row, formatValue(fieldValue))
		}
	}
	return row, nil
}

// isIgnoredField Helper to check if a field should be ignored
func isIgnoredField(field reflect.StructField) bool {
	return field.Tag.Get("csv") == "-"
}

// isSubStruct Helper to check if a field is a sub-struct (non-time struct)
func isSubStruct(field reflect.StructField) bool {
	return field.Type.Kind() == reflect.Struct &&
		field.Type != reflect.TypeOf(time.Time{}) &&
		!field.Anonymous
}

// formatValue formats a field value into a string for CSV
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
			return value.Interface().(time.Time).Format("2006-01-02 15:04")
		}
		return ""
	default:
		return ""
	}
}
