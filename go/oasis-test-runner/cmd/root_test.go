package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeParamSets(t *testing.T) {
	var zippedParams map[string][]string
	var expectedParamSets []map[string]string

	// Empty set.
	zippedParams = map[string][]string{}
	expectedParamSets = []map[string]string{}
	require.Equal(t, expectedParamSets, computeParamSets(zippedParams, map[string]string{}))

	// Single element.
	zippedParams = map[string][]string{
		"testParam1": []string{"1"},
		"testParam2": []string{"a"},
	}
	expectedParamSets = []map[string]string{
		{"testParam1": "1", "testParam2": "a"},
	}
	require.Equal(t, expectedParamSets, computeParamSets(zippedParams, map[string]string{}))

	// Combinations of two elements.
	zippedParams = map[string][]string{
		"testParam1": []string{"1", "2", "3"},
		"testParam2": []string{"a", "b"},
	}
	expectedParamSets = []map[string]string{
		{"testParam1": "1", "testParam2": "a"},
		{"testParam1": "1", "testParam2": "b"},
		{"testParam1": "2", "testParam2": "a"},
		{"testParam1": "2", "testParam2": "b"},
		{"testParam1": "3", "testParam2": "a"},
		{"testParam1": "3", "testParam2": "b"},
	}
	require.Equal(t, expectedParamSets, computeParamSets(zippedParams, map[string]string{}))

	// Duplicated elements - should not affect the outcome.
	zippedParams = map[string][]string{
		"testParam1": []string{"1", "1", "1"},
		"testParam2": []string{"a", "b"},
	}
	expectedParamSets = []map[string]string{
		{"testParam1": "1", "testParam2": "a"},
		{"testParam1": "1", "testParam2": "b"},
		{"testParam1": "1", "testParam2": "a"},
		{"testParam1": "1", "testParam2": "b"},
		{"testParam1": "1", "testParam2": "a"},
		{"testParam1": "1", "testParam2": "b"},
	}
	require.Equal(t, expectedParamSets, computeParamSets(zippedParams, map[string]string{}))

}
