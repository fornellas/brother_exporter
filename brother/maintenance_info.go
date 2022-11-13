package brother

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/fornellas/brother_exporter/prometheus"
	"github.com/iancoleman/strcase"
)

func ReadMaintenanceInfo(ioReader io.Reader) (prometheus.TimeSeriesGroup, error) {
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

	timeSeriesGroup := prometheus.TimeSeriesGroup{}

	for i, name := range names {
		strValue := strValues[i]

		floatValue, err := strconv.ParseFloat(strValue, 64)
		if err != nil {
			fmt.Printf("not a float %s: %s\n", name, err)
			continue
		}

		timeSeries, err := prometheus.NewTimeSeries(
			"brother_printer_"+strcase.ToSnake(name),
			prometheus.Labels{},
		)
		if err != nil {
			fmt.Printf("bad time series %s: %s\n", name, err)
			continue
		}
		timeSeries.Set(floatValue)

		timeSeriesGroup = append(timeSeriesGroup, timeSeries)
	}

	return timeSeriesGroup, nil
}
