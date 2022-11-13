package brother

import (
	"fmt"
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

func TestReadMaintenanceInfo(t *testing.T) {
	csvFile, err := os.Open(getFixturePath(t, "HL-L2350DW/mnt_info.csv"))
	require.NoError(t, err)

	timeSeriesGroup, err := ReadMaintenanceInfo(csvFile)
	require.NoError(t, err)
	fmt.Printf("%s", timeSeriesGroup)
}
