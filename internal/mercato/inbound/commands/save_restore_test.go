package commands

import (
	"bytes"
	"os"
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveRestore_FileFlag(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mct-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	oldWd, _ := os.Getwd()
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()

	// Stub services
	svc := mockServices()
	svc.Config = &stubConfig{
		getConfigFn: func() (domain.Config, error) {
			return domain.Config{}, nil
		},
	}
	svc.Entries = &stubEntries{
		listFn: func(opts service.ListOpts) ([]domain.Entry, error) {
			return []domain.Entry{}, nil
		},
	}
	svc.Markets = &stubMarkets{
		listFn: func() ([]domain.Market, error) {
			return []domain.Market{}, nil
		},
	}
	opts := &GlobalOpts{}

	t.Run("save --file custom.json", func(t *testing.T) {
		cmd := newSaveCmd(svc, opts)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--file", "custom.json"})

		err := cmd.Execute()
		require.NoError(t, err)

		_, err = os.Stat("custom.json")
		assert.NoError(t, err, "custom.json should be created")

		_, err = os.Stat(".mct.json")
		assert.Error(t, err, ".mct.json should not be created")
		assert.Contains(t, out.String(), "exported to custom.json")
	})

	t.Run("restore --file custom.json", func(t *testing.T) {
		// Prepare custom.json with a market
		content := `{"version":1,"markets":[{"name":"test","url":"https://test.com","branch":"main"}],"profiles":[]}`
		err := os.WriteFile("custom.json", []byte(content), 0644)
		require.NoError(t, err)

		// Mock restore/import behavior
		svc.Markets.(*stubMarkets).addMarketFn = func(url string, opts service.AddMarketOpts) (service.AddMarketResult, error) {
			return service.AddMarketResult{}, nil
		}

		cmd := newRestoreCmd(svc, opts)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		// Use -y to avoid prompt
		cmd.SetArgs([]string{"-f", "custom.json", "--yes"})

		err = cmd.Execute()
		require.NoError(t, err)
		assert.Contains(t, out.String(), "market test")
	})
}
