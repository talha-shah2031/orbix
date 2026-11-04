package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

func writeOutput(writer io.Writer, format string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	var normalized any
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&normalized); err != nil {
		return fmt.Errorf("normalize output: %w", err)
	}
	switch format {
	case "json":
		return writeJSON(writer, encoded)
	case "ndjson":
		if values, ok := normalized.([]any); ok {
			for _, item := range values {
				if err := writeJSONValue(writer, item); err != nil {
					return err
				}
			}
			return nil
		}
		return writeJSONValue(writer, normalized)
	case "yaml":
		yamlBytes, err := yaml.Marshal(normalized)
		if err != nil {
			return fmt.Errorf("encode YAML: %w", err)
		}
		_, err = writer.Write(yamlBytes)
		return err
	case "csv":
		return writeCSV(writer, normalized)
	case "table", "":
		return writeTable(writer, normalized)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeJSON(writer io.Writer, data []byte) error {
	_, err := fmt.Fprintf(writer, "%s\n", data)
	return err
}
func writeJSONValue(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeJSON(writer, data)
}

func writeCSV(writer io.Writer, value any) error {
	rows, columns := tabular(value)
	if len(columns) == 0 {
		return fmt.Errorf("CSV output requires object data")
	}
	encoder := csv.NewWriter(writer)
	if err := encoder.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, len(columns))
		for index, column := range columns {
			record[index] = display(row[column])
		}
		if err := encoder.Write(record); err != nil {
			return err
		}
	}
	encoder.Flush()
	return encoder.Error()
}

func writeTable(writer io.Writer, value any) error {
	if object, ok := value.(map[string]any); ok {
		keys := sortedKeys(object)
		for _, key := range keys {
			if _, err := fmt.Fprintf(writer, "%s\t%s\n", key, display(object[key])); err != nil {
				return err
			}
		}
		return nil
	}
	rows, columns := tabular(value)
	if len(columns) == 0 {
		_, err := fmt.Fprintln(writer, display(value))
		return err
	}
	widths := make([]int, len(columns))
	for index, column := range columns {
		widths[index] = len(column)
	}
	for _, row := range rows {
		for index, column := range columns {
			if length := len(display(row[column])); length > widths[index] {
				widths[index] = length
			}
		}
	}
	writeRow := func(values []string) error {
		for index, value := range values {
			if index > 0 {
				if _, err := io.WriteString(writer, "  "); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "%-*s", widths[index], value); err != nil {
				return err
			}
		}
		_, err := io.WriteString(writer, "\n")
		return err
	}
	if err := writeRow(columns); err != nil {
		return err
	}
	for index := range widths {
		widths[index] = max(widths[index], len(columns[index]))
	}
	for _, row := range rows {
		values := make([]string, len(columns))
		for i, column := range columns {
			values[i] = display(row[column])
		}
		if err := writeRow(values); err != nil {
			return err
		}
	}
	return nil
}

func tabular(value any) ([]map[string]any, []string) {
	var objects []any
	switch typed := value.(type) {
	case []any:
		objects = typed
	default:
		if array, ok := value.([]map[string]any); ok {
			for _, item := range array {
				objects = append(objects, item)
			}
		} else {
			objects = []any{value}
		}
	}
	rows := make([]map[string]any, 0, len(objects))
	columnSet := make(map[string]struct{})
	for _, object := range objects {
		if fields, ok := object.(map[string]any); ok {
			rows = append(rows, fields)
			for key := range fields {
				columnSet[key] = struct{}{}
			}
			continue
		}
		return nil, nil
	}
	columns := make([]string, 0, len(columnSet))
	for key := range columnSet {
		columns = append(columns, key)
	}
	sort.Strings(columns)
	return rows, columns
}

func sortedKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func display(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return string(typed)
	case bool:
		return fmt.Sprint(typed)
	case []any:
		values := make([]string, len(typed))
		for index, item := range typed {
			values[index] = display(item)
		}
		return strings.Join(values, ",")
	}
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Map {
		encoded, _ := json.Marshal(value)
		return string(encoded)
	}
	return fmt.Sprint(value)
}
