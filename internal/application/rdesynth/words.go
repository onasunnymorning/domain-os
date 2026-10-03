package rdesynth

// Fixed vocabularies the generator draws from. Every entry is plain ASCII
// with no XML-special characters, which is what lets the generator write them
// without escaping.

var adjectives = []string{
	"amber", "bold", "brisk", "calm", "clever", "cosmic", "crisp", "daring",
	"eager", "early", "fancy", "fresh", "gentle", "golden", "grand", "happy",
	"humble", "jolly", "keen", "kind", "lively", "lucky", "mellow", "mighty",
	"modern", "noble", "quick", "quiet", "rapid", "royal", "rustic", "silver",
	"simple", "smart", "solar", "sunny", "swift", "tidy", "urban", "vivid",
	"warm", "wild", "wise", "young", "zesty",
}

var nouns = []string{
	"anchor", "apple", "arrow", "bakery", "bridge", "canyon", "castle", "cloud",
	"compass", "crafts", "design", "falcon", "forest", "garden", "harbor",
	"island", "journal", "lantern", "maple", "market", "meadow", "museum",
	"orchard", "otter", "pixel", "planet", "press", "river", "rocket", "sail",
	"studio", "summit", "tailor", "thread", "tiger", "trail", "valley",
	"voyage", "willow", "works",
}

var firstNames = []string{
	"Alex", "Ana", "Ben", "Carla", "Chen", "Dana", "Diego", "Elif", "Emma",
	"Farah", "Hugo", "Ines", "Ivan", "Jamal", "Julia", "Kenji", "Lena", "Liam",
	"Maya", "Mateo", "Nadia", "Noah", "Olga", "Omar", "Priya", "Rosa", "Sam",
	"Sofia", "Tariq", "Yuki",
}

var lastNames = []string{
	"Alvarez", "Bauer", "Costa", "Dubois", "Evans", "Fischer", "Garcia",
	"Haddad", "Ivanova", "Jensen", "Kim", "Larsen", "Moreau", "Nakamura",
	"Okafor", "Patel", "Quinn", "Rossi", "Silva", "Tanaka", "Ueda", "Varga",
	"Weber", "Yilmaz", "Zhang",
}

var orgSuffixes = []string{"Ltd", "LLC", "GmbH", "SA", "BV", "Inc", "Studio", "Group"}

var streets = []string{
	"Main Street", "High Street", "Oak Avenue", "Station Road", "Market Square",
	"River Lane", "Park Road", "Church Street", "Mill Lane", "Harbour Way",
}

// place is a city with its country code, a postcode pattern and the dialling
// prefix its synthetic telephone numbers use.
type place struct {
	city, cc, pc, dial string
}

var places = []place{
	{"Springfield", "US", "62701", "1"},
	{"Toronto", "CA", "M5H 2N2", "1"},
	{"Bogota", "CO", "110111", "57"},
	{"Madrid", "ES", "28013", "34"},
	{"Lyon", "FR", "69002", "33"},
	{"Hamburg", "DE", "20095", "49"},
	{"Utrecht", "NL", "3511 AA", "31"},
	{"Leeds", "GB", "LS1 1UR", "44"},
	{"Osaka", "JP", "530-0001", "81"},
	{"Melbourne", "AU", "3000", "61"},
}
