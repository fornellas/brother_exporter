// https://prometheus.io/docs/concepts/data_model/

package prometheus

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
)

type Labels map[string]string

func (labels Labels) String() string {
	keys := []string{}
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	str := ""
	for i, key := range keys {
		if i > 0 {
			str += ","
		}
		value := labels[key]
		str += key + `="` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return str
}

type TimeSeries struct {
	metricName   string
	labels       Labels
	valueIEEE754 uint64
}

var validMetricNameRegexp = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]*$")
var validLabelNameRegexp = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]*$")

func NewTimeSeries(metricName string, labels Labels) (*TimeSeries, error) {
	if !validMetricNameRegexp.MatchString(metricName) {
		return nil, fmt.Errorf("name: bad value %v: it must match %s", metricName, validMetricNameRegexp)
	}
	for k, v := range labels {
		if !validLabelNameRegexp.MatchString(k) {
			return nil, fmt.Errorf("label: bad name %v: it must match %s", k, validLabelNameRegexp)
		}
		if strings.HasPrefix(k, "__") {
			return nil, fmt.Errorf("label: bad name %v: it can not start with __", k)
		}
		if v == "" {
			return nil, fmt.Errorf("label: %s: value can't be empty", k)
		}
	}

	return &TimeSeries{
		metricName: metricName,
		labels:     labels,
	}, nil
}

func (ts *TimeSeries) Set(value float64) {
	atomic.StoreUint64(&ts.valueIEEE754, math.Float64bits(value))
}

func (ts *TimeSeries) Value() float64 {
	return math.Float64frombits(atomic.LoadUint64(&ts.valueIEEE754))
}

func (ts *TimeSeries) String() string {
	return ts.metricName + `{` + ts.labels.String() + `} ` + strconv.FormatFloat(ts.Value(), 'E', -1, 64)
}

type TimeSeriesGroup struct {
	timeSeriesSlice []*TimeSeries
}

func NewTimeSeriesGroup() *TimeSeriesGroup {
	return &TimeSeriesGroup{
		timeSeriesSlice: []*TimeSeries{},
	}
}

func (tsg *TimeSeriesGroup) Add(ts *TimeSeries) {
	tsg.timeSeriesSlice = append(tsg.timeSeriesSlice, ts)
}

func (tsg TimeSeriesGroup) String() string {
	str := ""
	for _, ts := range tsg.timeSeriesSlice {
		str += ts.String() + "\n"
	}
	return str
}
