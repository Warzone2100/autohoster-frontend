package main

import "html/template"

var (
	chartSCcolorLost    = "#c33c"
	chartSCcolorWon     = "#3c3c"
	chartSCcolorNeutral = "#17fc"
)

type primitiveStackedChart struct {
	Caption   string
	AxisY     string
	AxisX     string
	Data      []primitiveStackedChartColumn
	TotalData int
}

type primitiveStackedChartColumn struct {
	Label  template.HTML
	Values []primitiveStackedChartColumnValue
}

type primitiveStackedChartColumnValue struct {
	Label string
	Color string
	Value int
}

func newSCColVal(l, c string, v int) primitiveStackedChartColumnValue {
	return primitiveStackedChartColumnValue{
		Label: l,
		Color: c,
		Value: v,
	}
}

func newSC(c, x, y string) *primitiveStackedChart {
	return &primitiveStackedChart{
		Caption:   c,
		AxisY:     y,
		AxisX:     x,
		Data:      []primitiveStackedChartColumn{},
		TotalData: 0,
	}
}

func (ch *primitiveStackedChart) calcTotals() *primitiveStackedChart {
	ch.TotalData = 0
	for _, v := range ch.Data {
		for _, vv := range v.Values {
			ch.TotalData += vv.Value
		}
	}
	return ch
}

func (ch *primitiveStackedChart) appendToColumn(colname, label, color string, value int) {
	for i, v := range ch.Data {
		if v.Label == template.HTML(colname) {
			ch.Data[i].Values = append(ch.Data[i].Values, newSCColVal(label, color, value))
			return
		}
	}
	ch.Data = append(ch.Data, primitiveStackedChartColumn{
		Label:  template.HTML(colname),
		Values: []primitiveStackedChartColumnValue{newSCColVal(label, color, value)},
	})
}
