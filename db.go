package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/georgysavva/scany/pgxscan"
)

type genericRequestParams struct {
	tableName               string
	limitClamp              int
	columnMappings          map[string]string
	sortDefaultOrder        string
	sortDefaultColumn       string
	sortColumns             []string
	filterColumnsFull       []string
	filterColumnsStartsWith []string
	searchColumn            string
	searchSimilarity        float64
	addWhereCase            string
}

func genericViewRequest[T any](r *http.Request, params genericRequestParams) (int, any) {
	reqLimit := max(1, parseQueryInt(r, "limit", 50))
	if reqLimit > params.limitClamp {
		reqLimit = 500
	}
	reqOffset := max(0, parseQueryInt(r, "offset", 0))
	reqSortOrder := parseQueryStringFiltered(r, "order", "desc", "asc")
	if params.sortDefaultOrder != "asc" {
		reqSortOrder = parseQueryStringFiltered(r, "order", "asc", "desc")
	}
	reqSortField := parseQueryStringFiltered(r, "sort", params.sortDefaultColumn, params.sortColumns...)
	if mapped, ok := params.columnMappings[reqSortField]; ok {
		reqSortField = mapped
	}

	wherecase := ""
	whereargs := []any{}

	reqFilterJ := parseQueryString(r, "filter", "")
	reqFilterFieldsUnmapped := map[string]string{}
	reqDoFilters := false
	if reqFilterJ != "" {
		err := json.Unmarshal([]byte(reqFilterJ), &reqFilterFieldsUnmapped)
		if err == nil && len(reqFilterFieldsUnmapped) > 0 {
			reqDoFilters = true
		}
	}

	reqFilterFields := map[string]string{}
	for k, v := range reqFilterFieldsUnmapped {
		m, ok := params.columnMappings[k]
		if ok {
			reqFilterFields[m] = v
		}
	}

	if reqDoFilters {
		for _, v := range params.filterColumnsFull {
			val, ok := reqFilterFields[v]
			if ok {
				whereargs = append(whereargs, val)
				if wherecase == "" {
					wherecase = "WHERE " + v + " = $1"
				} else {
					wherecase += " AND " + v + " = $1"
				}
			}
		}
		for _, v := range params.filterColumnsStartsWith {
			val, ok := reqFilterFields[v]
			if ok {
				whereargs = append(whereargs, val)
				if wherecase == "" {
					wherecase = "WHERE starts_with(" + v + ", $1)"
				} else {
					wherecase += fmt.Sprintf(" AND starts_with("+v+", $%d)", len(whereargs))
				}
			}
		}
	}

	reqSearch := parseQueryString(r, "search", "")
	similarity := params.searchSimilarity
	if reqSearch != "" && params.searchColumn != "" {
		whereargs = append(whereargs, reqSearch)
		if wherecase == "" {
			wherecase = fmt.Sprintf("WHERE similarity("+params.searchColumn+", $1::text) > %f or "+params.searchColumn+" ~ $1::text", similarity)
		} else {
			wherecase += fmt.Sprintf(" AND similarity("+params.searchColumn+", $%d::text) > %f or "+params.searchColumn+" ~ $1::text", len(whereargs), similarity)
		}
	}

	if params.addWhereCase != "" {
		if wherecase == "" {
			wherecase = "WHERE " + params.addWhereCase
		} else {
			wherecase += " AND " + params.addWhereCase
		}
	}

	ordercase := fmt.Sprintf("ORDER BY %s %s", reqSortField, reqSortOrder)
	limiter := fmt.Sprintf("LIMIT %d", reqLimit)
	offset := fmt.Sprintf("OFFSET %d", reqOffset)

	tn := params.tableName

	var totalsNoFilter int
	var totals int
	var rows []*T
	// log.Println(`SELECT * FROM ` + tn + ` ` + wherecase + ` ` + ordercase + ` ` + offset + ` ` + limiter)
	err := RequestMultiple(func() error {
		return dbpool.QueryRow(r.Context(), `SELECT count(`+tn+`) FROM `+tn).Scan(&totalsNoFilter)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `SELECT count(`+tn+`) FROM `+tn+` `+wherecase, whereargs...).Scan(&totals)
	}, func() error {
		return pgxscan.Select(r.Context(), dbpool, &rows, `SELECT * FROM `+tn+` `+wherecase+` `+ordercase+` `+offset+` `+limiter, whereargs...)
	})
	if err != nil {
		return 500, err
	}
	return 200, map[string]any{
		"total":            totals,
		"totalNotFiltered": totalsNoFilter,
		"rows":             rows,
	}
}
