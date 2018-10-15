package querying

import (
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/mgo.v2/bson"
)

func castToType(v string, t string) interface{} {
	if v == "" {
		return nil
	}

	switch t {
	case "int":
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	case "bson.ObjectId":
		return bson.ObjectIdHex(v)
	case "bool":
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}

	return v
}

func generateQueryClause(operator string, values []string, fieldType string) map[string]interface{} {
	// {"$eq": values}
	querySegment := make(map[string]interface{})
	newValues := make([]interface{}, len(values), len(values)+1)

	for i, v := range values {
		newValues[i] = castToType(v, fieldType)
	}

	if len(values) > 1 {
		querySegment[operator] = newValues
	} else {
		querySegment[operator] = newValues[0]
	}

	return bson.M(querySegment)
}

func GenerateQueryFromMultivaluedMap(queryMap map[string][]string, collectionType interface{}) interface{} {
	// bson.M{"status": "A", qty: { $lt: 30 } }
	query := make(map[string]interface{})
	allowedFields := make(map[string]string)
	val := reflect.ValueOf(collectionType)

	// reflect base class
	for i := 0; i < val.Type().NumField(); i++ {
		field := val.Type().Field(i)
		allowedFields[field.Tag.Get("json")] = field.Type.String()
	}

	// generate clause for each element in query map
	for k, v := range queryMap {
		clause := strings.SplitN(k, "__", 2)
		query[clause[0]] = generateQueryClause(clause[1], v, allowedFields[clause[0]])
	}

	return bson.M(query)
}
