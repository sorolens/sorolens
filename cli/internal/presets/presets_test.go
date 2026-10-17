package presets

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltInPresets_Validate(t *testing.T) {
	for _, p := range BuiltInPresets {
		t.Run(string(p.Type), func(t *testing.T) {
			require.NoError(t, p.Validate(), "built-in preset %q should validate", p.Type)
		})
	}
}

func TestPresetsByName_ReturnsAllAndIsSorted(t *testing.T) {
	all := PresetsByName()

	require.Len(t, all, len(BuiltInPresets), "should return every built-in preset")

	for i := 0; i < len(all)-1; i++ {
		assert.LessOrEqual(t, all[i].Label, all[i+1].Label, "presets should be sorted by label")
	}

	types := make(map[PresetType]struct{}, len(all))
	for _, p := range all {
		types[p.Type] = struct{}{}
	}
	for presetType := range BuiltInPresets {
		_, ok := types[presetType]
		assert.True(t, ok, "preset type %q should be present in PresetsByName", presetType)
	}
}

func TestLookup_FindsBuiltIn(t *testing.T) {
	for _, p := range BuiltInPresets {
		found, err := Lookup(p.Type)
		require.NoError(t, err, "lookup %q should succeed", p.Type)
		assert.Equal(t, p, found)
	}
}

func TestLookup_RejectsUnknown(t *testing.T) {
	_, err := Lookup(PresetType("nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown preset type")
}

func TestContractPreset_Validate_RejectsEmptyType(t *testing.T) {
	p := ContractPreset{
		Type:        "",
		Label:       "test",
		EventFilters: []string{"a"},
		FunctionNames: []string{"b"},
	}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type must be non-empty")
}

func TestContractPreset_Validate_RejectsEmptyLabel(t *testing.T) {
	p := ContractPreset{
		Type:        PresetDEX,
		Label:       "",
		EventFilters: []string{"a"},
		FunctionNames: []string{"b"},
	}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "label must be non-empty")
}

func TestContractPreset_Validate_RejectsEmptyEventFilter(t *testing.T) {
	p := ContractPreset{
		Type:        PresetDEX,
		Label:       "test",
		EventFilters: []string{"swap", ""},
		FunctionNames: []string{"a"},
	}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event_filters entries must be non-empty")
}

func TestContractPreset_Validate_RejectsEmptyFunctionName(t *testing.T) {
	p := ContractPreset{
		Type:        PresetDEX,
		Label:       "test",
		EventFilters: []string{"a"},
		FunctionNames: []string{"swap", ""},
	}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function_names entries must be non-empty")
}

func TestContractPreset_Validate_RejectsDuplicateEventFilters(t *testing.T) {
	p := ContractPreset{
		Type:        PresetDEX,
		Label:       "test",
		EventFilters: []string{"swap", "swap"},
		FunctionNames: []string{"a"},
	}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event_filters contains duplicate")
}

func TestContractPreset_Validate_RejectsDuplicateFunctionNames(t *testing.T) {
	p := ContractPreset{
		Type:        PresetDEX,
		Label:       "test",
		EventFilters: []string{"a"},
		FunctionNames: []string{"swap", "swap"},
	}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function_names contains duplicate")
}

func TestContractPreset_MarshalJSON_IncludesTypeAsString(t *testing.T) {
	p := ContractPreset{
		Type:        PresetDEX,
		Label:       "DEX / AMM",
		Description: "desc",
	}
	data, err := p.MarshalJSON()
	require.NoError(t, err)

	var decoded struct {
		Type        string `json:"type"`
		Label       string `json:"label"`
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "dex", decoded.Type)
	assert.Equal(t, "DEX / AMM", decoded.Label)
	assert.Equal(t, "desc", decoded.Description)
}
