package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
)

// This script logs into the ICANN MOSAPI and checks the status of a TLD
// It takes a csv as input with the follogin columns: tld,username,password

func main() {
	// FLAGS
	filename := flag.String("f", "", "(path to) the CSV file containign the TLDs and credentials")
	flag.Parse()

	if *filename == "" {
		log.Fatal("Please provide a filename")
	}

	// Read the file
	file, err := os.Open(*filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	data, err := reader.ReadAll()
	if err != nil {
		log.Fatalln(err)
	}

	// For each line in the file, connect tot the MOSAPI and check the status of the TLD
	for _, line := range data {
		tld := line[0]
		username := line[1]
		password := line[2]

		jar, err := cookiejar.New(nil)
		if err != nil {
			// error handling
		}

		// Login to the MOSAPI
		client := &http.Client{
			Jar: jar,
		}
		req, err := http.NewRequest("GET", "https://mosapi.icann.org/ry/"+tld+"/login", nil)
		if err != nil {
			log.Fatal(err)
		}
		req.SetBasicAuth(username, password)
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode != 200 {
			log.Println("login failed for ", tld)
		}
		client.Jar.SetCookies(resp.Request.URL, resp.Cookies())
		fmt.Println("login successful for ", tld)

		// Get the status of the TLD
		req, err = http.NewRequest("GET", "https://mosapi.icann.org/ry/"+tld+"/v2/monitoring/state", nil)
		if err != nil {
			log.Fatal(err)
		}
		resp, err = client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode != 200 {
			log.Println("failed to get status for ", tld)
		} else {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatalln(err)
			}

			// Print the response body in JSON
			fmt.Println(string(body))
		}
		// Logout
		req, err = http.NewRequest("GET", "https://mosapi.icann.org/ry/"+tld+"/logout", nil)
		if err != nil {
			log.Fatal(err)
		}
		resp, err = client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode != 200 {
			log.Println("logout failed for ", tld)
		} else {
			fmt.Println("logout successful for ", tld)
		}
	}

}
