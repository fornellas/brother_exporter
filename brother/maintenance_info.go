package brother

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/iancoleman/strcase"

	"github.com/fornellas/brother_exporter/prometheus"
)

type ColumnName string

type ValueMapFn func(string) (float64, error)

type GroupToLabels struct {
	MetricNameSuffix             string
	ColumnNameToLabelValueRegexp *regexp.Regexp
	LabelName                    string
	ValueMapFn                   ValueMapFn
}

type Plain struct {
	ColumnName       ColumnName
	MetricNameSuffix string
	ValueMapFn       ValueMapFn
	Labels           prometheus.Labels
}

type Config struct {
	Info          []ColumnName
	GroupToLabels []GroupToLabels
	Plains        []Plain
}

func (c *Config) getInfoTimeSeries(values map[ColumnName]string) (*prometheus.TimeSeries, error) {
	infoLabels := prometheus.Labels{}
	for _, infoLabel := range c.Info {
		value, ok := values[(infoLabel)]
		if !ok {
			return nil, fmt.Errorf("missing '%s'", infoLabel)
		}
		if value == "" {
			continue
		}
		infoLabels[strcase.ToSnake(string(infoLabel))] = value
	}
	infoTimeSeries, err := prometheus.NewTimeSeries("brother_printer_info", infoLabels)
	if err != nil {
		return nil, fmt.Errorf("bad info time series: %s", err)
	}
	return infoTimeSeries, nil
}

func (c *Config) getGroupedTimeSeries(values map[ColumnName]string) ([]*prometheus.TimeSeries, error) {
	timeSeriesSlice := []*prometheus.TimeSeries{}

	for _, groupToLabels := range c.GroupToLabels {
		for columnName := range values {
			var err error

			matches := groupToLabels.ColumnNameToLabelValueRegexp.FindAllStringSubmatch(string(columnName), -1)
			if len(matches) != 1 {
				continue
			}
			if len(matches[0]) != 2 {
				continue
			}
			labelValue := matches[0][1]

			rawValue, ok := values[columnName]
			if !ok {
				return nil, fmt.Errorf("%s: column does not exist", columnName)
			}

			var value float64
			if groupToLabels.ValueMapFn != nil {
				value, err = groupToLabels.ValueMapFn(rawValue)
				if err != nil {
					return nil, err
				}
			} else {
				value, err = strconv.ParseFloat(rawValue, 64)
				if err != nil {
					return nil, err
				}
			}

			timeSeries, err := prometheus.NewTimeSeries(
				fmt.Sprintf("brother_printer_%s", groupToLabels.MetricNameSuffix),
				prometheus.Labels{
					groupToLabels.LabelName: labelValue,
				},
			)
			if err != nil {
				return nil, err
			}
			timeSeries.Set(value)
			timeSeriesSlice = append(timeSeriesSlice, timeSeries)
		}
	}

	return timeSeriesSlice, nil
}

func (c *Config) getPlainTimeSeries(values map[ColumnName]string) ([]*prometheus.TimeSeries, error) {
	timeSeriesSlice := []*prometheus.TimeSeries{}

	for _, plain := range c.Plains {
		rawValue, ok := values[plain.ColumnName]
		if !ok {
			return nil, fmt.Errorf("Column %q not found", plain.ColumnName)
		}

		var value float64
		var err error
		if plain.ValueMapFn != nil {
			value, err = plain.ValueMapFn(rawValue)
			if err != nil {
				return nil, err
			}
		} else {
			value, err = strconv.ParseFloat(rawValue, 64)
			if err != nil {
				return nil, err
			}
		}

		timeSeries, err := prometheus.NewTimeSeries(
			fmt.Sprintf("brother_printer_%s", plain.MetricNameSuffix),
			plain.Labels,
		)
		if err != nil {
			return nil, err
		}
		timeSeries.Set(value)
		timeSeriesSlice = append(timeSeriesSlice, timeSeries)
	}

	return timeSeriesSlice, nil
}

func (c *Config) GetTimeSeriesGroup(values map[ColumnName]string) (*prometheus.TimeSeriesGroup, error) {
	timeSeriesGroup := prometheus.NewTimeSeriesGroup()

	infoTimeSeries, err := c.getInfoTimeSeries(values)
	if err != nil {
		return nil, err
	}
	timeSeriesGroup.Add(infoTimeSeries)

	groupedTimeSeries, err := c.getGroupedTimeSeries(values)
	if err != nil {
		return nil, err
	}
	timeSeriesGroup.Add(groupedTimeSeries...)

	plainTimeSeries, err := c.getPlainTimeSeries(values)
	if err != nil {
		return nil, err
	}
	timeSeriesGroup.Add(plainTimeSeries...)

	// TODO check not referred column names

	return timeSeriesGroup, nil
}

func divideBy100(valueStr string) (float64, error) {
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}
	return value / 100, nil
}

var ConfigMap = map[string]Config{
	"Brother HL-L2350DW series": Config{
		Info: []ColumnName{
			"Node Name",
			"Model Name",
			"Location",
			"Contact",
			"IP Address",
			"Serial No.",
			"Main Firmware Version",
			"Memory Size",
		},
		GroupToLabels: []GroupToLabels{
			GroupToLabels{
				MetricNameSuffix:             "part_remaining_life_ratio",
				ColumnNameToLabelValueRegexp: regexp.MustCompile(`^% of Life Remaining\((.+)\)$`),
				LabelName:                    "part",
				ValueMapFn:                   divideBy100,
			},
			GroupToLabels{
				MetricNameSuffix: "pages_printed_by_paper_size_total",
				ColumnNameToLabelValueRegexp: regexp.MustCompile(
					"^(A4/Letter|Legal/Folio|B5/Executive|Envelopes|A5|Others)$",
				),
				LabelName: "paper_size",
			},
			GroupToLabels{
				MetricNameSuffix: "pages_printed_by_paper_type_total",
				ColumnNameToLabelValueRegexp: regexp.MustCompile(
					"^(Plain/Thin/Recycled|Thick/Thicker/Bond|Envelopes/Env. Thick/Env. Thin|Label|Hagaki)$",
				),
				LabelName: "paper_type",
			},
			GroupToLabels{
				MetricNameSuffix:             "part_replace_total",
				ColumnNameToLabelValueRegexp: regexp.MustCompile(`^Replace Count\((.+)\)$`),
				LabelName:                    "part",
			},
			GroupToLabels{
				MetricNameSuffix: "paper_jam_total",
				ColumnNameToLabelValueRegexp: regexp.MustCompile(
					"^(Jam Tray 1|Jam Inside|Jam Rear|Jam 2-sided)$",
				),
				LabelName: "location",
			},
			GroupToLabels{
				MetricNameSuffix:             "errors_total",
				ColumnNameToLabelValueRegexp: regexp.MustCompile(`^Error Count ([0-9]+)$`),
				LabelName:                    "number",
			},
		},
		Plains: []Plain{
			Plain{
				ColumnName:       ColumnName("Print"),
				MetricNameSuffix: "pages_printed_total",
				Labels: prometheus.Labels{
					"type": "print",
				},
			},
			Plain{
				ColumnName:       ColumnName("Print 2-sided Print"),
				MetricNameSuffix: "duplex_pages_printed_total",
				Labels: prometheus.Labels{
					"type": "print",
				},
			},
			Plain{
				ColumnName:       ColumnName("Others"),
				MetricNameSuffix: "pages_printed_total",
				Labels: prometheus.Labels{
					"type": "others",
				},
			},
			Plain{
				ColumnName:       ColumnName("Others 2-sided Print"),
				MetricNameSuffix: "duplex_pages_printed_total",
				Labels: prometheus.Labels{
					"type": "others",
				},
			},
			Plain{
				ColumnName:       ColumnName("Average Coverage"),
				MetricNameSuffix: "average_coverage_ratio",
				ValueMapFn:       divideBy100,
			},
		},
	},
}

func ReadMaintenanceInfo(ioReader io.Reader) (*prometheus.TimeSeriesGroup, error) {
	csvReader := csv.NewReader(ioReader)

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) != 2 {
		return nil, fmt.Errorf("Expected 2 rows, got %d", len(records))
	}

	names := records[0]
	strValues := records[1]

	if len(names) != len(strValues) {
		return nil, fmt.Errorf("names row has %d columns and values row has %d", len(names), len(strValues))
	}

	values := map[ColumnName]string{}
	for i, name := range names {
		values[ColumnName(name)] = strValues[i]
	}

	modelName, ok := values["Model Name"]
	if !ok {
		return nil, fmt.Errorf("'Model Name' not reported by printer")
	}

	config, ok := ConfigMap[modelName]
	if !ok {
		return nil, fmt.Errorf("unknown model name: %s", modelName)
	}

	timeSeriesGroup, err := config.GetTimeSeriesGroup(values)
	if err != nil {
		return nil, err
	}

	return timeSeriesGroup, nil
}
