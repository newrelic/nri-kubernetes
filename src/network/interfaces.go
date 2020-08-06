package network

import (
	"time"

	"github.com/newrelic/nri-kubernetes/src/storage"
	"github.com/sirupsen/logrus"
)

const storageKey = "defaultNetworkInterface"

type defaultInterfaceFunc func(string) (string, error)

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
		logger.Debugf("Cached copy of %q expired. Refreshing", cached, storageKey)
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

func CachedDefaultInterface(
	logger *logrus.Logger,
	routeFile string,
	storage storage.Storage,
	ttl time.Duration,
) (string, error) {
	return doCachedDefaultInterface(logger, DefaultInterface, routeFile, storage, ttl)
}
