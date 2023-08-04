package main

import (
	"bytes"
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

func basicLayoutLookupRespond(page string, w http.ResponseWriter, r *http.Request, p interface{}) {
	in := layouts.Lookup(page)
	if in != nil {
		var params map[string]interface{}
		if p == nil {
			params = map[string]interface{}{}
		} else {
			m, mk := p.(map[string]interface{})
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

func basicLayoutLookupExecuteAnonymus(in *template.Template, p interface{}) string {
	m, mk := p.(map[string]interface{})
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
	"inc": func(i int) int {
		return i + 1
	},
	"dec": func(i int) int {
		return i - 1
	},
	"decf64": func(i float64) float64 {
		return i - 1
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
	"allianceToClassI": func(a int) int {
		if a == 3 {
			return 1
		} else {
			return a
		}
	},
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
	"avail": func(name string, data interface{}) bool {
		v := reflect.ValueOf(data)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		m, ok := data.(map[string]interface{})
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
	"tostr": func(val interface{}) string {
		if d, ok := val.(uint32); ok {
			return fmt.Sprint(d)
		}
		if d, ok := val.(float64); ok {
			return fmt.Sprint(d)
		}
		return "snan"
	},
	"datefmt": func(val time.Time) string {
		return val.Format("15:04 02 Jan 2006")
	},
}
