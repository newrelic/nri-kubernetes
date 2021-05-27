package network

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v2/src/storage"
)

const storageKey = "defaultNetworkInterface"

type defaultInterfaceFunc func(string) (string, error)

// DefaultInterface returns the default interface named used by the OS.
func DefaultInterface(routeFile string) (string, error) {
	return getDefaultInterface(routeFile)
}

func doCachedDefaultInterface(
	logger *logrus.Logger,
	f defaultInterfaceFunc,
	routeFile string,
	storage storage.Storage,
	ttl time.Duration,
) (string, error) {

	var cached string
	ts, err := storage.Read(storageKey, &cached)
	if err == nil {
		logger.Debugf(
			"Found cached copy of %q with value '%s' stored at %s",
			storageKey,
			cached,
			time.Unix(ts, 0),
		)
		if time.Now().Unix() < ts+int64(ttl.Seconds()) {
			return cached, nil
		}
		logger.Debugf("Cached copy of %q expired. Refreshing", storageKey)
	} else {
		logger.Debugf("Cached %q not found. Triggering discovery process", storageKey)
	}

	defaultInterface, err := f(routeFile)
	if err != nil {
		return "", err
	}

	logger.Debugf(
		"Caching default network interface '%s' using key %q", defaultInterface, storageKey)
	err = storage.Write(storageKey, defaultInterface)
	if err != nil {
		logger.WithError(err).Warnf("while storing %q in the cache", storageKey)
	}

	return defaultInterface, nil
}

// CachedDefaultInterface returns the default interface name used by
// the system. The result is cached and expired based on the given ttl.
func CachedDefaultInterface(
	logger *logrus.Logger,
	routeFile string,
	storage storage.Storage,
	ttl time.Duration,
) (string, error) {
	return doCachedDefaultInterface(logger, DefaultInterface, routeFile, storage, ttl)
}
