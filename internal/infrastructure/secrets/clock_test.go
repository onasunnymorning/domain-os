package secrets

import "time"

// rdevalidateNow pins verification time after key generation (OpenPGP treats
// an earlier time as "key expired").
func rdevalidateNow() time.Time { return time.Now().UTC().Add(time.Hour) }
