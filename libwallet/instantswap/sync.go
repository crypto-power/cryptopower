package instantswap

import (
	"context"
	"time"

	"code.cryptopower.dev/group/instantswap"
	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
)

const (
	retryInterval = 15 // 15 seconds

	configDBBkt                  = "instantswap_config"
	LastSyncedTimestampConfigKey = "instantswap_last_synced_timestamp"
)

func (instantSwap *InstantSwap) Sync(ctx context.Context) error {
	instantSwap.mu.RLock()

	if instantSwap.cancelSync != nil {
		instantSwap.mu.RUnlock()
		return errors.New(ErrSyncAlreadyInProgress)
	}

	instantSwap.ctx, instantSwap.cancelSync = context.WithCancel(ctx)

	defer func() {
		instantSwap.cancelSync = nil
	}()

	instantSwap.mu.RUnlock()

	log.Info("Exchange sync: started")
	exchangeServers := instantSwap.ExchangeServers()
	for _, exchangeServer := range exchangeServers {
		exchangeObject, err := instantSwap.NewExchanageServer(exchangeServer)
		if err != nil {
			log.Errorf("Error creating exchange server: %v", err)
			continue
		}

		err = instantSwap.syncServer(exchangeServer, exchangeObject)
		if err != nil {
			log.Errorf("Error syncing exchange server: %v", err)
			return err
		}
	}

	log.Info("Exchange sync: completed")
	instantSwap.saveLastSyncedTimestamp(time.Now().Unix())
	instantSwap.publishSynced()

	return nil
}

func (instantSwap *InstantSwap) syncServer(exchangeServer ExchangeServer, exchangeObject instantswap.IDExchange) error {
	if instantSwap.ctx.Err() != nil {
		return instantSwap.ctx.Err()
	}

	log.Info("Exchange sync: checking for updates for", exchangeServer.Server.CapFirstLetter())

	for {
		err := instantSwap.checkForUpdates(exchangeObject, exchangeServer)
		if err != nil {
			log.Errorf("Error checking for exchange updates: %v", err)
			time.Sleep(retryInterval * time.Second)
			continue
		}
		break
	}

	log.Info("Exchange sync: update complete for", exchangeServer.Server.CapFirstLetter())

	return nil
}

func (instantSwap *InstantSwap) IsSyncing() bool {
	instantSwap.mu.RLock()
	defer instantSwap.mu.RUnlock()
	return instantSwap.cancelSync != nil
}

func (instantSwap *InstantSwap) StopSync() {
	instantSwap.mu.RLock()
	if instantSwap.cancelSync != nil {
		instantSwap.cancelSync()
		instantSwap.cancelSync = nil
	}
	instantSwap.mu.RUnlock()
	log.Info("Exchange sync: stopped")
}

// check all saved orders which status are not completed and update their status
func (instantSwap *InstantSwap) checkForUpdates(exchangeObject instantswap.IDExchange, exchangeServer ExchangeServer) error {
	offset := 0
	instantSwap.mu.RLock()
	limit := 20
	instantSwap.mu.RUnlock()
	for {
		// Check if instantswap has been shutdown and exit if true.
		if instantSwap.ctx.Err() != nil {
			return instantSwap.ctx.Err()
		}

		orders, err := instantSwap.GetOrdersRaw(int32(offset), int32(limit), true)
		if err != nil {
			return err
		}

		if len(orders) == 0 {
			break
		}

		offset += len(orders)

		for _, order := range orders {
			// if the order was created before the ExchangeServer field was added
			// to the Order struct update it here. This prevents a crash when
			// attempting to open legacy orders
			switch order.ExchangeServer.Server {
			case ChangeNow:
				order.ExchangeServer.Config = ExchangeConfig{
					ApiKey: API_KEY_CHANGENOW,
				}
			case GoDex:
				order.ExchangeServer.Config = ExchangeConfig{
					ApiKey: API_KEY_GODEX,
				}
			default:
				order.ExchangeServer.Config = ExchangeConfig{}
			}

			err = instantSwap.updateOrder(order)
			if err != nil {
				log.Errorf("Error updating legacy order: %v", err)
			}

			if order.ExchangeServer == exchangeServer {
				// delay for 5 seconds to avoid rate limit
				time.Sleep(5 * time.Second)
				_, err = instantSwap.GetOrderInfo(exchangeObject, order.UUID)
				if err != nil {
					log.Errorf("Error getting order info: %v", err)
					return err
				}
			}
		}

	}

	return nil
}

func (instantSwap *InstantSwap) publishSynced() {
	instantSwap.notificationListenersMu.Lock()
	defer instantSwap.notificationListenersMu.Unlock()

	for _, notificationListener := range instantSwap.notificationListeners {
		notificationListener.OnExchangeOrdersSynced()
	}
}

func (instantSwap *InstantSwap) AddNotificationListener(notificationListener ExchangeNotificationListener, uniqueIdentifier string) error {
	instantSwap.notificationListenersMu.Lock()
	defer instantSwap.notificationListenersMu.Unlock()

	if _, ok := instantSwap.notificationListeners[uniqueIdentifier]; ok {
		return errors.New(ErrListenerAlreadyExist)
	}

	instantSwap.notificationListeners[uniqueIdentifier] = notificationListener
	return nil
}

func (instantSwap *InstantSwap) RemoveNotificationListener(uniqueIdentifier string) {
	instantSwap.notificationListenersMu.Lock()
	defer instantSwap.notificationListenersMu.Unlock()

	delete(instantSwap.notificationListeners, uniqueIdentifier)
}

func (instantSwap *InstantSwap) GetLastSyncedTimeStamp() int64 {
	return instantSwap.getLastSyncedTimestamp()
}

func (instantSwap *InstantSwap) saveLastSyncedTimestamp(lastSyncedTimestamp int64) {
	err := instantSwap.db.Set(configDBBkt, LastSyncedTimestampConfigKey, &lastSyncedTimestamp)
	if err != nil {
		log.Errorf("error setting config value for key: %s, error: %v", LastSyncedTimestampConfigKey, err)
	}
}

func (instantSwap *InstantSwap) getLastSyncedTimestamp() (lastSyncedTimestamp int64) {
	err := instantSwap.db.Get(configDBBkt, LastSyncedTimestampConfigKey, &lastSyncedTimestamp)
	if err != nil && err != storm.ErrNotFound {
		log.Errorf("error reading config value for key: %s, error: %v", LastSyncedTimestampConfigKey, err)
	}
	return lastSyncedTimestamp
}
