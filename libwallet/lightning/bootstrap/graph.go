package bootstrap

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"time"

	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/chainservice"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/channeldbservice"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/config"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/db"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/log"
	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/btcsuite/btcwallet/walletdb/bdb"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnwire"
	"go.etcd.io/bbolt"
)

const (
	threshold = 3 * 24 * time.Hour
	minEdges  = 25000
)

var (
	edgeBucket      = []byte("graph-edge")
	edgeIndexBucket = []byte("edge-index")
)

func getURL(workingDir string, db *channeldb.DB, tx walletdb.ReadTx) (string, error) {
	meta, err := db.FetchMeta(tx)
	if err != nil {
		logger.Errorf("chanDB.FetchMeta(): %v", err)
		return "", fmt.Errorf("chanDB.FetchMeta(): %w", err)
	}
	cfg, err := config.GetConfig(workingDir)
	if err != nil {
		logger.Errorf("config.GetConfig(%v): %v", workingDir, err)
		return "", fmt.Errorf("config.GetConfig(%v): %w", workingDir, err)
	}

	filename := "graph-" + fmt.Sprintf("%04x", meta.DbVersionNumber) + ".db"
	u, err := url.Parse(cfg.BootstrapURL)
	if err != nil {
		logger.Errorf("url.Parse(%v): %v", cfg.BootstrapURL, err)
		return "", fmt.Errorf("url.Parse(%v): %w", cfg.BootstrapURL, err)
	}
	u.Path = path.Join(u.Path, cfg.Network, "/graph/", filename)
	return u.String(), nil
}

func GraphURL(workingDir string, breezDB *db.DB) (string, error) {

	var err error
	if logger == nil {
		logger, err = log.GetLogger(workingDir, "BOOTSTRAP")
		if err != nil {
			return "", err
		}
		logger.SetLevel(btclog.LevelDebug)
	}

	// open the destination database.
	chanDB, chanDBCleanUp, err := channeldbservice.Get(workingDir)
	if err != nil {
		logger.Errorf("failed to open channeldb: %v", err)
		return "", fmt.Errorf("failed to open channeldb: %w", err)
	}
	defer chanDBCleanUp()

	var url string
	err = chanDB.View(func(tx walletdb.ReadTx) error {
		url, err = getURL(workingDir, chanDB, tx)
		return err
	}, func() {})
	if err != nil {
		logger.Errorf("getURL(): %v", err)
		return "", fmt.Errorf("getURL(): %w", err)
	}

	var done bool
	boltdb, err := bdb.UnderlineDB(chanDB.Backend)
	if err != nil {
		return "", err
	}
	err = boltdb.View(func(tx *bbolt.Tx) error {
		edges := tx.Bucket(edgeBucket)
		if edges == nil {
			done = true
			return nil
		}
		edgeIndex := edges.Bucket(edgeIndexBucket)
		if edgeIndex == nil {
			done = true
			return nil
		}
		if edgeIndex.Stats().KeyN < minEdges {
			done = true
			return nil
		}
		return nil
	})
	if done {
		logger.Infof("Downloading before checking chainservice: %v", url)
		return url, nil
	}

	chainService, chainServiceCleanUp, err := chainservice.Get(workingDir, breezDB)
	if err != nil {
		//chanDBCleanUp()
		logger.Errorf("failed to create chainservice: %v", err)
		return "", fmt.Errorf("failed to create chainservice: %w", err)
	}
	defer chainServiceCleanUp()

	chanID, err := chanDB.ChannelGraph().HighestChanID()
	if err != nil {
		logger.Errorf("chanDB.ChannelGraph().HighestChanID(): %v", err)
		return "", fmt.Errorf("chanDB.ChannelGraph().HighestChanID(): %w", err)
	}

	bs, err := chainService.BestBlock()
	if err != nil {
		logger.Errorf("chainService.BestBlock(): %v", err)
		return "", fmt.Errorf("chainService.BestBlock(): %w", err)
	}

	t := bs.Timestamp
	scid := lnwire.NewShortChanIDFromInt(chanID)
	logger.Infof("highest channel id = %v bs.Height=%v", scid.String(), bs.Height)
	_ = scid.BlockHeight
	if uint32(bs.Height) > scid.BlockHeight {
		hash, err := chainService.GetBlockHash(int64(scid.BlockHeight))
		if err != nil {
			logger.Errorf("chainService.GetBlockHash(%v): %v", scid.BlockHeight, err)
			return "", fmt.Errorf("chainService.GetBlockHash(%v): %w", scid.BlockHeight, err)
		}
		header, err := chainService.GetBlockHeader(hash)
		if err != nil {
			logger.Errorf("chainService.GetBlockHeader%v): %v", scid.BlockHeight, err)
			return "", fmt.Errorf("chainService.GetBlockHeader(%v): %w", scid.BlockHeight, err)
		}
		logger.Infof("header graph timestamp = %v\n", header.Timestamp)
		t = header.Timestamp
	} else {
		t = t.Add(time.Duration(int(scid.BlockHeight)-int(bs.Height)) * 10 * time.Minute)
		logger.Infof("estimated graph timestamp = %v\n", t)
	}

	if time.Now().Before(t.Add(threshold)) {
		logger.Infof("Not downloading graphdb because: %v < %v + %v", time.Now(), t, threshold)
		return "", nil
	}
	logger.Infof("Downloading graphdb because: %v >= %v + %v", time.Now(), t, threshold)
	return url, nil
}

func hasSourceNode(tx *bbolt.Tx) bool {
	nodes := tx.Bucket([]byte("graph-node"))
	if nodes == nil {
		return false
	}
	selfPub := nodes.Get([]byte("source"))
	return selfPub != nil
}

func ourNode(chanDB *channeldb.DB) (*channeldb.LightningNode, error) {
	graph := chanDB.ChannelGraph()
	node, err := graph.SourceNode()
	if err == channeldb.ErrSourceNodeNotSet || err == channeldb.ErrGraphNotFound {
		return nil, nil
	}
	return node, err
}

func ourData(tx walletdb.ReadWriteTx, ourNode *channeldb.LightningNode) (
	[]*channeldb.LightningNode, []*channeldb.ChannelEdgeInfo, []*channeldb.ChannelEdgePolicy, error) {

	nodeMap := make(map[string]*channeldb.LightningNode)
	var edges []*channeldb.ChannelEdgeInfo
	var policies []*channeldb.ChannelEdgePolicy

	err := ourNode.ForEachChannel(tx, func(tx walletdb.ReadTx,
		channelEdgeInfo *channeldb.ChannelEdgeInfo,
		toPolicy *channeldb.ChannelEdgePolicy,
		fromPolicy *channeldb.ChannelEdgePolicy) error {

		if toPolicy == nil || fromPolicy == nil {
			return ErrMissingPolicyError
		}
		nodeMap[hex.EncodeToString(toPolicy.Node.PubKeyBytes[:])] = toPolicy.Node
		edges = append(edges, channelEdgeInfo)
		if toPolicy != nil {
			policies = append(policies, toPolicy)
		}
		if fromPolicy != nil {
			policies = append(policies, fromPolicy)
		}
		return nil
	})

	if err != nil {
		return nil, nil, nil, err
	}
	var nodes []*channeldb.LightningNode
	for _, node := range nodeMap {
		nodes = append(nodes, node)
	}
	return nodes, edges, policies, nil
}

func putOurData(chanDB *channeldb.DB, node *channeldb.LightningNode, nodes []*channeldb.LightningNode,
	edges []*channeldb.ChannelEdgeInfo, policies []*channeldb.ChannelEdgePolicy) error {

	graph := chanDB.ChannelGraph()

	err := graph.SetSourceNode(node)
	if err != nil {
		return fmt.Errorf("graph.SetSourceNode(%x): %w", node.PubKeyBytes, err)
	}
	for _, n := range nodes {
		err = graph.AddLightningNode(n)
		if err != nil {
			return fmt.Errorf("graph.AddLightningNode(%x): %w", n.PubKeyBytes, err)
		}
	}
	for _, edge := range edges {
		err = graph.AddChannelEdge(edge)
		if err != nil && err != channeldb.ErrEdgeAlreadyExist {
			return fmt.Errorf("graph.AddChannelEdge(%x): %w", edge.ChannelID, err)
		}
	}
	for _, policy := range policies {
		err = graph.UpdateEdgePolicy(policy)
		if err != nil {
			return fmt.Errorf("graph.UpdateEdgePolicy(): %w", err)
		}
	}
	return nil
}
