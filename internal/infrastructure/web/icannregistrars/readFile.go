package icannregistrars

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

// GetICANNCSVRegistrarsFromFile reads the CSV file and returns a slice of CSVRegistrars
func GetICANNCSVRegistrarsFromFile(filename string) ([]CSVRegistrar, error) {

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true // To avoid `parse error on line 1, column 4: bare " in non-quoted-field` error
	data, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV file: %v", err)
	}

	// Make a slice of CSVRegistrars
	registrars := make([]CSVRegistrar, len(data)-1)
	for i, line := range data {
		if i == 0 {
			// Skip the header
			continue
		}

		// convert the IANAID to an int
		ianaID, err := strconv.Atoi(line[1])
		if err != nil {
			return nil, fmt.Errorf("error converting IANAID to int: %v", err)
		}

		registrars[i-1] = CSVRegistrar{
			Name:          line[0],
			IANAID:        ianaID,
			Country:       line[2],
			PublicContact: line[3],
			Link:          line[4],
		}
	}

	return registrars, nil
}
