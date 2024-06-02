package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

const (
	templateNotAuthorized  = "noauth"
	templateErrorForbidden = "error403"
	templatePlainMessage   = "plainmsg"
)

func basicLayoutHandler(page string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		basicLayoutLookupRespond(page, w, r, nil)
	}
}

func basicLayoutLookupRespond(page string, w http.ResponseWriter, r *http.Request, p any) {
	in := layouts.Lookup(page)
	if in != nil {
		var params map[string]any
		if p == nil {
			params = map[string]any{}
		} else {
			m, mk := p.(map[string]any)
			if !mk {
				log.Println("Basic respond got parameters interface of wrong type")
			} else {
				params = m
			}
		}
		params["NavWhere"] = page
		if strings.HasPrefix(r.Host, "dev.") {
			params["IsDevWebsite"] = true
		}
		params["IsEloRecalculating"] = isEloRecalculating.Load()
		sessionAppendUser(r, params)
		w.Header().Set("Server", "TacticalPepe webserver "+CommitHash)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
		err := in.Execute(w, params)
		if err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}

func basicLayoutLookupExecuteAnonymus(in *template.Template, p any) string {
	m, mk := p.(map[string]any)
	if !mk {
		log.Println("Basic respond got parameters interface of wrong type")
	}
	var tpl bytes.Buffer
	err := in.Execute(&tpl, m)
	if err != nil {
		log.Println(err)
	}
	return tpl.String()
}

var layoutFuncs = template.FuncMap{
	"noescape": func(s string) template.HTML {
		return template.HTML(s)
	},
	"inc": func(i any) any {
		switch v := i.(type) {
		case float32:
			return v + 1
		case float64:
			return v + 1
		case int:
			return v + 1
		case int8:
			return v + 1
		case int16:
			return v + 1
		case int32:
			return v + 1
		case int64:
			return v + 1
		}
		return nil
	},
	"dec": func(i any) any {
		switch v := i.(type) {
		case float32:
			return v - 1
		case float64:
			return v - 1
		case int:
			return v - 1
		case int8:
			return v - 1
		case int16:
			return v - 1
		case int32:
			return v - 1
		case int64:
			return v - 1
		}
		return nil
	},
	"sum": func(a int, b int) int {
		return a + b
	},
	"sub": func(a int, b int) int {
		return a - b
	},
	"div": func(a int, b int) int {
		return a / b
	},
	"divtf64": func(a int, b int) float64 {
		return float64(a) / float64(b)
	},
	"divf64": func(a float64, b float64) float64 {
		return a / b
	},
	"mult": func(a int, b int) int {
		return a * b
	},
	"multf64": func(a float64, b float64) float64 {
		return a * b
	},
	"rem": func(a int, b int) int {
		return a % b
	},
	"allianceToClass": func(a float64) float64 {
		if a == 3 {
			return 1
		} else {
			return a
		}
	},
	"allianceToClassI": templatesAllianceToClassI,
	"boolto10": func(a bool) int {
		if !a {
			return 0
		} else {
			return 1
		}
	},
	"f64tostring": func(a float64) string {
		return fmt.Sprintf("%.2f", a)
	},
	"avail": func(name string, data any) bool {
		v := reflect.ValueOf(data)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		m, ok := data.(map[string]any)
		if ok {
			_, ok := m[name]
			return ok
		}
		if v.Kind() != reflect.Struct {
			return false
		}
		return v.FieldByName(name).IsValid()
	},
	"GameTimeToString":  GameTimeToString,
	"GameTimeToStringI": GameTimeToStringI,
	"GameDirToWeek":     GameDirToWeek,
	"InstanceIDToWeek":  InstanceIDToWeek,
	"strcut": func(str string, num int) string { // https://play.golang.org/p/EzvhWMljku
		bnoden := str
		if len(str) > num {
			if num > 3 {
				num -= 3
			}
			bnoden = str[0:num] + "..."
		}
		return bnoden
	},
	"FormatBytes":   ByteCountIEC,
	"FormatPercent": FormatPercent,
	"tostr": func(val any) string {
		if d, ok := val.(uint32); ok {
			return fmt.Sprint(d)
		}
		if d, ok := val.(float64); ok {
			return fmt.Sprint(d)
		}
		return "snan"
	},
	"datefmt": func(val any) string {
		switch v := val.(type) {
		case time.Time:
			return v.Format("15:04 02 Jan 2006")
		case *time.Time:
			if v == nil {
				return "??:??"
			}
			return v.Format("15:04 02 Jan 2006")
		default:
			return "not a timestamp"
		}
	},
	"base64encode": func(val []uint8) string {
		return base64.StdEncoding.EncodeToString(val)
	},
}

func templatesAllianceToClassI(a int) int {
	if a == 3 {
		return 1
	} else {
		return a
	}
}
