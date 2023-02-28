package instantswap

import (
	"context"
	"fmt"
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

	exchangeServers := instantSwap.ExchangeServers()

	for _, exchangeServer := range exchangeServers {
		exchangeObject, err := instantSwap.NewExchanageServer(exchangeServer)
		if err != nil {
			log.Errorf("Error creating exchange server: %v", err)
			continue
			// return err
		}

		err = instantSwap.syncServer(exchangeServer, exchangeObject)
		if err != nil {
			log.Errorf("Error syncing exchange server: %v", err)
			return err
		}
	}

	return nil
}

func (instantSwap *InstantSwap) syncServer(exchangeServer ExchangeServer, exchangeObject instantswap.IDExchange) error {
	// exchangeObject, err := instantSwap.NewExchanageServer(exchangeServer)
	// if err != nil {
	// 	log.Errorf("Error creating exchange server: %v", err)
	// 	return err
	// }

	log.Info("Exchange sync: started")

	// for {
	// Check if instantswap has been shutdown and exit if true.
	if instantSwap.ctx.Err() != nil {
		return instantSwap.ctx.Err()
	}

	log.Info("Exchange sync: checking for updates")

	for {
		err := instantSwap.checkForUpdates(exchangeObject, exchangeServer)
		if err != nil {
			log.Errorf("Error checking for exchange updates: %v", err)
			time.Sleep(retryInterval * time.Second)
			// return err
			continue
		}
		break
	}

	log.Info("Exchange sync: update complete")
	instantSwap.saveLastSyncedTimestamp(time.Now().Unix())
	instantSwap.publishSynced()
	return nil
	// }
	// return nil
}

// func (instantswap *InstantSwap) handeOrderUpdate(orders []Order) {

// }

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
	i := 0
	for {
		i++
		fmt.Println("checkForUpdates [][][][] i is", i)
		// Check if instantswap has been shutdown and exit if true.
		if instantSwap.ctx.Err() != nil {
			return instantSwap.ctx.Err()
		}

		orders, err := instantSwap.GetOrdersRaw(int32(offset), int32(limit), true)
		if err != nil {
			return err
		}

		fmt.Println("checkForUpdates lenghth of orders is", len(orders))
		if len(orders) == 0 {
			break
		}

		offset += len(orders)
		fmt.Println("checkForUpdates offset is", offset)

		for _, order := range orders {
			if order.ExchangeServer == exchangeServer {
				// if i%2 == 0 {
				time.Sleep(5 * time.Second)
				// }
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
