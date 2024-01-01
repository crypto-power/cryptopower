package instantswap

import (
	"context"
	"time"

	"decred.org/dcrwallet/v3/errors"
	"github.com/asdine/storm"
	"github.com/crypto-power/instantswap/instantswap"
)

const (
	retryInterval  = 15 // 15 seconds
	maxSyncRetries = 3

	configDBBkt                  = "instantswap_config"
	LastSyncedTimestampConfigKey = "instantswap_last_synced_timestamp"
)

// Sync synchronizes the exchange orders, by looping through each exchange
// server and querying the order info and updating the order saved in the
// databse with the order returned from the order info query. MUST be called
// from a goroutine.
func (instantSwap *InstantSwap) Sync() {
	var syncCtx context.Context

	instantSwap.syncMu.Lock()
	if instantSwap.cancelSync == nil {
		syncCtx, instantSwap.cancelSync = context.WithCancel(instantSwap.ctx)
	}
	instantSwap.syncMu.Unlock()

	if syncCtx == nil {
		return // already syncing
	}

	defer func() {
		instantSwap.syncMu.Lock()
		instantSwap.cancelSync()
		instantSwap.cancelSync = nil
		instantSwap.syncMu.Unlock()
	}()

	log.Info("Exchange sync: started")
	exchangeServers := instantSwap.ExchangeServers()
	for _, exchangeServer := range exchangeServers {
		// Stop syncing if the syncCtx has been canceled.
		if syncCtx.Err() != nil {
			return
		}

		// Initialize the exchange server.
		exchangeObject, err := instantSwap.NewExchangeServer(exchangeServer)
		if err != nil {
			log.Errorf("Error instantiating exchange server: %v", err)
			continue // skip server if there was an error instantiating the server
		}

		err = instantSwap.syncServer(exchangeServer, exchangeObject)
		if err != nil {
			log.Errorf("Error syncing exchange (%s) server: %v", exchangeServer.Server, err)
			return
		}
	}

	log.Info("Exchange sync: completed")
	instantSwap.saveLastSyncedTimestamp(time.Now().Unix())
	instantSwap.publishSynced()
}

func (instantSwap *InstantSwap) syncServer(exchangeServer ExchangeServer, exchangeObject instantswap.IDExchange) error {
	if instantSwap.ctx.Err() != nil {
		return instantSwap.ctx.Err()
	}

	log.Info("Exchange sync: checking for updates on", exchangeServer.Server.CapFirstLetter())

	attempts := 0
	for {
		// Check if instantswap has been shutdown and exit if true.
		if instantSwap.ctx.Err() != nil {
			return instantSwap.ctx.Err()
		}

		attempts++
		if attempts > maxSyncRetries {
			return errors.Errorf("failed to sync exchange server [%v] after 3 attempts", exchangeServer)
		}
		err := instantSwap.checkForUpdates(exchangeObject, exchangeServer)
		if err != nil {
			log.Errorf("Error checking for exchange updates: %v", err)
			time.Sleep(retryInterval * time.Second) // delay for 15 seconds before retrying
			continue
		}
		break
	}

	log.Info("Exchange sync: update complete for", exchangeServer.Server.CapFirstLetter())

	return nil
}

func (instantSwap *InstantSwap) IsSyncing() bool {
	instantSwap.syncMu.RLock()
	defer instantSwap.syncMu.RUnlock()
	return instantSwap.cancelSync != nil
}

func (instantSwap *InstantSwap) StopSync() {
	instantSwap.syncMu.Lock()
	if instantSwap.cancelSync != nil {
		instantSwap.cancelSync()
		instantSwap.cancelSync = nil
		log.Info("Exchange sync: stopped")
	}
	instantSwap.syncMu.Unlock()
}

// check all saved orders which status are not completed and update their status
func (instantSwap *InstantSwap) checkForUpdates(exchangeObject instantswap.IDExchange, exchangeServer ExchangeServer) error {
	offset := 0
	limit := 20
	for {
		// Check if instantswap has been shutdown and exit if true.
		if instantSwap.ctx.Err() != nil {
			return instantSwap.ctx.Err()
		}

		orders, err := instantSwap.GetOrdersRaw(int32(offset), int32(limit), true, "", "")
		if err != nil {
			return err
		}

		if len(orders) == 0 {
			break // exit the loop if there are no more orders to check
		}

		offset += len(orders)

		for _, order := range orders {
			// if the order was created before the ExchangeServer field was added
			// to the Order struct update it here. This prevents a crash when
			// attempting to open legacy orders
			nilExchangeServer := ExchangeServer{}
			if order.ExchangeServer == nilExchangeServer {
				privKey := privKeyMap[order.Server]
				order.ExchangeServer.Server = order.Server
				order.ExchangeServer.Config = ExchangeConfig{
					APIKey: privKey,
				}

				err = instantSwap.updateOrder(order)
				if err != nil {
					log.Errorf("Error updating legacy order: %v", err)
				}
			}

			if order.ExchangeServer == exchangeServer {
				// delay for 5 seconds to avoid hitting the rate limit
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
		if notificationListener.OnExchangeOrdersSynced != nil {
			notificationListener.OnExchangeOrdersSynced()
		}
	}
}

func (instantSwap *InstantSwap) publishOrderCreated(order *Order) {
	instantSwap.notificationListenersMu.Lock()
	defer instantSwap.notificationListenersMu.Unlock()

	for _, notificationListener := range instantSwap.notificationListeners {
		if notificationListener.OnOrderCreated != nil {
			notificationListener.OnOrderCreated(order)
		}
	}
}

func (instantSwap *InstantSwap) PublishOrderSchedulerStarted() {
	instantSwap.notificationListenersMu.Lock()
	defer instantSwap.notificationListenersMu.Unlock()

	for _, notificationListener := range instantSwap.notificationListeners {
		if notificationListener.OnOrderSchedulerStarted != nil {
			notificationListener.OnOrderSchedulerStarted()
		}
	}
}

func (instantSwap *InstantSwap) PublishOrderSchedulerEnded() {
	instantSwap.notificationListenersMu.Lock()
	defer instantSwap.notificationListenersMu.Unlock()

	for _, notificationListener := range instantSwap.notificationListeners {
		if notificationListener.OnOrderSchedulerEnded != nil {
			notificationListener.OnOrderSchedulerEnded()
		}
	}
}

func (instantSwap *InstantSwap) AddNotificationListener(notificationListener *OrderNotificationListener, uniqueIdentifier string) error {
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
