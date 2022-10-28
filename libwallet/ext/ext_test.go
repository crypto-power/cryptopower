package ext

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	chainjson "github.com/decred/dcrd/rpc/jsonrpc/types/v3"
	apiTypes "github.com/decred/dcrdata/v7/api/types"
	"github.com/decred/dcrdata/v7/db/dbtypes"
)

var (
	agendaStatus = &chainjson.GetVoteInfoResult{
		CurrentHeight: 681658,
		StartHeight:   681472,
		EndHeight:     689535,
		Hash:          "00000000000000008a66428f2b98ab0ed1a220cfe23013acc393801d5e480b40",
		VoteVersion:   9,
		Quorum:        4032,
		TotalVotes:    931,
		Agendas: []chainjson.Agenda{
			{
				ID:             "reverttreasurypolicy",
				Description:    "Change maximum treasury expenditure policy as defined in DCP0007",
				Mask:           6,
				StartTime:      1631750400,
				ExpireTime:     1694822400,
				Status:         "active",
				QuorumProgress: 0,
				Choices: []chainjson.Choice{
					{
						ID:          "abstain",
						Description: "abstain voting for change",
						Bits:        0,
						IsAbstain:   true,
						IsNo:        false,
						Count:       0,
						Progress:    0,
					},
					{
						ID:          "no",
						Description: "keep the existing consensus rules",
						Bits:        2,
						IsAbstain:   false,
						IsNo:        true,
						Count:       0,
						Progress:    0,
					},
					{
						ID:          "yes",
						Description: "change to the new consensus rules",
						Bits:        4,
						IsAbstain:   false,
						IsNo:        false,
						Count:       0,
						Progress:    0,
					},
				},
			},
			{
				ID:             "explicitverupgrades",
				Description:    "Enable explicit version upgrades as defined in DCP0008",
				Mask:           24,
				StartTime:      1631750400,
				ExpireTime:     1694822400,
				Status:         "active",
				QuorumProgress: 0,
				Choices: []chainjson.Choice{
					{
						ID:          "abstain",
						Description: "abstain from voting",
						Bits:        0,
						IsAbstain:   true,
						IsNo:        false,
						Count:       0,
						Progress:    0,
					},
					{
						ID:          "no",
						Description: "keep the existing consensus rules",
						Bits:        8,
						IsAbstain:   false,
						IsNo:        true,
						Count:       0,
						Progress:    0,
					}, {

						ID:          "yes",
						Description: "change to the new consensus rules",
						Bits:        16,
						IsAbstain:   false,
						IsNo:        false,
						Count:       0,
						Progress:    0,
					},
				},
			},
		},
	}

	agendas = &[]apiTypes.AgendasInfo{
		{
			Name:        "reverttreasurypolicy",
			Description: "Change maximum treasury expenditure policy as defined in DCP0007",
			MileStone: &dbtypes.MileStone{
				ID:            0,
				Status:        dbtypes.ActivatedAgendaStatus,
				VotingStarted: 0,
				VotingDone:    649215,
				Activated:     657280,
				HardForked:    0,
				StartTime:     stringToTime("2021-09-16 00:00:00 +0000 UTC"),
				ExpireTime:    stringToTime("2023-09-16 00:00:00 +0000 UTC"),
			},
			VoteVersion: 9,
			Mask:        6,
		},
		{
			Name:        "explicitverupgrades",
			Description: "Enable explicit version upgrades as defined in DCP0008",
			MileStone: &dbtypes.MileStone{
				ID:            0,
				Status:        dbtypes.ActivatedAgendaStatus,
				VotingStarted: 0,
				VotingDone:    649215,
				Activated:     657280,
				HardForked:    0,
				StartTime:     stringToTime("2021-09-16 00:00:00 +0000 UTC"),
				ExpireTime:    stringToTime("2023-09-16 00:00:00 +0000 UTC"),
			},
			VoteVersion: 9, Mask: 24,
		},
		{
			Name:        "changesubsidysplit",
			Description: "Change block reward subsidy split to 10/80/10 as defined in DCP0010",
			MileStone: &dbtypes.MileStone{
				ID:            0,
				Status:        dbtypes.ActivatedAgendaStatus,
				VotingStarted: 0,
				VotingDone:    649215,
				Activated:     657280,
				HardForked:    0,
				StartTime:     stringToTime("2021-09-16 00:00:00 +0000 UTC"),
				ExpireTime:    stringToTime("2023-09-16 00:00:00 +0000 UTC"),
			},
			VoteVersion: 9,
			Mask:        384,
		},
	}

	exrate = &ExchangeRates{
		BtcIndex: "USD",
		DcrPrice: 26.182965232480655,
		BtcPrice: 22728.06395,
		Exchanges: map[string]BaseState{
			"binance": BaseState{
				Price:      0.001152,
				BaseVolume: 5.63390041,
				Volume:     4856.908,
				Change:     8e-06,
				Stamp:      1659444742,
			},
			"bittrex": BaseState{
				Price:      0.00114285,
				BaseVolume: 0.74640502,
				Volume:     651.46221106,
				Change:     -3.898946417820641e-06,
				Stamp:      0,
			},
			"dcrdex": BaseState{
				Price:      0.001157,
				BaseVolume: 0,
				Volume:     1160,
				Change:     0.006988868290729958,
				Stamp:      1659445032,
			},
			"huobi": BaseState{
				Price:      0.001165,
				BaseVolume: 0.0206273289,
				Volume:     17.705861716738195,
				Change:     2.099999999999997e-05,
				Stamp:      1659445185,
			},
		},
	}

	feeRateSummary = &apiTypes.MempoolTicketFeeInfo{
		Height: 681734,
		Time:   1659448720,
		FeeInfoMempool: chainjson.FeeInfoMempool{
			Number: 0,
			Min:    0,
			Max:    0,
			Mean:   0,
			Median: 0,
			StdDev: 0,
		},
		LowestMineable: 0,
	}

	feeRate = &apiTypes.MempoolTicketFees{
		Height:   681741,
		Time:     1659449708,
		Length:   0,
		Total:    0,
		FeeRates: []float64{},
	}

	adrrState = &AddressState{
		Address:            "DsTxPUVFxXeNgu5fzozr4mTR4tqqMaKcvpY",
		Balance:            0,
		TotalReceived:      95645588,
		TotalSent:          95645588,
		UnconfirmedBalance: 0,
		UnconfirmedTxs:     0,
		Txs:                17,
		TxIds: []string{
			"bd1bf8897a5c1a53f3e90c26fc908d03624f8bd5d21da49ba8fa80cb99bae84d",
			"335fc62ec6ebd8d29cef8dc98478807327ad2f2bc58a4ca6fb8a73411a38788f",
			"88b4e3b7162667d0d5aec2e78663342721413a6fea280062444ab8d9f13065ac",
			"af31265384acd2973a6856cf7efaf9a663b114af4af890f3cc1664cb34f7ea30",
			"5ebc20513c6ff5647f1471931957c739af69d09b0345a4032d39113d6759b938",
			"bc35ecef383393f0fe33f93075756c381197f3e2e5db4f1ea81b970d143ed10d",
			"22009e6feb0ecbd29004f8a5273163da53c112d93ec5f9dbff46bc41a30f90f0",
			"66e3d1622d4cb3337babf5e5a466f94b6a732e132c802cc219c6e26543d736e0",
			"025abf340da79c4fc4de29667c08fc22cdafbded24c7c2a994a27f6fb4d5fa17",
			"68dcb93e36879f33e741f5706463af10387aadeba193cb204453ec23ad1b1979",
			"70a5caffaac9105b47ff9b70213f3a9f8bbd28f09335dce763a99f1b39e74ccf",
			"2c922ec02ec0bc86803290191198232183e903ea2e79f7f6ddb6c7c7c193b9db",
			"a81ab3f0d0744c42fcc8e5c3b0b77382323d0cf6ac73a55f982dcadc4fe604c7",
			"ce314ec559ceae4d2a98ed4708848708c9cfde14b484b879838a2dc75c793910",
			"6928795181bd951849bcdebab2f012b1f6a71f8f1993e573ee1b531207c61255",
			"5015d14dcfd78998cfa13e0325798a74d95bbe75f167a49467303f70dde9bffd",
			"ac4b4b9af3e0b8cdb7918bb6bfdf9878ace398b92e6e5ef7844936c676041a26",
		},
	}

	xpubState = &XpubBalAndTxs{
		Xpub:               "dpubZFf1tYMxcku9nvDRxnYdE4yrEESrkuQFRq5RwA4KoYQKpDSRszN2emePTwLgfQpd4mZHGrHbQkKPZdjH1BcopomXRnr5Gt43rjpNEfeuJLN",
		Balance:            0,
		TotalReceived:      397117611,
		TotalSent:          397117611,
		UnconfirmedBalance: 0,
		UnconfirmedTxs:     0,
		Txs:                35,
		TxIds: []string{
			"bd1bf8897a5c1a53f3e90c26fc908d03624f8bd5d21da49ba8fa80cb99bae84d",
			"335fc62ec6ebd8d29cef8dc98478807327ad2f2bc58a4ca6fb8a73411a38788f",
			"88b4e3b7162667d0d5aec2e78663342721413a6fea280062444ab8d9f13065ac",
			"af31265384acd2973a6856cf7efaf9a663b114af4af890f3cc1664cb34f7ea30",
			"5ebc20513c6ff5647f1471931957c739af69d09b0345a4032d39113d6759b938",
			"bc35ecef383393f0fe33f93075756c381197f3e2e5db4f1ea81b970d143ed10d",
		},
		UsedTokens:  27,
		XpubAddress: []XpubAddress{},
	}
)

func stringToTime(t string) time.Time {
	time, _ := time.Parse("2006-01-02 00:00:00 +0000 UTC", t)
	return time
}

func mainnetService() *Service {
	chainParams, err := utils.DCRChainParams("mainnet")
	if err != nil {
		fmt.Println("Error creating chain params.")
		return nil
	}

	return NewService(chainParams)
}

var service = mainnetService()

func TestGetBestBlock(t *testing.T) {

	tests := []struct {
		name             string
		server           *httptest.Server
		expectedResponse int32
		expectedErr      error
	}{
		{
			name: "best block",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`681536`))
			})),
			expectedResponse: 681536,
			expectedErr:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			backendUrl["mainnet"][DcrData] = tc.server.URL + "/"
			backendUrl["testnet3"][DcrData] = tc.server.URL + "/"
			resp := service.GetBestBlock()
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedResponse, resp)
			}

		})
	}
}

func TestGetBestBlockTimeStamp(t *testing.T) {
	tests := []struct {
		name             string
		server           *httptest.Server
		expectedResponse int64
		expectedErr      error
	}{
		{
			name: "bestblock timeStamp",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"height":681649,"size":22216,"hash":"0000000000000000109256ba6dab7e921c4d7e98d00357f51bff3b2d28ef345e",
					"diff":3046219499.387013,"sdiff":227.59014758,"time":1659420872,"txlength":0,"ticket_pool":{"height":0,"size":41152,
					"value":9257360.83036327,"valavg":416.6979127819261,"winners":["ee6e3114edf309d473eb04642c9248abf9843232a428d3694eda8c41c0fdbd20",
					"25fe6b7904a157b06e2819b627682224a6cc0d41b1a7daa36133ac2354ef04fb","f1800bd09fdbb7e02721f9673d427b0eb481447d5a5a9357b75536d6b73dc06f",
					"37f941f0977b9dd7a05b37ecb9dcfab655e308168b1f474445f88cabbb40324f","12e62f0085ffaf7797b9d104d52f1c2cb1147b054180159977b37b5f4ee1833b"]}}`))
			})),
			expectedResponse: 1659420872,
			expectedErr:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			backendUrl["mainnet"][DcrData] = tc.server.URL + "/"
			backendUrl["testnet3"][DcrData] = tc.server.URL + "/"
			resp := service.GetBestBlockTimeStamp()
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedResponse, resp)
			}

		})
	}
}

func TestGetCurrentAgendaStatus(t *testing.T) {
	tests := []struct {
		name             string
		server           *httptest.Server
		expectedResponse *chainjson.GetVoteInfoResult
		expectedErr      error
	}{
		{
			name: "current agenda status",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"currentheight":681658,"startheight":681472,"endheight":689535,"hash":"00000000000000008a66428f2b98ab0ed1a220cfe23013acc393801d5e480b40",
					"voteversion":9,"quorum":4032,"totalvotes":931,"agendas":[{"id":"reverttreasurypolicy","description":"Change maximum treasury expenditure policy as defined in DCP0007",
					"mask":6,"starttime":1631750400,"expiretime":1694822400,"status":"active","quorumprogress":0,"choices":[{"id":"abstain","description":"abstain voting for change",
					"bits":0,"isabstain":true,"isno":false,"count":0,"progress":0},{"id":"no","description":"keep the existing consensus rules","bits":2,"isabstain":false,"isno":true,"count":0,"progress":0},
					{"id":"yes","description":"change to the new consensus rules","bits":4,"isabstain":false,"isno":false,"count":0,"progress":0}]},{"id":"explicitverupgrades",
					"description":"Enable explicit version upgrades as defined in DCP0008","mask":24,"starttime":1631750400,"expiretime":1694822400,"status":"active","quorumprogress":0,
					"choices":[{"id":"abstain","description":"abstain from voting","bits":0,"isabstain":true,"isno":false,"count":0,"progress":0},{"id":"no","description":"keep the existing consensus rules",
					"bits":8,"isabstain":false,"isno":true,"count":0,"progress":0},{"id":"yes","description":"change to the new consensus rules","bits":16,"isabstain":false,"isno":false,"count":0,"progress":0}]}]}`))
			})),
			expectedResponse: agendaStatus,
			expectedErr:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			backendUrl["mainnet"][DcrData] = tc.server.URL + "/"
			backendUrl["testnet3"][DcrData] = tc.server.URL + "/"
			resp, err := service.GetCurrentAgendaStatus()
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedResponse, resp)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedErr, err)
			}

		})
	}
}

func TestGetAgendas(t *testing.T) {
	tests := []struct {
		name             string
		server           *httptest.Server
		expectedResponse *[]apiTypes.AgendasInfo
		expectedErr      error
	}{
		{
			name: "agendas list",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[{"name":"reverttreasurypolicy","description":"Change maximum treasury expenditure policy as defined in DCP0007","status":"finished","votingStarted":0,"votingdone":649215,"activated":657280,"hardforked":0,"starttime":"2021-09-16T00:00:00Z","expiretime":"2023-09-16T00:00:00Z","voteversion":9,"mask":6},
						{"name":"explicitverupgrades","description":"Enable explicit version upgrades as defined in DCP0008","status":"finished","votingStarted":0,"votingdone":649215,"activated":657280,"hardforked":0,"starttime":"2021-09-16T00:00:00Z","expiretime":"2023-09-16T00:00:00Z","voteversion":9,"mask":24},
						{"name":"changesubsidysplit","description":"Change block reward subsidy split to 10/80/10 as defined in DCP0010","status":"finished","votingStarted":0,"votingdone":649215,"activated":657280,"hardforked":0,"starttime":"2021-09-16T00:00:00Z","expiretime":"2023-09-16T00:00:00Z","voteversion":9,"mask":384}]`))
			})),
			expectedResponse: agendas,
			expectedErr:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			backendUrl["mainnet"][DcrData] = tc.server.URL + "/"
			backendUrl["testnet3"][DcrData] = tc.server.URL + "/"
			resp, err := service.GetAgendas()
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedResponse, resp)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedErr, err)
			}

		})
	}
}

func TestGetTreasuryBalance(t *testing.T) {
	tests := []struct {
		name             string
		server           *httptest.Server
		expectedResponse int64
		expectedErr      error
	}{
		{
			name: "treasury balance",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"height":681712,"maturity_height":681456,"balance":75919531830200,"output_count":129283,
						"add_count":13,"added":61780107690000,"spend_count":5,"spent":779373012698,"tbase_count":129265,
						"tbase":14918797152898,"immature_count":256,"immature":26729161216}`))
			})),
			expectedResponse: 75919531830200,
			expectedErr:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			backendUrl["mainnet"][DcrData] = tc.server.URL + "/"
			backendUrl["testnet3"][DcrData] = tc.server.URL + "/"
			resp, err := service.GetTreasuryBalance()
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedResponse, resp)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedErr, err)
			}

		})
	}
}

func TestGetExchangeRate(t *testing.T) {
	tests := []struct {
		name             string
		server           *httptest.Server
		expectedResponse *ExchangeRates
		expectedErr      error
	}{
		{
			name: "exchange rate",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"btcIndex":"USD","dcrPrice":26.182965232480655,"btcPrice":22728.06395,
					"exchanges":{
						"binance":{"price":0.001152,"base_volume":5.63390041,"volume":4856.908,"change":0.000008,"timestamp":1659444742},
						"bittrex":{"price":0.00114285,"base_volume":0.74640502,"volume":651.46221106,"change":-0.000003898946417820641},
						"dcrdex":{"price":0.001157,"volume":1160,"change":0.006988868290729958,"timestamp":1659445032},
						"huobi":{"price":0.001165,"base_volume":0.0206273289,"volume":17.705861716738195,"change":0.00002099999999999997,"timestamp":1659445185}}}`))
			})),
			expectedResponse: exrate,
			expectedErr:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			backendUrl["mainnet"][DcrData] = tc.server.URL + "/"
			backendUrl["testnet3"][DcrData] = tc.server.URL + "/"
			resp, err := service.GetExchangeRate()
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedResponse, resp)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedErr, err)
			}

		})
	}
}

func TestGetTicketFeeRateSummary(t *testing.T) {
	tests := []struct {
		name             string
		server           *httptest.Server
		expectedResponse *apiTypes.MempoolTicketFeeInfo
		expectedErr      error
	}{
		{
			name: "Ticket Fee rate summary",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"height":681734,"time":1659448720,"number":0,"min":0,"max":0,"mean":0,"median":0,"stddev":0,"lowest_mineable":0}`))
			})),
			expectedResponse: feeRateSummary,
			expectedErr:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			backendUrl["mainnet"][DcrData] = tc.server.URL + "/"
			backendUrl["testnet3"][DcrData] = tc.server.URL + "/"
			resp, err := service.GetTicketFeeRateSummary()
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedResponse, resp)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedErr, err)
			}

		})
	}
}

func TestGetTicketFeeRate(t *testing.T) {
	tests := []struct {
		name             string
		server           *httptest.Server
		expectedResponse *apiTypes.MempoolTicketFees
		expectedErr      error
	}{
		{
			name: "Ticket Fee rate",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"height":681741,"time":1659449708,"length":0,"total":0,"top_fees":[]}`))
			})),
			expectedResponse: feeRate,
			expectedErr:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			backendUrl["mainnet"][DcrData] = tc.server.URL + "/"
			backendUrl["testnet3"][DcrData] = tc.server.URL + "/"
			resp, err := service.GetTicketFeeRate()
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedResponse, resp)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedErr, err)
			}

		})
	}
}

func TestGetAddress(t *testing.T) {
	tests := []struct {
		name             string
		server           *httptest.Server
		expectedResponse *AddressState
		expectedErr      error
	}{
		{
			name: "address state",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"page":1,"totalPages":1,"itemsOnPage":1000,"address":"DsTxPUVFxXeNgu5fzozr4mTR4tqqMaKcvpY","balance":"0",
					"totalReceived":"95645588","totalSent":"95645588","unconfirmedBalance":"0","unconfirmedTxs":0,"txs":17,
					"txids":["bd1bf8897a5c1a53f3e90c26fc908d03624f8bd5d21da49ba8fa80cb99bae84d","335fc62ec6ebd8d29cef8dc98478807327ad2f2bc58a4ca6fb8a73411a38788f",
					"88b4e3b7162667d0d5aec2e78663342721413a6fea280062444ab8d9f13065ac","af31265384acd2973a6856cf7efaf9a663b114af4af890f3cc1664cb34f7ea30",
					"5ebc20513c6ff5647f1471931957c739af69d09b0345a4032d39113d6759b938","bc35ecef383393f0fe33f93075756c381197f3e2e5db4f1ea81b970d143ed10d",
					"22009e6feb0ecbd29004f8a5273163da53c112d93ec5f9dbff46bc41a30f90f0","66e3d1622d4cb3337babf5e5a466f94b6a732e132c802cc219c6e26543d736e0",
					"025abf340da79c4fc4de29667c08fc22cdafbded24c7c2a994a27f6fb4d5fa17","68dcb93e36879f33e741f5706463af10387aadeba193cb204453ec23ad1b1979",
					"70a5caffaac9105b47ff9b70213f3a9f8bbd28f09335dce763a99f1b39e74ccf","2c922ec02ec0bc86803290191198232183e903ea2e79f7f6ddb6c7c7c193b9db",
					"a81ab3f0d0744c42fcc8e5c3b0b77382323d0cf6ac73a55f982dcadc4fe604c7","ce314ec559ceae4d2a98ed4708848708c9cfde14b484b879838a2dc75c793910",
					"6928795181bd951849bcdebab2f012b1f6a71f8f1993e573ee1b531207c61255","5015d14dcfd78998cfa13e0325798a74d95bbe75f167a49467303f70dde9bffd",
					"ac4b4b9af3e0b8cdb7918bb6bfdf9878ace398b92e6e5ef7844936c676041a26"]}`))
			})),
			expectedResponse: adrrState,
			expectedErr:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.server.Close()
			backendUrl["mainnet"][DcrData] = tc.server.URL + "/"
			backendUrl["testnet3"][DcrData] = tc.server.URL + "/"
			resp, err := service.GetAddress("DsTxPUVFxXeNgu5fzozr4mTR4tqqMaKcvpY")
			if !reflect.DeepEqual(resp, tc.expectedResponse) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedResponse, resp)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("(%v), expected (%v), got (%v)", tc.name, tc.expectedErr, err)
			}

		})
	}
}
