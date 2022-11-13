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

type GroupToLabels struct {
	MetricNameSuffix string
	ColumnNames      []ColumnName
	LabelName        string
	LabelValueMapFn  func(ColumnName) (string, error)
	ValueMapFn       func(string) (float64, error)
}

type Config struct {
	Info          []ColumnName
	GroupToLabels []GroupToLabels
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
		for _, columnName := range groupToLabels.ColumnNames {
			var err error

			labelValue := string(columnName)
			if groupToLabels.LabelValueMapFn != nil {
				var err error
				labelValue, err = groupToLabels.LabelValueMapFn(columnName)
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
			timeSeries.Set(value)
			timeSeriesSlice = append(timeSeriesSlice, timeSeries)
		}
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

	// TODO check not referred column names

	return timeSeriesGroup, nil
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
		},
		GroupToLabels: []GroupToLabels{
			GroupToLabels{
				MetricNameSuffix: "part_remaining_life_ratio",
				ColumnNames: []ColumnName{
					`% of Life Remaining(Drum Unit)`,
					`% of Life Remaining(Toner)`,
				},
				LabelName: "part",
				LabelValueMapFn: func(columnName ColumnName) (string, error) {
					partRegexp := regexp.MustCompile(`^% of Life Remaining\((.+)\)$`)
					matches := partRegexp.FindAllStringSubmatch(string(columnName), -1)
					if len(matches) != 1 {
						return "", fmt.Errorf("%q: does not match %q", columnName, partRegexp)
					}
					return matches[0][1], nil
				},
				ValueMapFn: func(valueStr string) (float64, error) {
					value, err := strconv.ParseFloat(valueStr, 64)
					if err != nil {
						return 0, err
					}
					return value / 100, nil
				},
			},
			GroupToLabels{
				MetricNameSuffix: "pages_printed_by_paper_size_total",
				ColumnNames: []ColumnName{
					"A4/Letter",
					"Legal/Folio",
					"B5/Executive",
					"Envelopes",
					"A5",
					"Others",
				},
				LabelName: "paper_size",
			},
			GroupToLabels{
				MetricNameSuffix: "part_replace_total",
				ColumnNames: []ColumnName{
					"Replace Count(Toner)",
					"Replace Count(Drum Unit)",
				},
				LabelName: "part",
				LabelValueMapFn: func(columnName ColumnName) (string, error) {
					partRegexp := regexp.MustCompile(`^Replace Count\((.+)\)$`)
					matches := partRegexp.FindAllStringSubmatch(string(columnName), -1)
					if len(matches) != 1 {
						return "", fmt.Errorf("%q: does not match %q", columnName, partRegexp)
					}
					return matches[0][1], nil
				},
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
