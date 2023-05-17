Checkpoint{
	Height: {{.Height}},
	BlockHeader: &wire.BlockHeader{
					Version: {{.BlockHeader.Version}},
					PrevBlock:  chainhash.Hash {{"{"}} {{range $index, $element := .BlockHeader.PrevBlock}}{{$element}},{{end}} {{"}"}},
					MerkleRoot: chainhash.Hash {{"{"}} {{range $index, $element := .BlockHeader.MerkleRoot}}{{$element}},{{end}} {{"}"}},
					Timestamp: time.Unix({{.BlockHeader.Timestamp.Unix}}, 0),
					Bits: {{.BlockHeader.Bits}},
					Nonce: {{.BlockHeader.Nonce}},
	},
	FilterHeader: &chainhash.Hash {{"{"}} {{range $index, $element := .FilterHeader}}{{$element}},{{end}} {{"}"}},
},