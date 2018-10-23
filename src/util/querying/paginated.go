package querying

import (
	"errors"
	"strconv"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type PaginatedList struct {
	Results interface{} `json:"results"`
	Start   int         `json:"start"`
	End     int         `json:"end"`
	Total   int         `json:"total"`
}

func ExtractStartEnd(queryParams map[string][]string) (start, end int64, err error) {
	startV, hasStart := queryParams["start"]

	if !hasStart || len(startV) != 1 {
		start = 0
	} else {
		start, err = strconv.ParseInt(startV[0], 10, 0)

		if err != nil {
			return 0, 0, errors.New("Query param 'start' not formatted properly")
		}
	}

	endV, hasEnd := queryParams["end"]

	if !hasEnd || len(endV) != 1 {
		end = 10
	} else {
		end, err = strconv.ParseInt(endV[0], 10, 0)

		if err != nil {
			return 0, 0, errors.New("Query param 'end' not formatted properly")
		}
	}

	return start, end, nil
}

func GetPaginatedList(
	collection *mgo.Collection,
	schemaTypeSlice interface{},
	start, end int,
	clauses map[string]interface{},
) (results *PaginatedList, err error) {
	query := collection.Find(bson.M(clauses))
	total, err := query.Count()

	if err != nil {
		return results, err
	}

	err = query.Skip(start).Limit(end).All(schemaTypeSlice)

	if err != nil {
		return results, err
	}

	results = &PaginatedList{
		Results: schemaTypeSlice,
		Start:   start,
		End:     end,
		Total:   total,
	}

	return results, nil
}
