package database

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func castToType(v, t string) interface{} {
	// NOTE: nil check?
	// NOTE: different types of ints?
	// TODO: decimal support
	switch t {
	case "int":
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	case "bool":
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}

	return v
}

func castToTypeMultiple(vals []string, t string) interface{} {
	if len(vals) < 1 {
		// NOTE: error check on nil?
		return nil
	} else if len(vals) < 2 {
		return castToType(vals[0], t)
	} else {
		castedTypes := new([]interface{})

		for _, v := range vals {
			*castedTypes = append(*castedTypes, castToType(v, t))
		}

		return *castedTypes
	}
}

func translateOp(op string) string {
	switch strings.ToLower(op) {
	case "eq":
		return "="
	case "neq":
		return "<>"
	case "is_null":
		return "IS NULL"
	case "is_not_null":
		return "IS NOT NULL"
	case "like":
		return "LIKE"
	case "in":
		return "in"
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	case "lte":
		return "<="
	default:
		return "="
	}
}

func TranslateQueryMap(queryMap map[string][]string, collectionType interface{}) (map[string]interface{}, error) {
	// bson.M{"status": "A", qty: { $lt: 30 } }
	translatedQueryMap := make(map[string]interface{})
	allowedFields := make(map[string]string)
	val := reflect.ValueOf(collectionType)

	// reflect base class
	for i := 0; i < val.Type().NumField(); i++ {
		field := val.Type().Field(i)
		allowedFields[field.Tag.Get("json")] = field.Type.String()
	}

	// generate clause for each element in query map
	for k, v := range queryMap {
		// TODO: check list for reserved names
		if k == "start" || k == "end" {
			continue
		}

		clause := strings.SplitN(k, "__", 2)

		if len(clause) < 2 {
			return nil, errors.New("Incorrecly formatted query param clause")
		}

		column := clause[0]

		if _, exists := allowedFields[column]; !exists {
			return nil, fmt.Errorf("%s is not a queryable field", column)
		}

		op := translateOp(clause[1])

		if op == "=" && len(v) > 1 {
			op = "in"
		}

		translatedQueryMap[fmt.Sprintf("%s %s", column, op)] = castToTypeMultiple(v, allowedFields[column])
	}

	return translatedQueryMap, nil
}
