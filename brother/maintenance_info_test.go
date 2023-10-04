package brother

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func getFixturePath(t *testing.T, path string) string {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	prefix := filepath.Dir(file)
	return filepath.Join(prefix, "fixtures", path)
}

var expectedMetrics = `brother_printer_average_coverage_ratio{} 4.82E-02
brother_printer_duplex_pages_printed_total{type="others"} 0E+00
brother_printer_duplex_pages_printed_total{type="print"} 3.2E+02
brother_printer_error_info{description="Toner Low",number="1"} 1E+00
brother_printer_errors_total{number="1"} 5.39E+02
brother_printer_errors_total{number="10"} 0E+00
brother_printer_errors_total{number="2"} 0E+00
brother_printer_errors_total{number="3"} 0E+00
brother_printer_errors_total{number="4"} 0E+00
brother_printer_errors_total{number="5"} 0E+00
brother_printer_errors_total{number="6"} 0E+00
brother_printer_errors_total{number="7"} 0E+00
brother_printer_errors_total{number="8"} 0E+00
brother_printer_errors_total{number="9"} 0E+00
brother_printer_info{ip_address="192.168.0.100",main_firmware_version="1.72",memory_size="64",model_name="Brother HL-L2350DW series",node_name="BRW4CD5775E3B3B",serial_no_="E78252F2N882157"} 1E+00
brother_printer_pages_printed_by_paper_size_total{paper_size="A4/Letter"} 5.41E+02
brother_printer_pages_printed_by_paper_size_total{paper_size="A5"} 0E+00
brother_printer_pages_printed_by_paper_size_total{paper_size="B5/Executive"} 0E+00
brother_printer_pages_printed_by_paper_size_total{paper_size="Envelopes"} 0E+00
brother_printer_pages_printed_by_paper_size_total{paper_size="Legal/Folio"} 0E+00
brother_printer_pages_printed_by_paper_size_total{paper_size="Others"} 0E+00
brother_printer_pages_printed_by_paper_type_total{paper_type="Envelopes/Env. Thick/Env. Thin"} 0E+00
brother_printer_pages_printed_by_paper_type_total{paper_type="Hagaki"} 0E+00
brother_printer_pages_printed_by_paper_type_total{paper_type="Label"} 0E+00
brother_printer_pages_printed_by_paper_type_total{paper_type="Plain/Thin/Recycled"} 5.41E+02
brother_printer_pages_printed_by_paper_type_total{paper_type="Thick/Thicker/Bond"} 0E+00
brother_printer_pages_printed_total{type="others"} 4E+00
brother_printer_pages_printed_total{type="print"} 5.37E+02
brother_printer_paper_jam_total{location="Jam 2-sided"} 0E+00
brother_printer_paper_jam_total{location="Jam Inside"} 0E+00
brother_printer_paper_jam_total{location="Jam Rear"} 0E+00
brother_printer_paper_jam_total{location="Jam Tray 1"} 0E+00
brother_printer_part_remaining_life_ratio{part="Drum Unit"} 9.6E-01
brother_printer_part_remaining_life_ratio{part="Toner"} 2E-01
brother_printer_part_replace_total{part="Drum Unit"} 0E+00
brother_printer_part_replace_total{part="Toner"} 0E+00`

func TestReadMaintenanceInfo(t *testing.T) {
	csvFile, err := os.Open(getFixturePath(t, "HL-L2350DW/mnt_info.csv"))
	require.NoError(t, err)

	timeSeriesGroup, err := ReadMaintenanceInfo(csvFile)
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, timeSeriesGroup.String())
}
