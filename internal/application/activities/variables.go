package activities

import (
	"fmt"
	"os"
)

var (
	BASEURL      = fmt.Sprintf("http://%s:%s", os.Getenv("API_HOST"), os.Getenv("API_PORT"))
	BEARER_TOKEN = fmt.Sprintf("Bearer %s", os.Getenv("ADMIN_TOKEN"))
)
