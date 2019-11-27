package terraform

import (
	"io/ioutil"
	"testing"
)

func TestTerraformParser(t *testing.T) {
	parser := &Parser{}

	var input interface{}
	sampleFileBytes, err := ioutil.ReadFile("testdata/sample.tf")
	if err != nil {
		t.Fatalf("error reading sample file: %v", err)
	}

	if err = parser.Unmarshal(sampleFileBytes, &input); err != nil {
		t.Fatalf("parser should not have thrown an error: %v", err)
	}

	if input == nil {
		t.Error("there should be information parsed but its nil")
	}

	inputMap := input.(map[string]interface{})
	if len(inputMap["resource"].([]map[string]interface{})) <= 0 {
		t.Error("there should be resources defined in the parsed file, but none found")
	}
}
