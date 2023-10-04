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

type ColumnNumber int

type ColumnName string

type Entry struct {
	ColumnNumber ColumnNumber
	ColumnName   ColumnName
	Value        string
}

type Entries []Entry

func (entries Entries) Get(columnName ColumnName) (*Entry, bool, error) {
	found := false
	var retEntry Entry
	for _, entry := range entries {
		if entry.ColumnName == columnName {
			if found {
				return nil, false, fmt.Errorf("column %q not unique", columnName)
			}
			found = true
			retEntry = entry
		}
	}
	if !found {
		return nil, false, nil
	}
	return &retEntry, true, nil
}

type ValueMapFn func(string) (float64, error)

type GroupToLabels struct {
	MetricNameSuffix             string
	ColumnNameToLabelValueRegexp *regexp.Regexp
	LabelName                    string
	ValueMapFn                   ValueMapFn
	MinColumn                    ColumnNumber
	MaxColumn                    ColumnNumber
}

type Plain struct {
	ColumnName       ColumnName
	MetricNameSuffix string
	ValueMapFn       ValueMapFn
	Labels           prometheus.Labels
	MinColumn        ColumnNumber
	MaxColumn        ColumnNumber
}

type Config struct {
	Info          []ColumnName
	GroupToLabels []GroupToLabels
	Plains        []Plain
	Ignore        []ColumnName
}

func (c *Config) getInfoTimeSeries(entries Entries) ([]*prometheus.TimeSeries, []ColumnNumber, error) {
	infoLabels := prometheus.Labels{}
	processedColumnNumbers := []ColumnNumber{}

	for _, infoLabel := range c.Info {
		entry, ok, err := entries.Get(infoLabel)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("missing '%s'", infoLabel)
		}

		processedColumnNumbers = append(processedColumnNumbers, entry.ColumnNumber)

		if entry.Value == "" {
			continue
		}

		infoLabels[strcase.ToSnake(string(infoLabel))] = entry.Value
	}

	infoTimeSeries, err := prometheus.NewTimeSeries("brother_printer_info", infoLabels)
	if err != nil {
		return nil, nil, fmt.Errorf("bad info time series: %s", err)
	}
	infoTimeSeries.Set(1.0)
	return []*prometheus.TimeSeries{infoTimeSeries}, processedColumnNumbers, nil
}

func (c *Config) getGroupedTimeSeries(entries Entries) ([]*prometheus.TimeSeries, []ColumnNumber, error) {
	timeSeriesSlice := []*prometheus.TimeSeries{}
	processedColumnNumbers := []ColumnNumber{}

	for _, groupToLabels := range c.GroupToLabels {
		for _, entry := range entries {
			var err error

			if groupToLabels.MinColumn != 0 && entry.ColumnNumber < groupToLabels.MinColumn {
				continue
			}
			if groupToLabels.MaxColumn != 0 && entry.ColumnNumber > groupToLabels.MaxColumn {
				continue
			}

			matches := groupToLabels.ColumnNameToLabelValueRegexp.FindAllStringSubmatch(string(entry.ColumnName), -1)
			if len(matches) != 1 {
				continue
			}
			if len(matches[0]) != 2 {
				continue
			}
			processedColumnNumbers = append(processedColumnNumbers, entry.ColumnNumber)

			labelValue := matches[0][1]

			var floatValue float64
			if groupToLabels.ValueMapFn != nil {
				floatValue, err = groupToLabels.ValueMapFn(entry.Value)
				if err != nil {
					return nil, nil, err
				}
			} else {
				floatValue, err = strconv.ParseFloat(entry.Value, 64)
				if err != nil {
					return nil, nil, err
				}
			}

			timeSeries, err := prometheus.NewTimeSeries(
				fmt.Sprintf("brother_printer_%s", groupToLabels.MetricNameSuffix),
				prometheus.Labels{
					groupToLabels.LabelName: labelValue,
				},
			)
			if err != nil {
				return nil, nil, err
			}
			timeSeries.Set(floatValue)
			timeSeriesSlice = append(timeSeriesSlice, timeSeries)
		}
	}

	return timeSeriesSlice, processedColumnNumbers, nil
}

func (c *Config) getPlainTimeSeries(entries Entries) ([]*prometheus.TimeSeries, []ColumnNumber, error) {
	timeSeriesSlice := []*prometheus.TimeSeries{}
	processedColumnNumbers := []ColumnNumber{}

	for _, plain := range c.Plains {
		found := false
		for _, entry := range entries {
			if entry.ColumnName != plain.ColumnName {
				continue
			}

			if plain.MinColumn != 0 && entry.ColumnNumber < plain.MinColumn {
				continue
			}
			if plain.MaxColumn != 0 && entry.ColumnNumber > plain.MaxColumn {
				continue
			}

			found = true
			processedColumnNumbers = append(processedColumnNumbers, entry.ColumnNumber)

			var floatValue float64
			var err error
			if plain.ValueMapFn != nil {
				floatValue, err = plain.ValueMapFn(entry.Value)
				if err != nil {
					return nil, nil, err
				}
			} else {
				floatValue, err = strconv.ParseFloat(entry.Value, 64)
				if err != nil {
					return nil, nil, err
				}
			}

			timeSeries, err := prometheus.NewTimeSeries(
				fmt.Sprintf("brother_printer_%s", plain.MetricNameSuffix),
				plain.Labels,
			)
			if err != nil {
				return nil, nil, err
			}
			timeSeries.Set(floatValue)
			timeSeriesSlice = append(timeSeriesSlice, timeSeries)
			break
		}

		if !found {
			return nil, nil, fmt.Errorf("Column %q not found", plain.ColumnName)
		}
	}

	return timeSeriesSlice, processedColumnNumbers, nil
}

func (c *Config) GetTimeSeriesGroup(entries Entries) (*prometheus.TimeSeriesGroup, error) {
	timeSeriesGroup := prometheus.NewTimeSeriesGroup()
	processedColumnNumbers := []ColumnNumber{}

	for _, fn := range []func(Entries) ([]*prometheus.TimeSeries, []ColumnNumber, error){
		c.getInfoTimeSeries,
		c.getGroupedTimeSeries,
		c.getPlainTimeSeries,
	} {
		infoTimeSeries, columnNumbers, err := fn(entries)
		if err != nil {
			return nil, err
		}
		timeSeriesGroup.Add(infoTimeSeries...)
		for _, columnNumber := range columnNumbers {
			for _, processedColumnNumber := range processedColumnNumbers {
				if columnNumber == processedColumnNumber {
					return nil, fmt.Errorf("Column %d used twice", columnNumber)
				}
			}
		}
		processedColumnNumbers = append(processedColumnNumbers, columnNumbers...)
	}

	for _, entry := range entries {
		found := false
		for _, processedColumnNumber := range processedColumnNumbers {
			if processedColumnNumber == entry.ColumnNumber {
				found = true
				break
			}
		}
		for _, ignoreColumnName := range c.Ignore {
			if ignoreColumnName == entry.ColumnName {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("Unprocessed column %q=%q", entry.ColumnName, entry.Value)
		}
	}

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
				MinColumn: 12,
				MaxColumn: 17,
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
				MinColumn: 23,
				MaxColumn: 28,
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
		Ignore: []ColumnName{
			"Page Counter",
			"Total",
			"Total 2-sided Print",
			"Total Paper Jams",
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

	var entries Entries
	for i, name := range names {
		if name == "" {
			continue
		}
		entries = append(entries, Entry{
			ColumnNumber: ColumnNumber(i),
			ColumnName:   ColumnName(name),
			Value:        strValues[i],
		})
	}

	modelEntry, ok, err := entries.Get("Model Name")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("missing Model name")
	}

	config, ok := ConfigMap[modelEntry.Value]
	if !ok {
		return nil, fmt.Errorf("unknown model name: %s", modelEntry.Value)
	}

	timeSeriesGroup, err := config.GetTimeSeriesGroup(entries)
	if err != nil {
		return nil, err
	}

	return timeSeriesGroup, nil
}
