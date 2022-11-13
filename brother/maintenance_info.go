package brother

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/iancoleman/strcase"

	"github.com/fornellas/brother_exporter/prometheus"
)

type ColumnName string

type TimeSeriesLabels struct {
	MetricNameSuffix string
	ColumnNames      []ColumnName
	LabelName        string
	MapFn            func(ColumnName) (string, error)
}

type Config struct {
	Info             []ColumnName
	TimeSeriesLabels map[string]map[ColumnName]string
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

func (c *Config) GetTimeSeriesGroup(values map[ColumnName]string) (*prometheus.TimeSeriesGroup, error) {
	timeSeriesGroup := prometheus.NewTimeSeriesGroup()

	infoTimeSeries, err := c.getInfoTimeSeries(values)
	if err != nil {
		return nil, err
	}
	timeSeriesGroup.Add(infoTimeSeries)

	// TODO check unrefe

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
