package entities

type RDEHost struct {
	Name   string          `xml:"name"`
	RoID   string          `xml:"roid"`
	Status []RDEHostStatus `xml:"status"`
	Addr   []RDEHostAddr   `xml:"addr"`
	ClID   string          `xml:"clID"`
	CrRr   string          `xml:"crRr"`
	CrDate string          `xml:"crDate"`
	UpRr   string          `xml:"upRr"`
	UpDate string          `xml:"upDate"`
}

type RDEHostStatus struct {
	S string `xml:"s,attr"`
}

type RDEHostAddr struct {
	IP string `xml:"ip,attr"`
	ID string `xml:",chardata"`
}
