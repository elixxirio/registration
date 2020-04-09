package node

import (
	"encoding/json"
	"github.com/pkg/errors"
	"gitlab.com/elixxir/primitives/utils"
)

type Info struct {
	RegCode string
	Order   string
}

// LoadInfo opens a JSON file and marshals it into a slice of Info. An error is
// returned when an issue is encountered reading the JSON file or unmarshaling
// the data.
func LoadInfo(filePath string) ([]Info, error) {
	// Data loaded from file will be stored here
	var infos []Info

	// Open file and get the JSON data
	jsonData, err := utils.ReadFile(filePath)
	if err != nil {
		return nil, errors.Errorf("Could not load JSON file %s: %v",
			filePath, err)
	}

	// Unmarshal the JSON data
	err = json.Unmarshal(jsonData, infos)
	if err != nil {
		return nil, errors.Errorf("Could not unmarshal JSON from file %s: %v",
			filePath, err)
	}

	return infos, nil
}
