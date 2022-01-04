package genesis

// originally from, comments preserved (whatever they mean):
// https://github.com/trick77/bc-src/blob/494467594ff6456012029d3be11a80de24602b84/lib/bc/genesis.raw.js
const (
	HASH           = "a8212d5a65f579c2018b19172be34e4422a93c8437f8e7c19ddc8cad15353862"
	PREVIOUS_HASH  = "b6615809ca3ff24562ce2724ef010e369a976cb9068570074f88919eaddcd08f"
	VERSION        = 1
	SCHEMA_VERSION = 1
	HEIGHT         = 1
	MINER          = "0x028d3af888e08aa8380e5866b6ed068bd60e7b19"
	DIFFICULTY     = "296401962029366"
	TIMESTAMP      = 0
	MERKLE_ROOT    = "b277537249649f9e7e56d549805ccfdec56fddd6153c6bf4ded85c3e072ccbdf"
	CHAIN_ROOT     = "b4816d65eabac8f1a143805ffc6f4ca148c4548e020de3db21207a4849ea9abe"
	DISTANCE       = 15698005660040
	TOTAL_DISTANCE = "15698005660040"
	NONCE          = "bf03cdfbc60f2d075a3dadf27e5a372b64cbaea4c20923048dfd8c431408c332"
	NRG_GRANT      = 200000000
	TWN            = 0 // Overline
	//TWS_LIST = make([]interface{}, 0, 0) // Overline // no const arrays in golang

	EMBLEM_WEIGHT                 = 6757
	EMBLEM_CHAIN_FINGERPRINT_ROOT = "87b2bc3f12e3ded808c6d4b9b528381fa2a7e95ff2368ba93191a9495daa7f50"
	EMBLEM_CHAIN_ADDRESS          = "0x28b94f58b11ac945341329dbf2e5ef7f8bd44225"

	TX_COUNT              = 0
	TX_FEE_BASE           = 0
	TX_DISTANCE_SUM_LIMIT = 0

	CHILD_BLOCKCHAIN_COUNT = 5 // not used in genesis module now
	//BLOCKCHAIN_HEADERS_MAP = make(map[string]*p2p_pb.BlockchainHeader) // no const maps in golang

	BLOCKCHAIN_FINGERPRINTS_ROOT = "d65ffda8a561b53c09377ef7d3ee9ebbf18a618c603faf2631c1bbb7d66a03ac"
	AT_BALANCE_DIGEST            = "91199a150de07009d537b4b8191c67d81c657e0df96062e46ecd05ab5d4480ae489e57f397739a6a897889bc68a96c78096f4044cb63e8c274cbb43b853faab7"
)
