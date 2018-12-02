package database

import (
	"errors"
	"strconv"
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

func ToPaginatedList(
	filledSchemas interface{},
	start, end, total int,
) *PaginatedList {
	return &PaginatedList{
		Results: filledSchemas,
		Start:   start,
		End:     end,
		Total:   total,
	}
}
