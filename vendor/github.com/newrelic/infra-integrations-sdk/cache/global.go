package cache

var instance, err = NewCache()

// Save marshalls and stores the data the cache is holding into disk as a JSON file.
func Save() error {
	return instance.Save()
}

// Get looks for a key in the cache and returns its value together with the timestamp
// of when it was last set. The third returned value indicates whether
// the key has been found or not.
func Get(name string) (float64, int64, bool) {
	return instance.Get(name)
}

// Set adds a value into the cache together with the current timestamp.
func Set(name string, value float64) int64 {
	return instance.Set(name, value)
}

// Status will return an error if any was found during global Cache creation.
func Status() error {
	return err
}
